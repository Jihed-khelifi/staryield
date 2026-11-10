package service

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

const (
	// BroadcastRecoveryJob is the scheduler job resuming interrupted broadcasts.
	BroadcastRecoveryJob = "broadcast-recovery"
	// broadcastChunk is how many recipients one fan-out step handles. Each
	// step is one transaction (insert + cursor + heartbeat); the realtime
	// publishes of a step are pipelined.
	broadcastChunk = 1000
	// broadcastStale is how long a broadcast may go without a heartbeat
	// before the recovery job resumes it.
	broadcastStale = 2 * time.Minute
	// broadcastGiveUp fails a broadcast that is still unfinished this long
	// after it was created.
	broadcastGiveUp = 24 * time.Hour
	// broadcastRunLimit bounds one run (a resumed run continues the rest).
	broadcastRunLimit = 30 * time.Minute
)

type broadcastCounters struct {
	chunks, resumed, chunkRetries, completed atomic.Int64
}

// BroadcastStats are the fan-out counters (since start).
type BroadcastStats struct {
	Chunks       int64 `json:"chunks"`
	Resumed      int64 `json:"resumed"`
	ChunkRetries int64 `json:"chunk_retries"`
	Completed    int64 `json:"completed"`
}

// BroadcastSnapshot returns the fan-out counters.
func (s *NotificationService) BroadcastSnapshot() BroadcastStats {
	return BroadcastStats{Chunks: s.bstats.chunks.Load(), Resumed: s.bstats.resumed.Load(),
		ChunkRetries: s.bstats.chunkRetries.Load(), Completed: s.bstats.completed.Load()}
}

// Wait blocks until in-flight broadcasts are delivered (tests).
func (s *NotificationService) Wait() { s.wg.Wait() }

// Shutdown waits for in-flight broadcasts until ctx (the shutdown deadline)
// ends, then asks them to stop; an unfinished broadcast resumes after
// restart through broadcast-recovery.
func (s *NotificationService) Shutdown(ctx context.Context) {
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return
	case <-ctx.Done():
	}
	s.cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.log.Warn("broadcasts: fan-out still running at shutdown; it resumes after restart")
	}
}

// runBroadcast fans a broadcast out under its advisory lock: only one runner
// (this process or another) works on a broadcast at a time. A run that stops
// early leaves the broadcast `sending`; the recovery job continues it.
func (s *NotificationService) runBroadcast(parent context.Context, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(parent, broadcastRunLimit)
	defer cancel()
	stop := context.AfterFunc(s.runCtx, cancel)
	defer stop()
	log := s.log.With(zap.String("broadcast_id", id.String()))
	acquired, err := s.repo.WithBroadcastLock(ctx, id, func(ctx context.Context) error { return s.fanOut(ctx, id) })
	switch {
	case err != nil && ctx.Err() == nil:
		log.Error("broadcast: fan-out interrupted (will resume)", zap.Error(err))
	case err != nil:
		log.Warn("broadcast: fan-out stopped (will resume)", zap.Error(err))
	case !acquired:
		log.Debug("broadcast: another runner holds the broadcast")
	}
}

// RecoverBroadcasts resumes queued or sending broadcasts whose heartbeat is
// older than two minutes (a crashed or restarted instance). Safe to run on
// every instance: the per-broadcast advisory lock admits one runner.
func (s *NotificationService) RecoverBroadcasts(ctx context.Context) error {
	now := s.clock.Now()
	stale, err := s.repo.StaleBroadcasts(ctx, now.Add(-broadcastStale), 10)
	if err != nil {
		return err
	}
	for i := range stale {
		b := &stale[i]
		if ctx.Err() != nil || s.runCtx.Err() != nil {
			return nil
		}
		if now.Sub(b.CreatedAt) > broadcastGiveUp {
			s.log.Error("broadcast: giving up on an unfinished broadcast", zap.String("broadcast_id", b.ID.String()))
			_ = s.repo.UpdateBroadcast(ctx, b.ID, map[string]any{"status": "failed"})
			continue
		}
		s.log.Info("broadcast: resuming", zap.String("broadcast_id", b.ID.String()), zap.String("status", b.Status))
		s.bstats.resumed.Add(1)
		s.wg.Add(1)
		func() {
			defer s.wg.Done()
			s.runBroadcast(ctx, b.ID)
		}()
	}
	return nil
}

func audienceRoles(audience string) []domain.Role {
	switch audience {
	case "clients":
		return []domain.Role{domain.RoleClient}
	case "psychics":
		return []domain.Role{domain.RolePsychic}
	case "all":
		return []domain.Role{domain.RoleClient, domain.RolePsychic}
	}
	return nil // "users": any role (the explicit list decides)
}

// candidateList returns the sorted explicit candidates of a broadcast (chosen
// users, and/or the users online now) or nil when the whole users table is
// scanned. Recipients are always walked in user_id (byte) order, the order
// PostgreSQL sorts uuids in.
func (s *NotificationService) candidateList(ctx context.Context, b *domain.Broadcast) ([]uuid.UUID, error) {
	var list []uuid.UUID
	explicit := false
	if b.Audience == "users" {
		explicit = true
		for _, raw := range b.TargetIDs {
			if id, err := uuid.Parse(raw); err == nil {
				list = append(list, id)
			}
		}
	}
	if b.OnlineOnly {
		online, err := s.presence.OnlineUsers(ctx, 100000)
		if err != nil {
			return nil, err
		}
		if explicit {
			list = intersect(online, list)
		} else {
			list = online
		}
		explicit = true
	}
	if !explicit {
		return nil, nil
	}
	if list == nil {
		list = []uuid.UUID{}
	}
	sort.Slice(list, func(i, j int) bool { return bytes.Compare(list[i][:], list[j][:]) < 0 })
	// Drop duplicates.
	out := list[:0]
	for i, id := range list {
		if i == 0 || id != list[i-1] {
			out = append(out, id)
		}
	}
	return out, nil
}

