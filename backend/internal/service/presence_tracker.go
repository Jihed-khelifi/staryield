package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// PresenceJob reconciles presence history with live status.
const PresenceJob = "presence-history"

// StatusLookup computes advisors' effective statuses in one batch.
type StatusLookup func(ctx context.Context, list []domain.Psychic) (map[uuid.UUID]domain.PresenceStatus, error)

// PresenceTracker turns published status changes into availability history
// (psychic_presence_intervals) and keeps psychics.last_online_at current.
// The history feeds the "Active hours" KPI; last_online_at ranks advisors
// on the home page.
type PresenceTracker struct {
	tx       database.TxManager
	repo     repository.PresenceRepository
	statuses StatusLookup
	clock    clock.Clock
	// onChange runs after the history changed (directory cache).
	onChange func()
}

// NewPresenceTracker wires the tracker.
func NewPresenceTracker(tx database.TxManager, repo repository.PresenceRepository, statuses StatusLookup, clk clock.Clock,
	onChange func()) *PresenceTracker {
	if onChange == nil {
		onChange = func() {}
	}
	return &PresenceTracker{tx: tx, repo: repo, statuses: statuses, clock: clk, onChange: onChange}
}

// historyStatus maps an effective status to the interval it opens ("" for
// offline, which closes the open interval).
func historyStatus(st domain.PresenceStatus) domain.PresenceStatus {
	switch st {
	case domain.PresenceOnline, domain.PresenceBusy, domain.PresenceAway:
		return st
	}
	return ""
}

func countsAsOnline(st domain.PresenceStatus) bool {
	return st == domain.PresenceOnline || st == domain.PresenceBusy
}

// Observe records an advisor's newly published status. It never fails the
// caller: errors are logged. A concurrent Observe for the same advisor may
// hit the one-open-interval unique index; the transition is then retried
// once against the interval that won.
func (t *PresenceTracker) Observe(ctx context.Context, psychicID uuid.UUID, st domain.PresenceStatus) {
	// Always its own transaction: an error here must never abort a caller's
	// transaction (PostgreSQL poisons a transaction after any failed statement).
	ctx = database.Detach(ctx)
	err := t.observe(ctx, psychicID, st)
	if err != nil && database.IsUniqueViolation(err) {
		err = t.observe(ctx, psychicID, st)
	}
	warnErr(ctx, "presence: record status change", err)
}

func (t *PresenceTracker) observe(ctx context.Context, psychicID uuid.UUID, st domain.PresenceStatus) error {
	now := t.clock.Now()
	want := historyStatus(st)
	changed := false
	err := t.tx.WithinTx(ctx, func(ctx context.Context) error {
		open, err := t.repo.OpenForUpdate(ctx, psychicID)
		if err != nil {
			return err
		}
		if (open != nil && open.Status == want) || (open == nil && want == "") {
			return nil // nothing to record
		}
		changed = true
		if open != nil {
			if err := t.repo.Close(ctx, open.ID, now); err != nil {
				return err
			}
			if countsAsOnline(open.Status) {
				if err := t.repo.TouchLastOnline(ctx, psychicID, now); err != nil {
					return err
				}
			}
		}
		if want == "" {
			return nil
		}
		if err := t.repo.Start(ctx, &domain.PresenceInterval{ID: domain.NewID(), PsychicID: psychicID, Status: want,
			StartedAt: now, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		if countsAsOnline(want) {
			return t.repo.TouchLastOnline(ctx, psychicID, now)
		}
		return nil
	})
	if err == nil && changed {
		t.onChange()
	}
	return err
}

// Reconcile fixes open intervals whose advisor's live status changed without
// a published event (a crashed connection whose presence simply expired).
// It is safe to run on several instances at once.
func (t *PresenceTracker) Reconcile(ctx context.Context) error {
	open, err := t.repo.OpenIntervals(ctx, 5000)
	if err != nil || len(open) == 0 {
		return err
	}
	ids := make([]uuid.UUID, len(open))
	for i, iv := range open {
		ids[i] = iv.PsychicID
	}
	avail, err := t.repo.AvailabilityOf(ctx, ids)
	if err != nil {
		return err
	}
	list := make([]domain.Psychic, 0, len(open))
	for _, id := range ids {
		av, ok := avail[id]
		if !ok {
			av = domain.AvailabilityOffline // deleted advisor: close as offline
		}
		list = append(list, domain.Psychic{UserID: id, Availability: av})
	}
	statuses, err := t.statuses(ctx, list)
	if err != nil {
		return err
	}
	for _, iv := range open {
		st, ok := statuses[iv.PsychicID]
		if !ok {
			st = domain.PresenceOffline
		}
		if historyStatus(st) != iv.Status {
			t.Observe(ctx, iv.PsychicID, st)
		}
	}
	return nil
}
