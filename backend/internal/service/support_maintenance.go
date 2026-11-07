package service

// revisions 3.0: support desk, hourly maintenance (job "support-maintenance"):
// resolved requests close after 7 days without a reply, closed requests are
// deleted after the retention period, and masked addresses of requests that
// pre-date the support desk are filled in.

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// SupportMaintenanceReport counts what a run did.
type SupportMaintenanceReport struct {
	AutoClosed int
	Deleted    int64
	Masked     int
}

const (
	supportBatch      = 200
	supportMaxBatches = 25
)

// RunMaintenance is the worker job: it runs Maintain at the current time.
func (s *SupportService) RunMaintenance(ctx context.Context) error {
	_, err := s.Maintain(ctx, s.clock.Now())
	return err
}

// Maintain runs one maintenance pass as of now. It is safe on several
// instances at once (rows are claimed with SKIP LOCKED).
func (s *SupportService) Maintain(ctx context.Context, now time.Time) (SupportMaintenanceReport, error) {
	var rep SupportMaintenanceReport
	if !s.enabled() {
		return rep, nil
	}
	closed, err := s.autoClose(ctx, now)
	rep.AutoClosed = closed
	if err != nil {
		return rep, err
	}
	for i := 0; i < supportMaxBatches; i++ {
		n, err := s.repo.DeleteClosed(ctx, now.Add(-s.desk.Settings.Retention), supportBatch)
		if err != nil {
			return rep, err
		}
		rep.Deleted += n
		if n < supportBatch {
			break
		}
	}
	rep.Masked, err = s.backfillMasks(ctx)
	if rep.AutoClosed+int(rep.Deleted)+rep.Masked > 0 {
		s.invalidateSummary(ctx)
	}
	return rep, err
}

// autoClose closes resolved requests whose last word was more than 7 days ago
// (a requester's reply re-opens a request, so what is still resolved had none).
func (s *SupportService) autoClose(ctx context.Context, now time.Time) (int, error) {
	before := now.Add(-supportAutoCloseDays * 24 * time.Hour)
	total := 0
	for i := 0; i < supportMaxBatches; i++ {
		n := 0
		err := s.desk.Tx.WithinTx(ctx, func(ctx context.Context) error {
			rows, err := s.repo.ClaimResolved(ctx, before, supportBatch)
			if err != nil {
				return err
			}
			for j := range rows {
				t := &rows[j]
				upd := map[string]any{"updated_at": now, "version": t.Version + 1}
				for k, v := range statusFields(t.Status, domain.SupportClosed, now) {
					upd[k] = v
				}
				// A system entry records why (skipped on a full thread).
				if t.MessageCount < domain.MaxSupportMessages {
					id := domain.NewID()
					sealed, err := s.sealer.Seal(msgSupportAutoClose, supportMessageAD(id))
					if err != nil {
						return err
					}
					if err := s.repo.AddMessage(ctx, &domain.SupportMessage{ID: id, TicketID: t.ID, AuthorType: domain.SupportAuthorSystem,
						Visibility: domain.SupportInternal, BodyEnc: sealed, BodyChars: utf8.RuneCountInString(msgSupportAutoClose),
						EmailStatus: domain.SupportEmailNone, CreatedAt: now}); err != nil {
						return err
					}
					upd["message_count"], upd["last_message_at"] = t.MessageCount+1, now
				}
				if err := s.repo.Update(ctx, t.ID, upd); err != nil {
					return err
				}
			}
			n = len(rows)
			return nil
		})
		total += n
		if err != nil {
			return total, err
		}
		if n < supportBatch {
			break
		}
	}
	return total, nil
}

// backfillMasks fills in the masked address of requests created before the
// support desk (their sealed address is opened once, here).
func (s *SupportService) backfillMasks(ctx context.Context) (int, error) {
	done := 0
	for i := 0; i < 5; i++ {
		rows, err := s.repo.Unmasked(ctx, supportBatch)
		if err != nil {
			return done, err
		}
		for j := range rows {
			masked := "•••"
			if email, err := s.sealer.Open(rows[j].EmailEnc, supportAD(rows[j].ID, "email")); err == nil {
				masked = MaskEmail(email)
			}
			if err := s.repo.Update(ctx, rows[j].ID, map[string]any{"email_masked": masked}); err != nil {
				return done, err
			}
			done++
		}
		if len(rows) < supportBatch {
			break
		}
	}
	return done, nil
}
