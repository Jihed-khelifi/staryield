package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// Follow-ups: auto-away. An Online advisor whose live-chat requests expire
// unanswered N times in a row (psychics.missed_requests_in_row) is set Away,
// told why, and the counter starts again. Declines are answers: they reset the
// counter like an accept or the advisor's own availability change. The
// advisor is never set Offline, and never while already Away/Offline or in a
// reading.

// Messages of the auto-away admin endpoints (translated at the API edge).
const msgAutoAwayRange = "Choose between 2 and 10."

// Threshold bounds and defaults of the advisor_auto_away setting.
const (
	AutoAwayMin     = 2
	AutoAwayMax     = 10
	AutoAwayDefault = 3
)

// AutoAwaySettings is the advisor_auto_away site setting.
type AutoAwaySettings struct {
	Enabled        bool `json:"enabled"`
	MissedRequests int  `json:"missed_requests"`
}

// defaultAutoAway is what the setting resolves to when it cannot be read, and
// what the migration seeds.
func defaultAutoAway() AutoAwaySettings {
	return AutoAwaySettings{Enabled: true, MissedRequests: AutoAwayDefault}
}

// autoAwayFrom parses the stored document: a missing document, a missing field
// or a threshold outside 2..10 falls back to the default for that field.
func autoAwayFrom(st *domain.SiteSetting) AutoAwaySettings {
	out := defaultAutoAway()
	if st == nil {
		return out
	}
	if v, ok := st.Value["enabled"].(bool); ok {
		out.Enabled = v
	}
	if n := mInt(st.Value, "missed_requests", out.MissedRequests); n >= AutoAwayMin && n <= AutoAwayMax {
		out.MissedRequests = n
	}
	return out
}

func (s AutoAwaySettings) toMap() domain.JSONMap {
	return domain.JSONMap{"enabled": s.Enabled, "missed_requests": s.MissedRequests}
}

// StatusPublisher broadcasts an advisor's current effective status.
type StatusPublisher interface {
	PublishStatus(ctx context.Context, psychicID uuid.UUID)
}

// AutoAwayDeps are the collaborators of AutoAwayService.
type AutoAwayDeps struct {
	Tx       database.TxManager
	Psychics repository.PsychicRepository
	Chat     repository.ChatRepository
	Site     repository.SiteRepository
	Audit    repository.AuditRepository
	// Advisors publishes the advisor's new status (presence and directory):
	// *PsychicService.
	Advisors StatusPublisher
	Notifier Notifier
	Tracker  analytics.Tracker
	Clock    clock.Clock
	Log      *zap.Logger
	// IPHashSecret keys the IP hash of audit entries.
	IPHashSecret string
}

// AutoAwayService counts unanswered requests and sets advisors Away.
type AutoAwayService struct{ d AutoAwayDeps }

// NewAutoAwayService wires the service; Notifier defaults to a no-op.
func NewAutoAwayService(d AutoAwayDeps) *AutoAwayService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Log == nil {
		d.Log = zap.NewNop()
	}
	return &AutoAwayService{d: d}
}

// Attach registers the service's hooks on the chat service.
func (s *AutoAwayService) Attach(chat *ChatService) {
	chat.OnRequestMissed(s.OnRequestMissed)
	chat.OnRequestAnswered(s.OnRequestAnswered)
}

// settings reads the stored setting (database, never stale). An unreadable or
// missing setting resolves to the defaults.
func (s *AutoAwayService) settings(ctx context.Context) AutoAwaySettings {
	st, err := s.d.Site.Setting(ctx, domain.SiteSettingAdvisorAutoAway)
	if err != nil {
		if !isNotFound(err) {
			warnErr(ctx, "auto-away: read settings", err)
		}
		return defaultAutoAway()
	}
	return autoAwayFrom(st)
}

// OnRequestAnswered resets the advisor's counter: they accepted or declined a
// request (the chat service calls it after the commit).
func (s *AutoAwayService) OnRequestAnswered(ctx context.Context, psychicID uuid.UUID) {
	warnErr(ctx, "auto-away: reset counter", s.d.Psychics.ResetMissed(ctx, psychicID))
}

