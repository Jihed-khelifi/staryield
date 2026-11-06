package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// memDB is an in-memory stand-in for the identity tables. memTx snapshots
// it so a failed transaction leaves no trace (atomicity tests).
type memDB struct {
	users    map[uuid.UUID]domain.User
	profiles map[uuid.UUID]memProfile
	consents map[string]domain.Consent
	events   []domain.ConsentEvent
	actions  map[uuid.UUID]domain.ActionToken
	resets   []domain.PasswordResetToken
	revoked  map[uuid.UUID]int
	psychics map[uuid.UUID]domain.Psychic
	audit    []domain.AuditLog
	// failConsentUpsert makes the consent write fail (rollback tests).
	failConsentUpsert bool
}

type memProfile struct {
	domain.Profile
	version int
}

func newMemDB() *memDB {
	return &memDB{users: map[uuid.UUID]domain.User{}, profiles: map[uuid.UUID]memProfile{}, consents: map[string]domain.Consent{},
		actions: map[uuid.UUID]domain.ActionToken{}, revoked: map[uuid.UUID]int{}, psychics: map[uuid.UUID]domain.Psychic{}}
}

func (m *memDB) clone() *memDB {
	c := *m
	c.users, c.profiles, c.consents = maps.Clone(m.users), maps.Clone(m.profiles), maps.Clone(m.consents)
	c.actions, c.revoked, c.psychics = maps.Clone(m.actions), maps.Clone(m.revoked), maps.Clone(m.psychics)
	c.events = append([]domain.ConsentEvent(nil), m.events...)
	c.resets = append([]domain.PasswordResetToken(nil), m.resets...)
	c.audit = append([]domain.AuditLog(nil), m.audit...)
	return &c
}

type memTx struct{ db *memDB }

func (t memTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	snap := t.db.clone()
	if err := fn(ctx); err != nil {
		*t.db = *snap
		return err
	}
	return nil
}

func isExpr(v any) bool {
	_, ok := v.(clause.Expr)
	return ok
}

func timePtr(v any) *time.Time {
	switch t := v.(type) {
	case time.Time:
		return &t
	case *time.Time:
		return t
	}
	return nil
}

func floatPtr(v any) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

// memUsers implements the parts of UserRepository the identity code uses.
type memUsers struct {
	repository.UserRepository
	db *memDB
}

func (r memUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.db.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}

func (r memUsers) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return r.GetByID(ctx, id)
}

