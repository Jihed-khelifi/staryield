// Package worker runs the server-authoritative background loops: per-minute
// billing, request expiry, reconnection grace expiry, settlement retries,
// presence reconciliation and housekeeping. Every job claims rows with
// SELECT … FOR UPDATE SKIP LOCKED, so any number of API instances can run
// the scheduler concurrently without double-processing.
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Scheduler owns the background loops.
type Scheduler struct {
	chat     *service.ChatService
	requests *service.CustomRequestService
	tokens   repository.TokenRepository
	clock    clock.Clock
	cfg      config.WorkerConfig
	log      *zap.Logger
	wg       sync.WaitGroup
	cleanups []Cleanup
	jobs     []Job
	tickJobs []TickJob // billing renewals, …
}

// Cleanup is an extra housekeeping job run with Maintenance.
type Cleanup struct {
	Name string
	Run  func(ctx context.Context, now time.Time) (int64, error)
}

// TickJob is an extra time-critical job run with Tick (e.g. membership
// renewals); it must be safe to run on every instance concurrently.
type TickJob struct {
	Name string
	Run  func(ctx context.Context) error
}

// WithTickJobs registers extra jobs run on every Tick.
func (s *Scheduler) WithTickJobs(j ...TickJob) *Scheduler {
	s.tickJobs = append(s.tickJobs, j...)
	return s
}

// WithCleanups registers extra housekeeping jobs (expired admin sessions,
// old notifications).
func (s *Scheduler) WithCleanups(c ...Cleanup) *Scheduler {
	s.cleanups = append(s.cleanups, c...)
	return s
}

// Job is an extra background loop run every Every and, when Wake is set,
// immediately whenever Wake fires (e.g. the e-mail outbox after an
// enqueue). Jobs must be safe to run on several instances at once.
type Job struct {
	Name  string
	Every time.Duration
	Wake  <-chan struct{}
	Run   func(ctx context.Context) error
}

// WithJobs registers extra background loops.
func (s *Scheduler) WithJobs(j ...Job) *Scheduler {
	s.jobs = append(s.jobs, j...)
	return s
}

// RunJob runs a registered job once by name (tests drive the scheduler
// explicitly).
func (s *Scheduler) RunJob(ctx context.Context, name string) error {
	for _, j := range s.jobs {
		if j.Name == name {
			return j.Run(ctx)
		}
	}
	return fmt.Errorf("worker: no job named %q", name)
}

// New returns a scheduler.
func New(chat *service.ChatService, requests *service.CustomRequestService, tokens repository.TokenRepository,
	clk clock.Clock, cfg config.WorkerConfig, log *zap.Logger) *Scheduler {
	return &Scheduler{chat: chat, requests: requests, tokens: tokens, clock: clk, cfg: cfg, log: log.With(zap.String("component", "worker"))}
}

// Start launches the loops; they stop when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.loop(ctx, "sessions", s.cfg.TickInterval, s.Tick)
	s.loop(ctx, "presence", s.cfg.PresenceSweep, func(ctx context.Context) error {
		return s.chat.SweepPresence(ctx, 2000)
	})
	// Chat ordering: publish message events held back behind a gap that
	// never closed (realtime.GapFlushAge).
	s.loop(ctx, "chat-gaps", time.Second, s.FlushMessageGaps)
	s.loop(ctx, "maintenance", s.cfg.MaintenanceTick, s.Maintenance)
	for _, j := range s.jobs {
		s.loopWake(ctx, j)
	}
}

// loopWake is loop with an extra wake-up channel.
func (s *Scheduler) loopWake(ctx context.Context, j Job) {
	if j.Every <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log := s.log.With(zap.String("job", j.Name))
		t := time.NewTicker(j.Every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-j.Wake:
			}
			s.runOnce(ctx, log, j.Every, j.Run)
		}
	}()
}

// Wait blocks until all loops have exited.
func (s *Scheduler) Wait() { s.wg.Wait() }

func (s *Scheduler) loop(ctx context.Context, name string, every time.Duration, job func(context.Context) error) {
	if every <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log := s.log.With(zap.String("job", name))
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.runOnce(ctx, log, every, job)
			}
		}
	}()
}

// runOnce executes one job iteration with a deadline and panic guard so a
// faulty iteration never stops the loop.
func (s *Scheduler) runOnce(parent context.Context, log *zap.Logger, every time.Duration, job func(context.Context) error) {
	timeout := max(every*10, 30*time.Second)
	ctx, cancel := context.WithTimeout(logger.WithContext(parent, log), timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			log.Error("worker job panicked", zap.Any("panic", r), zap.Stack("stack"))
		}
	}()
	start := time.Now()
	if err := job(ctx); err != nil && parent.Err() == nil {
		log.Error("worker job failed", zap.Error(err), zap.Duration("elapsed", time.Since(start)))
	}
}

// Tick runs the time-critical session jobs (also called directly by tests).
func (s *Scheduler) Tick(ctx context.Context) error {
	batch := s.cfg.BatchSize
	var errs []error
	if n, err := s.chat.ExpireDue(ctx, batch); err != nil {
		errs = append(errs, fmt.Errorf("expire requests: %w", err))
	} else if n > 0 {
		logger.FromContext(ctx).Info("chat requests expired", zap.Int("count", n))
	}
	if _, err := s.chat.BillDue(ctx, batch); err != nil {
		errs = append(errs, fmt.Errorf("bill sessions: %w", err))
	}
	if n, err := s.chat.EndExpiredGrace(ctx, batch); err != nil {
		errs = append(errs, fmt.Errorf("end grace: %w", err))
	} else if n > 0 {
		logger.FromContext(ctx).Info("chat sessions ended after grace period", zap.Int("count", n))
	}
	if err := s.FlushMessageGaps(ctx); err != nil {
		errs = append(errs, err)
	}
	for _, j := range s.tickJobs {
		if err := j.Run(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", j.Name, err))
		}
	}
	return joinErrs(errs)
}

// FlushMessageGaps publishes chat message events whose predecessor never
// arrived within realtime.GapFlushAge (clients then fill the hole through
// the messages "since" endpoint).
func (s *Scheduler) FlushMessageGaps(ctx context.Context) error {
	n, err := s.chat.FlushMessageGaps(ctx)
	if err != nil {
		return fmt.Errorf("flush chat gaps: %w", err)
	}
	if n > 0 {
		logger.FromContext(ctx).Warn("chat events published past a sequence gap", zap.Int("count", n))
	}
	return nil
}

// Maintenance runs slower housekeeping jobs.
func (s *Scheduler) Maintenance(ctx context.Context) error {
	var errs []error
	if n, err := s.chat.SettlePending(ctx, s.cfg.BatchSize); err != nil {
		errs = append(errs, fmt.Errorf("settle sessions: %w", err))
	} else if n > 0 {
		logger.FromContext(ctx).Warn("settled sessions via retry", zap.Int("count", n))
	}
	if _, err := s.requests.ExpireDue(ctx, s.cfg.BatchSize); err != nil {
		errs = append(errs, fmt.Errorf("expire custom requests: %w", err))
	}
	if _, err := s.tokens.DeleteExpired(ctx, s.clock.Now().Add(-24*time.Hour)); err != nil {
		errs = append(errs, fmt.Errorf("purge tokens: %w", err))
	}
	for _, c := range s.cleanups {
		if n, err := c.Run(ctx, s.clock.Now()); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", c.Name, err))
		} else if n > 0 {
			logger.FromContext(ctx).Info("housekeeping", zap.String("job", c.Name), zap.Int64("rows", n))
		}
	}
	return joinErrs(errs)
}

func joinErrs(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%v", errs)
}
