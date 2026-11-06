package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mileusna/useragent"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Signed-in devices. Every refresh-token family is one device
// (user_sessions row, id = family id, also carried by access tokens).
// Signing a device out revokes its refresh tokens and sets a Redis marker
// (auth:famrev:<family>) for the lifetime of an access token, which
// Authenticate checks together with the account status in one round trip,
// so the device's access token stops working at once. Its realtime
// connections are closed too; other devices are untouched.

const (
	famrevPrefix = "auth:famrev:"
	seenPrefix   = "auth:seen:"
	// lastSeenEvery throttles user_sessions.last_seen_at writes per device.
	lastSeenEvery = time.Minute
	// famrevGrace covers clock skew and in-flight requests beyond the
	// access-token lifetime.
	famrevGrace = 60 * time.Second
	// reactivationTTL bounds the token offered when signing in to a
	// self-deactivated (or closing) account.
	reactivationTTL = 5 * time.Minute
)

func famrevKey(id uuid.UUID) string { return famrevPrefix + id.String() }
func seenKey(id uuid.UUID) string   { return seenPrefix + id.String() }

// authIdentity holds the optional device-management collaborators.
type authIdentity struct {
	sessions repository.SessionRepository
	actions  repository.ActionTokenRepository
}

// WithIdentity enables signed-in device records, immediate single-device
// sign-out and the reactivation offer for self-deactivated accounts.
func (s *AuthService) WithIdentity(sessions repository.SessionRepository, actions repository.ActionTokenRepository) *AuthService {
	s.ident = &authIdentity{sessions: sessions, actions: actions}
	return s
}

// familyRevoker closes the realtime connections of one device
// (implemented by realtime.Publisher).
type familyRevoker interface {
	RevokeFamily(ctx context.Context, userID, family uuid.UUID) error
}

// DeviceInfo is what a User-Agent header says about a device.
type DeviceInfo struct {
	Label   string
	Type    string // desktop | mobile | tablet | bot | unknown
	Browser string
	OS      string
}

// DescribeUserAgent turns a User-Agent header into a short device label
// such as "Chrome on macOS" or "Safari on iPhone". Browsers do not reveal
// hardware models, so labels name the browser and the platform only.
func DescribeUserAgent(raw string) DeviceInfo {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DeviceInfo{Label: "Unknown device", Type: "unknown"}
	}
	ua := useragent.Parse(raw)
	browser := ua.Name
	switch browser {
	case useragent.MobileSafari:
		browser = useragent.Safari
	case useragent.HeadlessChrome:
		browser = useragent.Chrome
	case useragent.Msie:
		browser = useragent.InternetExplorer
	}
	platform := ua.OS
	switch ua.OS {
	case useragent.IOS:
		switch {
		case strings.Contains(raw, "iPad"):
			platform = "iPad"
		case strings.Contains(raw, "iPod"):
			platform = "iPod"
		case strings.Contains(raw, "iPhone"):
			platform = "iPhone"
		}
	case useragent.CrOS:
		platform = useragent.ChromeOS
	case useragent.WindowsNT, useragent.WindowsPhoneOS:
		platform = useragent.Windows
	}
	kind := "unknown"
	switch {
	case ua.Bot:
		kind = "bot"
	case ua.Tablet || platform == "iPad":
		kind = "tablet"
	case ua.Mobile:
		kind = "mobile"
	case ua.Desktop:
		kind = "desktop"
	}
	browser, platform = clip(browser, 40), clip(platform, 40)
	label := "Unknown device"
	switch {
	case browser != "" && platform != "":
		label = browser + " on " + platform
	case browser != "":
		label = browser
	case platform != "":
		label = platform
	}
	return DeviceInfo{Label: clip(label, 80), Type: kind, Browser: browser, OS: platform}
}

// clip truncates s to at most n bytes on a rune boundary.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// recordDevice creates the device row of a new family (before its first
// refresh token, which references it) or records activity on rotation.
func (s *AuthService) recordDevice(ctx context.Context, rt *domain.RefreshToken, newFamily bool) error {
	if s.ident == nil {
		return nil
	}
	if !newFamily {
		return s.ident.sessions.Touch(ctx, rt.FamilyID, rt.CreatedAt, &rt.ExpiresAt)
	}
	info := DescribeUserAgent(rt.UserAgent)
	return s.ident.sessions.Create(ctx, &domain.UserSession{
		ID: rt.FamilyID, UserID: rt.UserID, UserAgent: rt.UserAgent, DeviceLabel: info.Label, DeviceType: info.Type,
		Browser: info.Browser, OS: info.OS, IPHash: rt.IPHash, LastSeenAt: rt.CreatedAt, ExpiresAt: rt.ExpiresAt,
		CreatedAt: rt.CreatedAt, UpdatedAt: rt.CreatedAt,
	})
}