// OnRequestMissed counts one request that expired unanswered. When the count
// reaches the threshold the counter is reset (claimed by exactly one caller,
// even when several expiries race) and, if the advisor's stored availability is
// Online and they are not in a reading, they are set Away and notified.
func (s *AutoAwayService) OnRequestMissed(ctx context.Context, psychicID uuid.UUID) {
	set := s.settings(ctx)
	if !set.Enabled {
		// Nothing is counted while the feature is off, so turning it on never
		// finds a stale count.
		warnErr(ctx, "auto-away: reset counter", s.d.Psychics.ResetMissed(ctx, psychicID))
		return
	}
	n, err := s.d.Psychics.IncrementMissed(ctx, psychicID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "auto-away: count missed request", err)
		}
		return
	}
	if n < set.MissedRequests {
		return
	}
	away := false
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		away = false
		// The claim: only the caller whose UPDATE finds the counter at the
		// threshold continues, so one burst yields one notification.
		claimed, err := s.d.Psychics.ResetMissedAtLeast(ctx, psychicID, set.MissedRequests)
		if err != nil || !claimed {
			return err
		}
		if open, err := s.d.Chat.OpenSessionForPsychic(ctx, psychicID); err == nil && open.Status != domain.SessionRequested {
			return nil // in a reading: leave them as they are
		} else if err != nil && !isNotFound(err) {
			return err
		}
		away, err = s.d.Psychics.SetAwayIfOnline(ctx, psychicID)
		return err
	})
	if err != nil {
		warnErr(ctx, "auto-away: set away", err)
		return
	}
	if !away {
		return
	}
	pctx := reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: psychicID, Role: domain.RolePsychic})
	s.d.Tracker.Track(pctx, "psychic.auto_away", map[string]any{"missed": n})
	s.d.Advisors.PublishStatus(ctx, psychicID)
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAutoAway, Data: map[string]any{"missed": set.MissedRequests}, Link: autoAwayLink}, psychicID)
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

// AutoAwayView is GET /admin/advisors/auto-away.
type AutoAwayView struct {
	Enabled        bool      `json:"enabled"`
	MissedRequests int       `json:"missed_requests"`
	Version        int       `json:"version"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      *UserRef  `json:"updated_by,omitempty"`
}

// AutoAwayInput is PUT /admin/advisors/auto-away; nil fields stay as they are.
type AutoAwayInput struct {
	Enabled        *bool `json:"enabled"`
	MissedRequests *int  `json:"missed_requests"`
	// Version is the version the admin read; a stale one is a 409.
	Version *int `json:"version"`
}

// Settings returns the setting as the admin console shows it.
func (s *AutoAwayService) Settings(ctx context.Context) (*AutoAwayView, error) {
	st, err := s.d.Site.Setting(ctx, domain.SiteSettingAdvisorAutoAway)
	if err != nil && !isNotFound(err) {
		return nil, mapRepoErr(err, "Setting")
	}
	set := autoAwayFrom(st)
	view := &AutoAwayView{Enabled: set.Enabled, MissedRequests: set.MissedRequests, Version: 1, UpdatedAt: s.d.Clock.Now()}
	if st != nil {
		view.Version, view.UpdatedAt = st.Version, st.UpdatedAt
		if st.UpdatedBy != nil {
			if names, err := s.d.Site.UserNames(ctx, []uuid.UUID{*st.UpdatedBy}); err == nil {
				if name, ok := names[*st.UpdatedBy]; ok {
					view.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
				}
			}
		}
	}
	return view, nil
}

// UpdateSettings changes the setting. A stale version is a 409
// version_conflict; an unchanged value is not a new version.
func (s *AutoAwayService) UpdateSettings(ctx context.Context, in AutoAwayInput) (*AutoAwayView, error) {
	fields := map[string]string{}
	if in.Enabled == nil && in.MissedRequests == nil {
		fields["enabled"] = msgLifecycleEmpty
	}
	if in.MissedRequests != nil && (*in.MissedRequests < AutoAwayMin || *in.MissedRequests > AutoAwayMax) {
		fields["missed_requests"] = msgAutoAwayRange
	}
	if in.Version != nil && *in.Version < 1 {
		fields["version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after AutoAwaySettings
	changed := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Site.SettingForUpdate(ctx, domain.SiteSettingAdvisorAutoAway)
		if err != nil {
			return err
		}
		if in.Version != nil && *in.Version != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		before = autoAwayFrom(st)
		after = before
		if in.Enabled != nil {
			after.Enabled = *in.Enabled
		}
		if in.MissedRequests != nil {
			after.MissedRequests = *in.MissedRequests
		}
		if after == before {
			return nil
		}
		changed = true
		return s.d.Site.SaveSetting(ctx, domain.SiteSettingAdvisorAutoAway, after.toMap(), st.Version+1, actor(ctx), s.d.Clock.Now())
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		recordAudit(ctx, s.d.Audit, s.d.Clock, s.d.IPHashSecret, "advisors.auto_away.updated", "site_setting", domain.SiteSettingAdvisorAutoAway,
			map[string]any{"before": before.toMap(), "after": after.toMap()})
	}
	return s.Settings(ctx)
}