func (r memUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.db.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r memUsers) GetByIDs(_ context.Context, ids []uuid.UUID) ([]domain.User, error) {
	var out []domain.User
	for _, id := range ids {
		if u, ok := r.db.users[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

func (r memUsers) Updates(_ context.Context, id uuid.UUID, fields map[string]any) error {
	u, ok := r.db.users[id]
	if !ok {
		return repository.ErrNotFound
	}
	for k, v := range fields {
		switch k {
		case "email":
			for oid, o := range r.db.users {
				if oid != id && o.Email == v.(string) {
					return &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
				}
			}
			u.Email = v.(string)
		case "display_name":
			u.DisplayName = v.(string)
		case "locale":
			u.Locale = v.(string)
		case "password_hash":
			u.PasswordHash = v.(string)
		case "status":
			u.Status = v.(domain.UserStatus)
		case "status_reason":
			u.StatusReason = v.(string)
		case "deactivated_by":
			u.DeactivatedBy = v.(string)
		case "status_changed_at":
			u.StatusChangedAt = timePtr(v)
		case "closure_scheduled_at":
			u.ClosureScheduledAt = timePtr(v)
		case "email_verified_at":
			u.EmailVerifiedAt = timePtr(v)
		case "deleted_at":
			u.DeletedAt = timePtr(v)
		case "last_login_at":
			u.LastLoginAt = timePtr(v)
		case "token_version":
			if !isExpr(v) {
				return fmt.Errorf("token_version must be an expression")
			}
			u.TokenVersion++
		default:
			return fmt.Errorf("memUsers: unexpected user column %q", k)
		}
	}
	r.db.users[id] = u
	return nil
}

func (r memUsers) GetProfile(_ context.Context, id uuid.UUID) (*domain.Profile, error) {
	p, ok := r.db.profiles[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &p.Profile, nil
}

func (r memUsers) CreateProfile(_ context.Context, p *domain.Profile) error {
	r.db.profiles[p.UserID] = memProfile{Profile: *p, version: 1}
	return nil
}

func (r memUsers) UpdateProfile(_ context.Context, id uuid.UUID, fields map[string]any) error {
	mp, ok := r.db.profiles[id]
	if !ok {
		return repository.ErrNotFound
	}
	p := &mp.Profile
	for k, v := range fields {
		switch k {
		case "settings_version":
			mp.version = v.(int)
		case "nickname":
			p.Nickname = v.(string)
		case "birth_date":
			p.BirthDate = timePtr(v)
		case "birth_time":
			p.BirthTime = v.(string)
		case "birth_place_id":
			p.BirthPlaceID = v.(string)
		case "birth_place":
			p.BirthPlace = v.(string)
		case "birth_place_label":
			p.BirthPlaceLabel = v.(string)
		case "birth_latitude":
			p.BirthLatitude = floatPtr(v)
		case "birth_longitude":
			p.BirthLongitude = floatPtr(v)
		case "birth_timezone":
			p.BirthTimezone = v.(string)
		case "country_code":
			p.CountryCode = v.(string)
		case "location":
			p.Location = v.(string)
		case "mood":
			p.Mood = v.(string)
		case "theme":
			p.Theme = v.(string)
		case "timezone":
			p.Timezone = v.(string)
		case "house_system":
			p.HouseSystem = v.(domain.HouseSystem)
		case "avatar_url":
			p.AvatarURL = v.(string)
		case "avatar_key":
			p.AvatarKey = v.(string)
		case "notify_email":
			p.NotifyEmail = v.(bool)
		case "notify_push":
			p.NotifyPush = v.(bool)
		case "notify_marketing":
			p.NotifyMarketing = v.(bool)
		case "notify_horoscope":
			p.NotifyHoroscope = v.(bool)
		case "notify_events":
			p.NotifyEvents = v.(bool)
		case "notify_chart":
			p.NotifyChart = v.(bool)
		case "share_birth_details_with_advisors":
			p.ShareBirthDetailsWithAdvisors = v.(bool)
		default:
			return fmt.Errorf("memUsers: unexpected profile column %q", k)
		}
	}
	r.db.profiles[id] = mp
	return nil
}

type memAccounts struct{ db *memDB }

func (r memAccounts) SettingsVersion(_ context.Context, id uuid.UUID, _ bool) (int, error) {
	p, ok := r.db.profiles[id]
	if !ok {
		return 0, repository.ErrNotFound
	}
	return p.version, nil
}

func (r memAccounts) DueClosures(_ context.Context, now time.Time, _ int) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for id, u := range r.db.users {
		if u.Status == domain.UserDeactivated && u.DeactivatedBy == domain.DeactivatedByClosure && u.ClosureScheduledAt != nil &&
			!u.ClosureScheduledAt.After(now) {
			out = append(out, id)
		}
	}
	return out, nil
}

type memConsents struct{ db *memDB }

func consentKey(id uuid.UUID, c domain.ConsentCategory) string { return id.String() + "|" + string(c) }

func (r memConsents) List(_ context.Context, id uuid.UUID) ([]domain.Consent, error) {
	var out []domain.Consent
	for _, c := range domain.ConsentCategories {
		if v, ok := r.db.consents[consentKey(id, c)]; ok {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r memConsents) Get(_ context.Context, id uuid.UUID, c domain.ConsentCategory) (*domain.Consent, error) {
	v, ok := r.db.consents[consentKey(id, c)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &v, nil
}

func (r memConsents) Upsert(_ context.Context, c *domain.Consent) error {
	if r.db.failConsentUpsert {
		return errors.New("consent write failed")
	}
	r.db.consents[consentKey(c.UserID, c.Category)] = *c
	return nil
}

func (r memConsents) AddEvent(_ context.Context, e *domain.ConsentEvent) error {
	r.db.events = append(r.db.events, *e)
	return nil
}

func (r memConsents) ListEvents(_ context.Context, id uuid.UUID, _ int) ([]domain.ConsentEvent, error) {
	var out []domain.ConsentEvent
	for _, e := range r.db.events {
		if e.UserID == id {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r memConsents) DeleteForUser(_ context.Context, id uuid.UUID) (int64, error) {
	var n int64
	for k := range r.db.consents {
		if strings.HasPrefix(k, id.String()+"|") {
			delete(r.db.consents, k)
			n++
		}
	}
	return n, nil
}

type memActions struct{ db *memDB }

func (r memActions) Create(_ context.Context, t *domain.ActionToken) error {
	for _, o := range r.db.actions {
		if o.TokenHash == t.TokenHash {
			return repository.ErrDuplicate
		}
	}
	r.db.actions[t.ID] = *t
	return nil
}

func (r memActions) InvalidateOpen(_ context.Context, userID uuid.UUID, p domain.ActionPurpose, at time.Time) error {
	for id, t := range r.db.actions {
		if t.UserID == userID && t.Purpose == p && t.UsedAt == nil {
			t.UsedAt = &at
			r.db.actions[id] = t
		}
	}
	return nil
}

func (r memActions) Consume(_ context.Context, hash string, p domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	for id, t := range r.db.actions {
		if t.TokenHash == hash && t.Purpose == p && t.UsedAt == nil && t.ExpiresAt.After(now) {
			t.UsedAt = &now
			r.db.actions[id] = t
			return &t, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r memActions) LatestOpen(_ context.Context, userID uuid.UUID, p domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	var best *domain.ActionToken
	for _, t := range r.db.actions {
		if t.UserID == userID && t.Purpose == p && t.UsedAt == nil && t.ExpiresAt.After(now) && (best == nil || t.CreatedAt.After(best.CreatedAt)) {
			t := t
			best = &t
		}
	}
	if best == nil {
		return nil, repository.ErrNotFound
	}
	return best, nil
}

func (r memActions) DeleteForUser(_ context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	for id, t := range r.db.actions {
		if t.UserID == userID {
			delete(r.db.actions, id)
			n++
		}
	}
	return n, nil
}

func (r memActions) DeleteExpired(context.Context, time.Time) (int64, error) { return 0, nil }

func (r memActions) open(userID uuid.UUID, p domain.ActionPurpose) []domain.ActionToken {
	var out []domain.ActionToken
	for _, t := range r.db.actions {
		if t.UserID == userID && t.Purpose == p && t.UsedAt == nil {
			out = append(out, t)
		}
	}
	return out
}

type memTokens struct {
	repository.TokenRepository
	db *memDB
}

func (r memTokens) RevokeAllForUser(_ context.Context, id uuid.UUID, _ string, _ time.Time) (int64, error) {
	r.db.revoked[id]++
	return 1, nil
}

func (r memTokens) InvalidateResets(context.Context, uuid.UUID, time.Time) error { return nil }

func (r memTokens) CreateReset(_ context.Context, t *domain.PasswordResetToken) error {
	r.db.resets = append(r.db.resets, *t)
	return nil
}

type memPsychics struct {
	repository.PsychicRepository
	db *memDB
}

func (r memPsychics) Updates(_ context.Context, id uuid.UUID, fields map[string]any) error {
	p, ok := r.db.psychics[id]
	if !ok {
		return repository.ErrNotFound
	}
	for k, v := range fields {
		switch k {
		case "listed":
			p.Listed = v.(bool)
		case "featured":
			p.Featured = v.(bool)
		case "availability":
			p.Availability = v.(domain.Availability)
		}
	}
	r.db.psychics[id] = p
	return nil
}

type memAudit struct {
	repository.AuditRepository
	db *memDB
}

func (r memAudit) Create(_ context.Context, a *domain.AuditLog) error {
	r.db.audit = append(r.db.audit, *a)
	return nil
}

// memMail records enqueued e-mail (it lives outside memDB: the outbox is
// part of the transaction in production, so tests read it directly).
type memMail struct{ sent []EmailRequest }

func (m *memMail) Enqueue(_ context.Context, reqs ...EmailRequest) error {
	m.sent = append(m.sent, reqs...)
	return nil
}

func (m *memMail) byTemplate(name string) []EmailRequest {
	var out []EmailRequest
	for _, r := range m.sent {
		if r.Template == name {
			out = append(out, r)
		}
	}
	return out
}

// linkToken extracts ?token= from an e-mail's call to action.
func linkToken(r EmailRequest) string {
	if r.Content.Action == nil {
		return ""
	}
	_, tok, _ := strings.Cut(r.Content.Action.URL, "token=")
	return tok
}

// Recorders for the eraser's optional collaborators.
type eraseCalls struct {
	push, exports, sessions int
	exportKeys              []string
}

type memPush struct {
	repository.PushRepository
	calls *eraseCalls
}

func (r memPush) DeleteAllForUser(context.Context, uuid.UUID) (int64, error) {
	r.calls.push++
	return 1, nil
}

type memExports struct {
	repository.ExportRepository
	calls *eraseCalls
}

func (r memExports) ListAllForUser(_ context.Context, id uuid.UUID) ([]domain.DataExportJob, error) {
	var out []domain.DataExportJob
	for _, k := range r.calls.exportKeys {
		out = append(out, domain.DataExportJob{ID: uuid.New(), UserID: id, ObjectKey: k, Status: domain.ExportReady})
	}
	return out, nil
}

func (r memExports) SoftDeleteForUser(context.Context, uuid.UUID, time.Time) (int64, error) {
	r.calls.exports++
	return 1, nil
}

type memSessions struct {
	repository.SessionRepository
	calls *eraseCalls
}

func (r memSessions) DeleteForUser(context.Context, uuid.UUID) (int64, error) {
	r.calls.sessions++
	return 1, nil
}

type failingBilling struct{ err error }

func (b failingBilling) CancelForErasure(context.Context, uuid.UUID) error { return b.err }

// memWallets has no wallets (Me then omits the wallet).
type memWallets struct{ repository.WalletRepository }

func (memWallets) Get(context.Context, uuid.UUID) (*domain.Wallet, error) {
	return nil, repository.ErrNotFound
}

// memDevices answers device revocation lookups (Authenticate's database
// fallback) and records activity.
type memDevices struct {
	repository.SessionRepository
	revoked map[uuid.UUID]bool
	touched map[uuid.UUID]time.Time
}

func (r *memDevices) IsRevoked(_ context.Context, id uuid.UUID) (bool, error) {
	return r.revoked[id], nil
}

func (r *memDevices) Touch(_ context.Context, id uuid.UUID, at time.Time, _ *time.Time) error {
	r.touched[id] = at
	return nil
}

// downCache is a cache that is unreachable.
type downCache struct{}

var errCacheDown = errors.New("cache down")

func (downCache) Get(context.Context, string) (string, bool, error)          { return "", false, errCacheDown }
func (downCache) Set(context.Context, string, string, time.Duration) error   { return errCacheDown }
func (downCache) Del(context.Context, ...string) error                       { return errCacheDown }
func (downCache) Incr(context.Context, string, time.Duration) (int64, error) { return 0, errCacheDown }
func (downCache) TTL(context.Context, string) (time.Duration, error)         { return 0, errCacheDown }
func (downCache) MGet(context.Context, ...string) ([]string, []bool, error) {
	return nil, nil, errCacheDown
}