// authState is the account status plus the device's revocation state.
type authState struct {
	statusEntry
	revoked bool // the device (family) was signed out
	seen    bool // activity was recorded within lastSeenEvery
}

// authState reads the cached account status, the device's revocation
// marker and its last-seen marker in one round trip. Without the cache the
// database answers (status and user_sessions.revoked_at).
func (s *AuthService) authState(ctx context.Context, userID, family uuid.UUID) (authState, error) {
	if s.ident == nil || family == uuid.Nil {
		st, err := s.status(ctx, userID)
		return authState{statusEntry: st, seen: true}, err
	}
	if mg, ok := s.kv.(cache.MultiGetter); ok {
		vals, found, err := mg.MGet(ctx, statusKey(userID), famrevKey(family), seenKey(family))
		if err == nil {
			out := authState{revoked: found[1], seen: found[2]}
			if out.revoked {
				return out, nil
			}
			if e, valid := decodeStatus(vals[0]); found[0] && valid {
				out.statusEntry = e
				return out, nil
			}
			st, err := s.status(ctx, userID)
			out.statusEntry = st
			return out, err
		}
		logger.FromContext(ctx).Warn("auth: cache unavailable; checking the device in the database", zap.Error(err))
	} else if _, found, err := s.kv.Get(ctx, famrevKey(family)); err == nil {
		if found {
			return authState{revoked: true}, nil
		}
		_, seen, _ := s.kv.Get(ctx, seenKey(family))
		st, err := s.status(ctx, userID)
		return authState{statusEntry: st, seen: seen}, err
	}
	revoked, err := s.ident.sessions.IsRevoked(ctx, family)
	if err != nil {
		return authState{}, apperror.Internal(err)
	}
	if revoked {
		return authState{revoked: true}, nil
	}
	st, err := s.status(ctx, userID)
	// While the cache is down activity is not recorded (no throttle).
	return authState{statusEntry: st, seen: true}, err
}

// markSeen records device activity at most once per lastSeenEvery (the
// counter makes exactly one concurrent request win the write).
func (s *AuthService) markSeen(ctx context.Context, family uuid.UUID) {
	if s.ident == nil || family == uuid.Nil {
		return
	}
	n, err := s.kv.Incr(ctx, seenKey(family), lastSeenEvery)
	if err != nil || n != 1 {
		return
	}
	warnErr(ctx, "auth: record device activity", s.ident.sessions.Touch(ctx, family, s.clock.Now(), nil))
}

// afterFamilyRevoked makes a signed-out device's access token fail at once
// and closes that device's realtime connections (other devices stay).
func (s *AuthService) afterFamilyRevoked(ctx context.Context, userID, family uuid.UUID) {
	if err := s.kv.Set(ctx, famrevKey(family), "1", s.issuer.TTL()+famrevGrace); err != nil {
		// Without the marker the access token would live until it expires:
		// invalidate every access token instead (other devices refresh).
		warnErr(ctx, "auth: device revocation marker", err)
		if err := s.users.Updates(ctx, userID, map[string]any{"token_version": gorm.Expr("token_version + 1")}); err != nil {
			warnErr(ctx, "auth: bump token_version", err)
		}
		s.InvalidateStatus(ctx, userID)
	}
	if fr, ok := s.events.(familyRevoker); ok {
		warnErr(ctx, "auth: realtime device revoke", fr.RevokeFamily(ctx, userID, family))
	} else {
		warnErr(ctx, "auth: realtime revoke", s.events.Revoke(ctx, userID))
	}
}

// offerReactivation answers a correct sign-in on an account its owner
// paused or scheduled for closure with a short-lived reactivation token.
// It returns nil when the account is not self-deactivated.
func (s *AuthService) offerReactivation(ctx context.Context, user *domain.User) error {
	if s.ident == nil || user.Status != domain.UserDeactivated ||
		(user.DeactivatedBy != domain.DeactivatedBySelf && user.DeactivatedBy != domain.DeactivatedByClosure) {
		return nil
	}
	raw, err := security.RandomToken(32)
	if err != nil {
		return apperror.Internal(err)
	}
	now := s.clock.Now()
	exp := now.Add(reactivationTTL)
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.ident.actions.InvalidateOpen(ctx, user.ID, domain.ActionReactivate, now); err != nil {
			return err
		}
		return s.ident.actions.Create(ctx, &domain.ActionToken{ID: domain.NewID(), UserID: user.ID, Purpose: domain.ActionReactivate,
			TokenHash: security.HashToken(raw), ExpiresAt: exp, CreatedAt: now, UpdatedAt: now})
	})
	if err != nil {
		return apperror.Internal(err)
	}
	s.tracker.Track(reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: user.ID, Role: user.Role}), "user.reactivation_offered",
		map[string]any{"by": user.DeactivatedBy})
	if user.DeactivatedBy == domain.DeactivatedByClosure && user.ClosureScheduledAt != nil {
		return apperror.ClosurePending(raw, exp, *user.ClosureScheduledAt)
	}
	return apperror.AccountDeactivated(raw, exp)
}

