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
	s.loop(ctx, "maintenance", s.cfg.MaintenanceTick, s.Maintenance)
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
	return joinErrs(errs)
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
	return joinErrs(errs)
}

func joinErrs(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%v", errs)
}