// fanOut delivers a broadcast chunk by chunk, resuming from its cursor. Each
// chunk is idempotent (unique (broadcast_id, user_id) + ON CONFLICT DO
// NOTHING), so a re-run or a second runner never duplicates a notification.
func (s *NotificationService) fanOut(ctx context.Context, id uuid.UUID) error {
	b, err := s.repo.GetBroadcast(ctx, id)
	if err != nil {
		return err
	}
	if b.Status != "queued" && b.Status != "sending" {
		return nil
	}
	log := s.log.With(zap.String("broadcast_id", id.String()))
	if err := s.repo.UpdateBroadcast(ctx, id, map[string]any{"status": "sending", "heartbeat_at": s.clock.Now()}); err != nil {
		return err
	}
	roles := audienceRoles(b.Audience)
	candidates, err := s.candidateList(ctx, b)
	if err != nil {
		return err
	}
	cursor := b.CursorUserID
	if candidates != nil && cursor != nil {
		i := sort.Search(len(candidates), func(i int) bool { return bytes.Compare(candidates[i][:], cursor[:]) > 0 })
		candidates = candidates[i:]
	}
	data := map[string]any{"broadcast_id": b.ID, "title": map[string]any(b.Title), "body": map[string]any(b.Body)}

	for chunk := 0; ; chunk++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var ids []uuid.UUID
		var next uuid.UUID
		exhausted := false
		if candidates != nil {
			part := candidates[:min(broadcastChunk, len(candidates))]
			candidates = candidates[len(part):]
			exhausted = len(candidates) == 0
			if len(part) == 0 {
				break
			}
			if ids, err = s.repo.AudienceChunk(ctx, roles, part, nil, b.CampaignID, len(part)); err != nil {
				return err
			}
			next = part[len(part)-1]
		} else {
			if ids, err = s.repo.AudienceChunk(ctx, roles, nil, cursor, b.CampaignID, broadcastChunk); err != nil {
				return err
			}
			if len(ids) == 0 {
				break
			}
			exhausted = len(ids) < broadcastChunk
			next = ids[len(ids)-1]
		}
		now := s.clock.Now()
		rows := make([]domain.Notification, 0, len(ids))
		for _, uid := range ids {
			rows = append(rows, domain.Notification{ID: domain.NewID(), UserID: uid, Kind: NotifyBroadcast, Data: domain.JSONMap(data),
				Link: b.Link, BroadcastID: &b.ID, CreatedAt: now})
		}
		inserted, err := s.insertChunk(ctx, b.ID, rows, next, now)
		if err != nil {
			return err
		}
		cursor = &next
		s.bstats.chunks.Add(1)
		s.afterChunk(ctx, rows, inserted)
		if s.chunkHook != nil {
			if err := s.chunkHook(chunk); err != nil {
				return err
			}
		}
		if exhausted {
			break
		}
	}

	// recipients is recomputed from the rows, so it is exact whatever the
	// history of runs.
	total, err := s.repo.CountBroadcastRecipients(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateBroadcast(ctx, id, map[string]any{"status": "sent", "recipients": total, "sent_at": s.clock.Now(),
		"heartbeat_at": s.clock.Now()}); err != nil {
		return err
	}
	s.bstats.completed.Add(1)
	log.Info("broadcast delivered", zap.Int("recipients", total))
	return nil
}

// insertChunk stores one chunk with up to three attempts.
func (s *NotificationService) insertChunk(ctx context.Context, id uuid.UUID, rows []domain.Notification, cursor uuid.UUID, now time.Time) (map[uuid.UUID]bool, error) {
	var err error
	for attempt := 0; attempt < insertAttempts; attempt++ {
		var ids []uuid.UUID
		if ids, err = s.repo.InsertBroadcastChunk(ctx, id, rows, cursor, now); err == nil {
			set := make(map[uuid.UUID]bool, len(ids))
			for _, i := range ids {
				set[i] = true
			}
			return set, nil
		}
		s.bstats.chunkRetries.Add(1)
		if attempt+1 < insertAttempts && !sleepCtx(ctx, insertBackoff[min(attempt, len(insertBackoff)-1)]) {
			return nil, errors.Join(err, ctx.Err())
		}
	}
	return nil, err
}

// afterChunk publishes (pipelined) and fans out the rows this chunk actually
// inserted. It outlives a cancelled run context so a chunk that was committed
// is not left without its realtime event, e-mail and push.
func (s *NotificationService) afterChunk(ctx context.Context, rows []domain.Notification, inserted map[uuid.UUID]bool) {
	fresh := rows[:0:0]
	for i := range rows {
		if inserted[rows[i].ID] {
			fresh = append(fresh, rows[i])
		}
	}
	if len(fresh) == 0 {
		return
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	failed := publishNotifications(fctx, s.events, fresh, nil)
	if len(failed) > 0 {
		sleepCtx(fctx, publishRetryDelay)
		if failed = publishNotifications(fctx, s.events, fresh, failed); len(failed) > 0 {
			s.log.Warn("broadcast: realtime publish failed after retry", zap.Int("count", len(failed)))
		}
	}
	s.channels.Deliver(fctx, fresh)
}