// DeviceView is a signed-in device as shown in the security settings.
type DeviceView struct {
	ID         uuid.UUID `json:"id"`
	Label      string    `json:"label"`
	DeviceType string    `json:"device_type"`
	Browser    string    `json:"browser"`
	OS         string    `json:"os"`
	// Current marks the device making the request.
	Current    bool      `json:"current"`
	SignedInAt time.Time `json:"signed_in_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func deviceView(d *domain.UserSession, current uuid.UUID) DeviceView {
	label, kind, browser, os := d.DeviceLabel, d.DeviceType, d.Browser, d.OS
	if label == "" { // rows backfilled from refresh tokens
		info := DescribeUserAgent(d.UserAgent)
		label, kind, browser, os = info.Label, info.Type, info.Browser, info.OS
	}
	return DeviceView{ID: d.ID, Label: label, DeviceType: kind, Browser: browser, OS: os, Current: d.ID == current,
		SignedInAt: d.CreatedAt, LastSeenAt: d.LastSeenAt, ExpiresAt: d.ExpiresAt}
}

func (s *AuthService) requireIdentity() error {
	if s.ident == nil {
		return apperror.FeatureUnavailable("Device management is not available.")
	}
	return nil
}

// Devices pages the signed-in devices of the caller, most recent first.
func (s *AuthService) Devices(ctx context.Context, p reqctx.Principal, cursor string, limit int) (Page[DeviceView], error) {
	if err := s.requireIdentity(); err != nil {
		return Page[DeviceView]{}, err
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[DeviceView]{}, err
	}
	size := limit
	if size <= 0 || size > 50 {
		size = 20
	}
	rows, err := s.ident.sessions.ListActive(ctx, p.UserID, s.clock.Now(), c, size+1)
	if err != nil {
		return Page[DeviceView]{}, apperror.Internal(err)
	}
	page := Page[DeviceView]{Items: make([]DeviceView, 0, min(len(rows), size))}
	for i := range rows {
		if i == size {
			last := rows[size-1]
			page.NextCursor = repository.EncodeCursor(last.LastSeenAt, last.ID.String())
			break
		}
		page.Items = append(page.Items, deviceView(&rows[i], p.FamilyID))
	}
	return page, nil
}

// RevokeDevice signs one of the caller's devices out. It answers 404 for
// devices of other users (no existence oracle) and is idempotent.
func (s *AuthService) RevokeDevice(ctx context.Context, p reqctx.Principal, id uuid.UUID) error {
	if err := s.requireIdentity(); err != nil {
		return err
	}
	d, err := s.ident.sessions.Get(ctx, id)
	if err != nil || d.UserID != p.UserID {
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return apperror.Internal(err)
		}
		return apperror.NotFound("Device")
	}
	if d.RevokedAt != nil {
		return nil
	}
	if _, err := s.tokens.RevokeFamily(ctx, id, "device_signed_out", s.clock.Now()); err != nil {
		return apperror.Internal(err)
	}
	s.afterFamilyRevoked(ctx, p.UserID, id)
	s.tracker.Track(ctx, "user.device_signed_out", map[string]any{"current": id == p.FamilyID})
	return nil
}

// RevokeOtherDevices signs every device of the caller out except the one
// making the request, and returns how many were signed out.
func (s *AuthService) RevokeOtherDevices(ctx context.Context, p reqctx.Principal) (int, error) {
	if err := s.requireIdentity(); err != nil {
		return 0, err
	}
	now := s.clock.Now()
	ids, err := s.ident.sessions.ActiveIDs(ctx, p.UserID, p.FamilyID, now)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			if _, err := s.tokens.RevokeFamily(ctx, id, "signed_out_elsewhere", now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	for _, id := range ids {
		s.afterFamilyRevoked(ctx, p.UserID, id)
	}
	s.tracker.Track(ctx, "user.devices_signed_out", map[string]any{"count": len(ids)})
	return len(ids), nil
}

// realtimeTicket issues a ticket bound to the caller's device when the
// store supports it.
func (s *AuthService) realtimeTicket(ctx context.Context, p reqctx.Principal) (string, error) {
	if ft, ok := s.tickets.(realtime.FamilyTickets); ok {
		return ft.IssueFamily(ctx, p.UserID, string(p.Role), p.FamilyID, s.cfg.TicketTTL)
	}
	return s.tickets.Issue(ctx, p.UserID, string(p.Role), s.cfg.TicketTTL)
}
