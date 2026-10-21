package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// AdminUserRow is a user in admin listings.
type AdminUserRow struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	StatusReason string     `json:"status_reason,omitempty"`
	DisplayName  string     `json:"display_name"`
	Locale       string     `json:"locale"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// AdminProfile is the subset of a profile administrators see.
type AdminProfile struct {
	Nickname    string `json:"nickname"`
	BirthDate   string `json:"birth_date,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Location    string `json:"location"`
	AvatarURL   string `json:"avatar_url"`
}

// AdminUserView is a user as seen by administrators.
type AdminUserView struct {
	AdminUserRow
	DeletedAt   *time.Time            `json:"deleted_at,omitempty"`
	Wallet      *WalletView           `json:"wallet,omitempty"`
	Profile     *AdminProfile         `json:"profile,omitempty"`
	Attribution *AttributionView      `json:"attribution,omitempty"`
	Stats       *repository.UserStats `json:"stats,omitempty"`
	MFAEnabled  *bool                 `json:"mfa_enabled,omitempty"`
}

// AuditView is an audit log entry.
type AuditView struct {
	ID         uuid.UUID      `json:"id"`
	ActorID    *uuid.UUID     `json:"actor_id,omitempty"`
	ActorEmail string         `json:"actor_email,omitempty"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Metadata   map[string]any `json:"metadata"`
	RequestID  string         `json:"request_id"`
	CreatedAt  time.Time      `json:"created_at"`
}

// UserUpdate is an admin change to an account (BR-09: only admins
// deactivate accounts).
type UserUpdate struct {
	Status       *domain.UserStatus `json:"status"`
	StatusReason *string            `json:"status_reason"`
	Role         *domain.Role       `json:"role"`
	DisplayName  *string            `json:"display_name"`
	Locale       *string            `json:"locale"`
	Email        *string            `json:"email"`
}

// CreateUserInput creates a client or administrator account.
type CreateUserInput struct {
	Email       string      `json:"email"`
	Password    string      `json:"password"`
	Role        domain.Role `json:"role"`
	DisplayName string      `json:"display_name"`
	Locale      string      `json:"locale"`
}

// PsychicInput creates or updates an advisor (admin only).
type PsychicInput struct {
	Email           *string   `json:"email"`
	Password        *string   `json:"password"`
	Name            *string   `json:"name"`
	Slug            *string   `json:"slug"`
	Headline        *string   `json:"headline"`
	About           *[]string `json:"about"`
	Philosophy      *string   `json:"philosophy"`
	ImageURL        *string   `json:"image_url"`
	Zodiac          *string   `json:"zodiac"`
	Skills          *[]string `json:"skills"`
	YearsExperience *int      `json:"years_experience"`
	RatePerMinute   *int64    `json:"rate_per_minute"`
	Listed          *bool     `json:"listed"`
	SortOrder       *int      `json:"sort_order"`
	Rating          *float64  `json:"rating"`
	ReviewsCount    *int      `json:"reviews_count"`
	Featured        *bool     `json:"featured"`
	FeaturedRank    *int      `json:"featured_rank"`
}

// ServiceInput creates or updates a psychic service.
type ServiceInput struct {
	Name         *string `json:"name"`
	Subtitle     *string `json:"subtitle"`
	Description  *string `json:"description"`
	PriceCredits *int64  `json:"price_credits"`
	Active       *bool   `json:"active"`
	SortOrder    *int    `json:"sort_order"`
}

// PackInput creates or updates a credit pack (BR-09).
type PackInput struct {
	Name       *string `json:"name"`
	Credits    *int64  `json:"credits"`
	PriceCents *int64  `json:"price_cents"`
	Currency   *string `json:"currency"`
	Active     *bool   `json:"active"`
	Featured   *bool   `json:"featured"`
	SortOrder  *int    `json:"sort_order"`
}

// AdminDeps are the admin service's collaborators.
type AdminDeps struct {
	Tx           database.TxManager
	Users        repository.UserRepository
	Wallets      repository.WalletRepository
	Tokens       repository.TokenRepository
	Psychics     repository.PsychicRepository
	Credits      repository.CreditRepository
	Audit        repository.AuditRepository
	Chat         repository.ChatRepository
	Growth       repository.GrowthRepository
	Insights     repository.InsightsRepository
	Wallet       *WalletService
	Auth         *AuthService
	AdminAuth    *AdminAuthService
	ChatSvc      *ChatService
	PsychicSvc   *PsychicService
	Events       EventPublisher
	Notifier     Notifier
	Presence     realtime.Presence
	Hasher       *security.PasswordHasher
	AvatarStore  storage.ObjectStore
	AdvisorStore storage.ObjectStore
	Clock        clock.Clock
	IPSecret     string
	MaxAdjust    int64
	Currency     string
	AdvisorImage ImagePolicy
}

// AdminService implements the administrative console API (FR-13, FR-14).
type AdminService struct {
	tx           database.TxManager
	users        repository.UserRepository
	wallets      repository.WalletRepository
	tokens       repository.TokenRepository
	psychics     repository.PsychicRepository
	credits      repository.CreditRepository
	audit        repository.AuditRepository
	chatRepo     repository.ChatRepository
	growth       repository.GrowthRepository
	insights     repository.InsightsRepository
	wallet       *WalletService
	auth         *AuthService
	adminAuth    *AdminAuthService
	chat         *ChatService
	psySvc       *PsychicService
	events       EventPublisher
	notify       Notifier
	presence     realtime.Presence
	hasher       *security.PasswordHasher
	avatarStore  storage.ObjectStore
	advisorStore storage.ObjectStore
	clock        clock.Clock
	ipSecret     string
	maxAdj       int64
	currency     string
	advisorImage ImagePolicy
}

// NewAdminService wires the admin service.
func NewAdminService(d AdminDeps) *AdminService {
	notify := d.Notifier
	if notify == nil {
		notify = NoopNotifier{}
	}
	return &AdminService{tx: d.Tx, users: d.Users, wallets: d.Wallets, tokens: d.Tokens, psychics: d.Psychics, credits: d.Credits,
		audit: d.Audit, chatRepo: d.Chat, growth: d.Growth, insights: d.Insights, wallet: d.Wallet, auth: d.Auth, adminAuth: d.AdminAuth,
		chat: d.ChatSvc, psySvc: d.PsychicSvc, events: d.Events, notify: notify, presence: d.Presence, hasher: d.Hasher,
		avatarStore: d.AvatarStore, advisorStore: d.AdvisorStore, clock: d.Clock, ipSecret: d.IPSecret, maxAdj: d.MaxAdjust,
		currency: d.Currency, advisorImage: d.AdvisorImage}
}

// record writes an audit entry; failures are logged loudly but do not undo
// the action that already happened.
func (s *AdminService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	recordAudit(ctx, s.audit, s.clock, s.ipSecret, action, targetType, targetID, meta)
}

func adminUserRow(u *domain.User) AdminUserRow {
	return AdminUserRow{ID: u.ID, Email: u.Email, Role: string(u.Role), Status: string(u.Status), StatusReason: u.StatusReason,
		DisplayName: u.DisplayName, Locale: u.Locale, CreatedAt: u.CreatedAt, LastLoginAt: u.LastLoginAt}
}

// Users lists accounts with optional search and filters.
func (s *AdminService) Users(ctx context.Context, q, role, status, cursor string, limit int) (Page[AdminUserRow], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminUserRow]{}, err
	}
	if role != "" && !domain.Role(role).Valid() {
		return Page[AdminUserRow]{}, apperror.Validation(map[string]string{"role": "Unknown role."})
	}
	if status != "" && !domain.UserStatus(status).Valid() {
		return Page[AdminUserRow]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	rows, err := s.users.List(ctx, repository.UserFilter{Query: truncateString(strings.TrimSpace(q), 100), Role: domain.Role(role),
		Status: domain.UserStatus(status), Cursor: c, Limit: limit})
	if err != nil {
		return Page[AdminUserRow]{}, apperror.Internal(err)
	}
	page := Page[AdminUserRow]{Items: make([]AdminUserRow, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, adminUserRow(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 50, 200) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// User returns one account with its wallet, profile, attribution and stats.
func (s *AdminService) User(ctx context.Context, id uuid.UUID) (*AdminUserView, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	v := &AdminUserView{AdminUserRow: adminUserRow(u), DeletedAt: u.DeletedAt}
	if w, err := s.wallets.Get(ctx, id); err == nil {
		wv := walletView(w)
		v.Wallet = &wv
	}
	if p, err := s.users.GetProfile(ctx, id); err == nil {
		ap := &AdminProfile{Nickname: p.Nickname, CountryCode: p.CountryCode, Location: p.Location, AvatarURL: p.AvatarURL}
		if p.BirthDate != nil {
			ap.BirthDate = p.BirthDate.Format("2006-01-02")
		}
		v.Profile = ap
	}
	if s.growth != nil {
		if a, err := s.growth.GetAttribution(ctx, id); err == nil {
			av := attributionView(ctx, s.growth, a)
			v.Attribution = &av
		}
	}
	if s.insights != nil {
		if st, err := s.insights.UserStats(ctx, id, s.currency); err == nil {
			v.Stats = st
		} else {
			warnErr(ctx, "admin: user stats", err)
		}
	}
	if u.Role == domain.RoleAdmin && s.adminAuth != nil {
		enabled := s.adminAuth.HasMFA(ctx, id)
		v.MFAEnabled = &enabled
	}
	return v, nil
}

// CreateUser creates a client or administrator account. Creating an
// administrator requires a recent second-factor confirmation.
func (s *AdminService) CreateUser(ctx context.Context, in CreateUserInput) (*AdminUserView, error) {
	fields := map[string]string{}
	email, ok := NormalizeEmail(in.Email)
	if !ok {
		fields["email"] = "Enter a valid email address."
	}
	if msg := security.ValidatePassword(in.Password, email); msg != "" {
		fields["password"] = msg
	}
	switch in.Role {
	case domain.RoleClient, domain.RoleAdmin:
	case domain.RolePsychic:
		fields["role"] = "Create advisors from the advisors page."
	default:
		fields["role"] = "Choose client or admin."
	}
	name := CleanLine(in.DisplayName)
	if utf8.RuneCountInString(name) > 80 {
		fields["display_name"] = "Use at most 80 characters."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if in.Role == domain.RoleAdmin {
		if err := s.adminAuth.RequireStepUp(ctx); err != nil {
			return nil, err
		}
	}
	if name == "" {
		name = email[:strings.IndexByte(email, '@')]
		name = truncateString(name, 40)
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	user := &domain.User{ID: domain.NewID(), Email: email, PasswordHash: hash, Role: in.Role, Status: domain.UserActive,
		DisplayName: name, Locale: normalizeLocale(in.Locale), TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Create(ctx, user); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				return apperror.Conflict(apperror.CodeEmailTaken, "An account with this email already exists.")
			}
			return err
		}
		if err := s.users.CreateProfile(ctx, newProfile(user.ID, nil, now)); err != nil {
			return err
		}
		return s.wallets.Create(ctx, &domain.Wallet{UserID: user.ID, UpdatedAt: now})
	})
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	s.record(ctx, "user.created", "user", user.ID.String(), map[string]any{"role": string(user.Role)})
	return s.User(ctx, user.ID)
}

// UpdateUser changes profile fields, role or status. Deactivation and bans
// revoke every session (web and admin), end live chats and close realtime
// connections immediately. Bans and admin role changes need step-up.
func (s *AdminService) UpdateUser(ctx context.Context, adminID, id uuid.UUID, in UserUpdate) (*AdminUserView, error) {
	if id == adminID {
		return nil, apperror.Forbidden("Administrators cannot change their own account here.")
	}
	before, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	if before.Status == domain.UserDeleted {
		return nil, apperror.InvalidState("This account has been deleted.")
	}
	fields := map[string]any{}
	invalid := map[string]string{}
	if in.Status != nil {
		switch *in.Status {
		case domain.UserActive, domain.UserDeactivated, domain.UserBanned:
			if *in.Status != before.Status {
				fields["status"] = *in.Status
				fields["status_changed_at"] = s.clock.Now()
			}
		case domain.UserDeleted:
			invalid["status"] = "Delete accounts with the delete action."
		default:
			invalid["status"] = "Use active, deactivated or banned."
		}
	}
	if in.StatusReason != nil {
		reason := CleanLine(*in.StatusReason)
		if utf8.RuneCountInString(reason) > 300 {
			invalid["status_reason"] = "Use at most 300 characters."
		}
		fields["status_reason"] = reason
	}
	if in.Status != nil && *in.Status == domain.UserBanned {
		if r, _ := fields["status_reason"].(string); r == "" {
			invalid["status_reason"] = "Give a reason for the ban."
		}
	}
	if in.Status != nil && *in.Status == domain.UserActive && in.StatusReason == nil {
		fields["status_reason"] = ""
	}
	if in.Role != nil {
		if !in.Role.Valid() {
			invalid["role"] = "Unknown role."
		} else if *in.Role != before.Role {
			if *in.Role == domain.RolePsychic || before.Role == domain.RolePsychic {
				invalid["role"] = "Advisor accounts are managed from the advisors page."
			} else {
				fields["role"] = *in.Role
			}
		}
	}
	if in.DisplayName != nil {
		name := CleanLine(*in.DisplayName)
		if name == "" || utf8.RuneCountInString(name) > 80 {
			invalid["display_name"] = "Use between 1 and 80 characters."
		} else {
			fields["display_name"] = name
		}
	}
	if in.Locale != nil {
		switch *in.Locale {
		case "en", "fr", "es":
			fields["locale"] = *in.Locale
		default:
			invalid["locale"] = "Choose en, fr or es."
		}
	}
	if in.Email != nil {
		email, ok := NormalizeEmail(*in.Email)
		if !ok {
			invalid["email"] = "Enter a valid email address."
		} else if email != before.Email {
			fields["email"] = email
		}
	}
	if len(invalid) > 0 {
		return nil, apperror.Validation(invalid)
	}
	if len(fields) == 0 {
		return nil, apperror.Validation(map[string]string{"status": "Nothing to update."})
	}
	roleChange := fields["role"] != nil
	newStatus, _ := fields["status"].(domain.UserStatus)
	disabling := newStatus == domain.UserDeactivated || newStatus == domain.UserBanned
	if newStatus == domain.UserBanned || (roleChange && (before.Role == domain.RoleAdmin || *in.Role == domain.RoleAdmin)) {
		if err := s.adminAuth.RequireStepUp(ctx); err != nil {
			return nil, err
		}
	}
	now := s.clock.Now()
	if disabling || roleChange || fields["email"] != nil {
		fields["token_version"] = gorm.Expr("token_version + 1")
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Updates(ctx, id, fields); err != nil {
			if database.IsUniqueViolation(err, "users_email_key") {
				return apperror.Conflict(apperror.CodeEmailTaken, "An account with this email already exists.")
			}
			return err
		}
		if disabling {
			_, err := s.tokens.RevokeAllForUser(ctx, id, "account_"+string(newStatus), now)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	s.auth.InvalidateStatus(ctx, id)
	if disabling || roleChange || fields["email"] != nil {
		warnErr(ctx, "admin: realtime revoke", s.events.Revoke(ctx, id))
	}
	if disabling || roleChange {
		s.adminAuth.RevokeAllFor(ctx, id, "account_changed")
	}
	if roleChange && before.Role == domain.RoleAdmin {
		s.adminAuth.ResetMFA(ctx, id)
	}
	if disabling {
		s.chat.EndAllForUser(ctx, id)
	}
	if before.Role == domain.RolePsychic && fields["status"] != nil {
		s.psySvc.PublishStatus(ctx, id)
	}
	meta := map[string]any{}
	for k, v := range fields {
		if k != "token_version" && k != "status_changed_at" {
			meta[k] = fmt.Sprint(v)
		}
	}
	action := "user.updated"
	switch newStatus {
	case domain.UserBanned:
		action = "user.banned"
	case domain.UserDeactivated:
		action = "user.deactivated"
	case domain.UserActive:
		action = "user.reactivated"
	}
	s.record(ctx, action, "user", id.String(), meta)
	return s.User(ctx, id)
}

// DeleteUser soft-deletes and anonymises an account. Personal data
// (email, names, birth data, avatar) is erased; the append-only ledger,
// earnings and audit history are kept with the pseudonymous ID.
func (s *AdminService) DeleteUser(ctx context.Context, adminID, id uuid.UUID, reason string) error {
	if id == adminID {
		return apperror.Forbidden("Administrators cannot delete their own account here.")
	}
	reason = CleanLine(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return apperror.Validation(map[string]string{"reason": "A reason (up to 300 characters) is required."})
	}
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return mapRepoErr(err, "User")
	}
	if u.Status == domain.UserDeleted {
		return nil
	}
	profile, _ := s.users.GetProfile(ctx, id)
	now := s.clock.Now()
	unusable, err := s.hasher.Hash(security.MustRandomToken(32))
	if err != nil {
		return apperror.Internal(err)
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Updates(ctx, id, map[string]any{
			"email": "deleted+" + id.String() + "@deleted.invalid", "display_name": "Deleted user", "password_hash": unusable,
			"status": domain.UserDeleted, "status_reason": reason, "status_changed_at": now, "deleted_at": now,
			"token_version": gorm.Expr("token_version + 1"),
		}); err != nil {
			return err
		}
		if profile != nil {
			if err := s.users.UpdateProfile(ctx, id, map[string]any{
				"nickname": "", "birth_date": nil, "birth_time": "", "birth_place_id": "", "birth_place": "",
				"birth_latitude": nil, "birth_longitude": nil, "birth_timezone": "", "location": "", "avatar_url": "",
				"avatar_key": "", "country_code": "",
			}); err != nil {
				return err
			}
		}
		if u.Role == domain.RolePsychic {
			if err := s.psychics.Updates(ctx, id, map[string]any{"listed": false, "featured": false, "availability": domain.AvailabilityOffline}); err != nil && !errors.Is(err, repository.ErrNotFound) {
				return err
			}
		}
		_, err := s.tokens.RevokeAllForUser(ctx, id, "account_deleted", now)
		return err
	})
	if err != nil {
		return mapRepoErr(err, "User")
	}
	if profile != nil && profile.AvatarKey != "" && s.avatarStore != nil {
		warnErr(ctx, "admin: delete avatar object", s.avatarStore.Delete(ctx, profile.AvatarKey))
	}
	s.auth.InvalidateStatus(ctx, id)
	warnErr(ctx, "admin: realtime revoke", s.events.Revoke(ctx, id))
	s.adminAuth.RevokeAllFor(ctx, id, "account_deleted")
	s.adminAuth.ResetMFA(ctx, id)
	s.chat.EndAllForUser(ctx, id)
	if u.Role == domain.RolePsychic {
		s.psySvc.PublishStatus(ctx, id)
	}
	s.record(ctx, "user.deleted", "user", id.String(), map[string]any{"reason": reason, "role": string(u.Role)})
	return nil
}

// LogoutEverywhere revokes every main-site session of a user.
func (s *AdminService) LogoutEverywhere(ctx context.Context, id uuid.UUID) error {
	if _, err := s.users.GetByID(ctx, id); err != nil {
		return mapRepoErr(err, "User")
	}
	if err := s.auth.LogoutAll(ctx, id); err != nil {
		return err
	}
	s.record(ctx, "user.logged_out_all", "user", id.String(), nil)
	return nil
}

// AdjustCredits applies a manual credit correction with a reason.
func (s *AdminService) AdjustCredits(ctx context.Context, adminID, userID uuid.UUID, amount int64, reason, key string) (*WalletView, error) {
	reason = CleanLine(reason)
	if amount == 0 || amount > s.maxAdj || amount < -s.maxAdj {
		return nil, apperror.Validation(map[string]string{"amount": fmt.Sprintf("Use a non-zero amount within ±%d.", s.maxAdj)})
	}
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return nil, apperror.Validation(map[string]string{"reason": "A reason (up to 300 characters) is required."})
	}
	if !ValidIdempotencyKey(key) {
		return nil, apperror.Validation(map[string]string{"idempotency_key": "A valid idempotency key is required."})
	}
	var w *domain.Wallet
	var applied bool
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		w, applied, err = s.wallet.Apply(ctx, Movement{UserID: userID, Type: domain.LedgerAdminAdjustment, Amount: amount,
			ReferenceType: "admin", IdempotencyKey: "admin_adjust:" + key, Description: reason, ActorID: &adminID})
		return err
	})
	if errors.Is(err, ErrInsufficientFunds) {
		return nil, apperror.Validation(map[string]string{"amount": "The adjustment would make the balance negative."})
	}
	if err != nil {
		return nil, mapRepoErr(err, "Wallet")
	}
	wv := walletView(w)
	if applied {
		s.record(ctx, "wallet.adjusted", "user", userID.String(), map[string]any{"amount": amount, "reason": reason})
		warnErr(ctx, "admin: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, wv, userID))
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifyCreditsAdjusted, Data: map[string]any{"amount": amount}, Link: "/profile"}, userID)
	}
	return &wv, nil
}

// PresenceView summarises who is connected right now.
type PresenceView struct {
	OnlineClients  int             `json:"online_clients"`
	OnlinePsychics int             `json:"online_psychics"`
	Users          []OnlineUserRow `json:"users"`
}

// OnlineUserRow is a connected user.
type OnlineUserRow struct {
	ID          uuid.UUID `json:"id"`
	Role        string    `json:"role"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// Presence lists connected users (first 500) for broadcast targeting.
func (s *AdminService) Presence(ctx context.Context, role string) (*PresenceView, error) {
	ids, err := s.presence.OnlineUsers(ctx, 5000)
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	users, err := s.users.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	v := &PresenceView{Users: []OnlineUserRow{}}
	for _, u := range users {
		if u.Status != domain.UserActive {
			continue
		}
		switch u.Role {
		case domain.RoleClient:
			v.OnlineClients++
		case domain.RolePsychic:
			v.OnlinePsychics++
		}
		if (role == "" || string(u.Role) == role) && len(v.Users) < 500 {
			v.Users = append(v.Users, OnlineUserRow{ID: u.ID, Role: string(u.Role), DisplayName: u.DisplayName, Email: u.Email})
		}
	}
	return v, nil
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validatePsychicInput(in PsychicInput, creating bool) (map[string]any, map[string]string) {
	upd := map[string]any{}
	fields := map[string]string{}
	str := func(p *string, col string, max int, required bool) {
		if p == nil {
			if required {
				fields[col] = "This field is required."
			}
			return
		}
		v := CleanLine(*p)
		if (required && v == "") || utf8.RuneCountInString(v) > max {
			fields[col] = fmt.Sprintf("Use between 1 and %d characters.", max)
			return
		}
		upd[col] = v
	}
	str(in.Name, "name", 80, creating)
	str(in.Headline, "headline", 200, false)
	str(in.Zodiac, "zodiac", 20, false)
	if in.Slug != nil {
		if !slugPattern.MatchString(*in.Slug) || len(*in.Slug) > 64 {
			fields["slug"] = "Use lowercase letters, digits and hyphens."
		} else {
			upd["slug"] = *in.Slug
		}
	} else if creating {
		fields["slug"] = "This field is required."
	}
	if in.ImageURL != nil {
		v := strings.TrimSpace(*in.ImageURL)
		if v != "" && !strings.HasPrefix(v, "https://") {
			fields["image_url"] = "Use an https:// URL."
		} else {
			upd["image_url"] = v
		}
	}
	if in.Philosophy != nil {
		upd["philosophy"] = truncateString(CleanText(*in.Philosophy), 600)
	}
	if in.About != nil {
		paras := domain.StringList{}
		for _, p := range *in.About {
			if v := CleanText(p); v != "" {
				paras = append(paras, truncateString(v, 2000))
			}
		}
		upd["about"] = paras
	}
	if in.Skills != nil {
		skills := domain.StringList{}
		for _, sk := range *in.Skills {
			if v := CleanLine(sk); v != "" {
				skills = append(skills, truncateString(v, 40))
			}
		}
		upd["skills"] = skills
	}
	if in.RatePerMinute != nil {
		if *in.RatePerMinute < 1 || *in.RatePerMinute > 10000 {
			fields["rate_per_minute"] = "Use a rate between 1 and 10000 credits."
		} else {
			upd["rate_per_minute"] = *in.RatePerMinute
		}
	} else if creating {
		fields["rate_per_minute"] = "This field is required."
	}
	if in.YearsExperience != nil {
		if *in.YearsExperience < 0 || *in.YearsExperience > 80 {
			fields["years_experience"] = "Use a value between 0 and 80."
		} else {
			upd["years_experience"] = *in.YearsExperience
		}
	}
	if in.Rating != nil {
		if *in.Rating < 0 || *in.Rating > 5 {
			fields["rating"] = "Use a value between 0 and 5."
		} else {
			upd["rating"] = *in.Rating
		}
	}
	if in.ReviewsCount != nil && *in.ReviewsCount >= 0 {
		upd["reviews_count"] = *in.ReviewsCount
	}
	if in.Listed != nil {
		upd["listed"] = *in.Listed
	}
	if in.SortOrder != nil {
		upd["sort_order"] = *in.SortOrder
	}
	if in.Featured != nil {
		upd["featured"] = *in.Featured
	}
	if in.FeaturedRank != nil {
		if *in.FeaturedRank < 0 || *in.FeaturedRank > 1000 {
			fields["featured_rank"] = "Use a rank between 0 and 1000."
		} else {
			upd["featured_rank"] = *in.FeaturedRank
		}
	}
	return upd, fields
}

// CreatePsychic creates an advisor account and profile.
func (s *AdminService) CreatePsychic(ctx context.Context, in PsychicInput) (*AdminPsychicView, error) {
	upd, fields := validatePsychicInput(in, true)
	email := ""
	if in.Email == nil {
		fields["email"] = "This field is required."
	} else if e, ok := NormalizeEmail(*in.Email); !ok {
		fields["email"] = "Enter a valid email address."
	} else {
		email = e
	}
	if in.Password == nil {
		fields["password"] = "This field is required."
	} else if msg := security.ValidatePassword(*in.Password, email); msg != "" {
		fields["password"] = msg
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	hash, err := s.hasher.Hash(*in.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	user := &domain.User{ID: domain.NewID(), Email: email, PasswordHash: hash, Role: domain.RolePsychic, Status: domain.UserActive,
		DisplayName: upd["name"].(string), Locale: "en", TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
	p := &domain.Psychic{UserID: user.ID, Name: upd["name"].(string), Slug: upd["slug"].(string), RatePerMinute: upd["rate_per_minute"].(int64),
		Availability: domain.AvailabilityOffline, Listed: true, Rating: 5, About: domain.StringList{}, Skills: domain.StringList{}, CreatedAt: now, UpdatedAt: now}
	if r, ok := upd["rating"].(float64); ok {
		p.Rating = r
	}
	if n, ok := upd["reviews_count"].(int); ok {
		p.ReviewsCount = n
	}
	p.RatingSum = p.Rating * float64(p.ReviewsCount)
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Create(ctx, user); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				return apperror.Conflict(apperror.CodeEmailTaken, "An account with this email already exists.")
			}
			return err
		}
		if err := s.users.CreateProfile(ctx, newProfile(user.ID, nil, now)); err != nil {
			return err
		}
		if err := s.wallets.Create(ctx, &domain.Wallet{UserID: user.ID, UpdatedAt: now}); err != nil {
			return err
		}
		if err := s.psychics.Create(ctx, p); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				return apperror.Conflict(apperror.CodeConflict, "This slug is already used by another advisor.")
			}
			return err
		}
		delete(upd, "name")
		delete(upd, "slug")
		delete(upd, "rate_per_minute")
		if len(upd) > 0 {
			return s.psychics.Updates(ctx, user.ID, upd)
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	s.record(ctx, "psychic.created", "psychic", user.ID.String(), map[string]any{"slug": p.Slug, "rate": p.RatePerMinute})
	return s.psychic(ctx, user.ID)
}

// AdminPsychicView is an advisor as seen by administrators.
type AdminPsychicView struct {
	PsychicView
	FeaturedRank int        `json:"featured_rank"`
	SortOrder    int        `json:"sort_order"`
	Availability string     `json:"availability"`
	Email        string     `json:"email"`
	UserStatus   string     `json:"user_status"`
	StatusReason string     `json:"status_reason,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ImageKey     string     `json:"-"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

func adminPsychicView(p *domain.Psychic, st domain.PresenceStatus, u *domain.User) AdminPsychicView {
	v := AdminPsychicView{PsychicView: psychicView(p, st), FeaturedRank: p.FeaturedRank, SortOrder: p.SortOrder,
		Availability: string(p.Availability), CreatedAt: p.CreatedAt, ImageKey: p.ImageKey}
	if u != nil {
		v.Email, v.UserStatus, v.StatusReason, v.DeletedAt = u.Email, string(u.Status), u.StatusReason, u.DeletedAt
	}
	return v
}

func (s *AdminService) psychic(ctx context.Context, id uuid.UUID) (*AdminPsychicView, error) {
	st, p, err := s.psySvc.Status(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Services, _ = s.psychics.ListServices(ctx, id, false)
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	v := adminPsychicView(p, st, u)
	return &v, nil
}

// Psychic returns one advisor with every service.
func (s *AdminService) Psychic(ctx context.Context, id uuid.UUID) (*AdminPsychicView, error) {
	return s.psychic(ctx, id)
}

// PsychicFilter narrows the admin advisor list.
type PsychicFilter struct {
	Query    string
	Status   string
	Listed   *bool
	Featured *bool
}

// Psychics lists all advisors including unlisted and banned ones.
func (s *AdminService) Psychics(ctx context.Context, f PsychicFilter) ([]AdminPsychicView, error) {
	list, err := s.psychics.List(ctx, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	st, err := s.psySvc.statuses(ctx, list)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(list))
	for i := range list {
		ids[i] = list[i].UserID
	}
	users, err := s.users.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byID := make(map[uuid.UUID]*domain.User, len(users))
	for i := range users {
		byID[users[i].ID] = &users[i]
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	out := make([]AdminPsychicView, 0, len(list))
	for i := range list {
		p := &list[i]
		u := byID[p.UserID]
		if q != "" && !strings.Contains(strings.ToLower(p.Name), q) && !strings.Contains(p.Slug, q) && (u == nil || !strings.Contains(u.Email, q)) {
			continue
		}
		if f.Status != "" && (u == nil || string(u.Status) != f.Status) {
			continue
		}
		if f.Listed != nil && p.Listed != *f.Listed {
			continue
		}
		if f.Featured != nil && p.Featured != *f.Featured {
			continue
		}
		out = append(out, adminPsychicView(p, st[p.UserID], u))
	}
	return out, nil
}

// UpdatePsychic edits an advisor, including pricing (BR-09), visibility
// (hide = listed false) and featured placement.
func (s *AdminService) UpdatePsychic(ctx context.Context, id uuid.UUID, in PsychicInput) (*AdminPsychicView, error) {
	upd, fields := validatePsychicInput(in, false)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	if r, ok := upd["rating"].(float64); ok {
		// Editing the displayed rating rebases the aggregate used by reviews.
		p, err := s.psychics.GetByUserID(ctx, id)
		if err != nil {
			return nil, mapRepoErr(err, "Advisor")
		}
		count := p.ReviewsCount
		if n, ok := upd["reviews_count"].(int); ok {
			count = n
		}
		upd["rating_sum"] = r * float64(count)
	} else if n, ok := upd["reviews_count"].(int); ok {
		p, err := s.psychics.GetByUserID(ctx, id)
		if err != nil {
			return nil, mapRepoErr(err, "Advisor")
		}
		upd["rating_sum"] = p.Rating * float64(n)
	}
	if err := s.psychics.Updates(ctx, id, upd); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.Conflict(apperror.CodeConflict, "This slug is already used by another advisor.")
		}
		return nil, mapRepoErr(err, "Advisor")
	}
	meta := map[string]any{}
	for k, v := range upd {
		switch k {
		case "rate_per_minute", "listed", "slug", "name", "featured", "featured_rank", "rating":
			meta[k] = v
		}
	}
	// Single-purpose toggles get their own audit action for readability.
	action := "psychic.updated"
	if len(meta) == 1 {
		if v, ok := meta["listed"].(bool); ok {
			action = map[bool]string{true: "psychic.unhidden", false: "psychic.hidden"}[v]
		}
		if v, ok := meta["featured"].(bool); ok {
			action = map[bool]string{true: "psychic.featured", false: "psychic.unfeatured"}[v]
		}
	}
	s.record(ctx, action, "psychic", id.String(), meta)
	s.psySvc.PublishStatus(ctx, id)
	return s.psychic(ctx, id)
}

func validateServiceInput(in ServiceInput, creating bool) (map[string]any, map[string]string) {
	upd, fields := map[string]any{}, map[string]string{}
	if in.Name != nil {
		v := CleanLine(*in.Name)
		if v == "" || utf8.RuneCountInString(v) > 80 {
			fields["name"] = "Use between 1 and 80 characters."
		} else {
			upd["name"] = v
		}
	} else if creating {
		fields["name"] = "This field is required."
	}
	if in.Subtitle != nil {
		upd["subtitle"] = truncateString(CleanLine(*in.Subtitle), 80)
	}
	if in.Description != nil {
		upd["description"] = truncateString(CleanText(*in.Description), 1000)
	}
	if in.PriceCredits != nil {
		if *in.PriceCredits < 1 || *in.PriceCredits > 1000000 {
			fields["price_credits"] = "Use a price between 1 and 1,000,000 credits."
		} else {
			upd["price_credits"] = *in.PriceCredits
		}
	} else if creating {
		fields["price_credits"] = "This field is required."
	}
	if in.Active != nil {
		upd["active"] = *in.Active
	}
	if in.SortOrder != nil {
		upd["sort_order"] = *in.SortOrder
	}
	return upd, fields
}

// CreateService adds a listed custom request to an advisor.
func (s *AdminService) CreateService(ctx context.Context, psychicID uuid.UUID, in ServiceInput) (*ServiceView, error) {
	upd, fields := validateServiceInput(in, true)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if _, err := s.psychics.GetByUserID(ctx, psychicID); err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	now := s.clock.Now()
	svc := &domain.PsychicService{ID: domain.NewID(), PsychicID: psychicID, Name: upd["name"].(string), PriceCredits: upd["price_credits"].(int64), Active: true, CreatedAt: now, UpdatedAt: now}
	if v, ok := upd["subtitle"].(string); ok {
		svc.Subtitle = v
	}
	if v, ok := upd["description"].(string); ok {
		svc.Description = v
	}
	if v, ok := upd["sort_order"].(int); ok {
		svc.SortOrder = v
	}
	if v, ok := upd["active"].(bool); ok {
		svc.Active = v
	}
	if err := s.psychics.CreateService(ctx, svc); err != nil {
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "service.created", "service", svc.ID.String(), map[string]any{"psychic_id": psychicID.String(), "price": svc.PriceCredits})
	v := servicesView([]domain.PsychicService{*svc})[0]
	return &v, nil
}

// UpdateService edits a service (pricing is admin-only, BR-09).
func (s *AdminService) UpdateService(ctx context.Context, id uuid.UUID, in ServiceInput) (*ServiceView, error) {
	upd, fields := validateServiceInput(in, false)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	if err := s.psychics.UpdateService(ctx, id, upd); err != nil {
		return nil, mapRepoErr(err, "Service")
	}
	svc, err := s.psychics.GetService(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Service")
	}
	s.record(ctx, "service.updated", "service", id.String(), map[string]any{"price": svc.PriceCredits, "active": svc.Active})
	v := servicesView([]domain.PsychicService{*svc})[0]
	return &v, nil
}

// DeleteService removes a service that no order references.
func (s *AdminService) DeleteService(ctx context.Context, id uuid.UUID) error {
	svc, err := s.psychics.GetService(ctx, id)
	if err != nil {
		return mapRepoErr(err, "Service")
	}
	if err := s.psychics.DeleteService(ctx, id); err != nil {
		if errors.Is(err, repository.ErrInUse) {
			return apperror.Conflict(apperror.CodeConflict, "Orders reference this service. Deactivate it instead.")
		}
		return mapRepoErr(err, "Service")
	}
	s.record(ctx, "service.deleted", "service", id.String(), map[string]any{"psychic_id": svc.PsychicID.String(), "name": svc.Name})
	return nil
}

// DeletePsychic soft-deletes an advisor account: it disappears from the
// directory, live chats end and sign-in stops; history is kept.
func (s *AdminService) DeletePsychic(ctx context.Context, adminID, id uuid.UUID, reason string) error {
	if _, err := s.psychics.GetByUserID(ctx, id); err != nil {
		return mapRepoErr(err, "Advisor")
	}
	return s.DeleteUser(ctx, adminID, id, reason)
}

// UploadPsychicImage stores a new advisor portrait in the advisor image
// bucket (R2_ADVISOR_IMAGE) and replaces the previous one.
func (s *AdminService) UploadPsychicImage(ctx context.Context, id uuid.UUID, data []byte) (*AdminPsychicView, error) {
	if _, off := s.advisorStore.(storage.Disabled); off || s.advisorStore == nil {
		return nil, apperror.FeatureUnavailable("Advisor image uploads are not available yet.")
	}
	p, err := s.psychics.GetByUserID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	img, err := ProcessImage(data, s.advisorImage)
	if err != nil {
		return nil, err
	}
	key := objectKey("advisors/"+id.String(), img.Extension())
	url, err := s.advisorStore.Put(ctx, key, img.ContentType, img.Data, "public, max-age=31536000, immutable")
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if err := s.psychics.Updates(ctx, id, map[string]any{"image_url": url, "image_key": key}); err != nil {
		_ = s.advisorStore.Delete(ctx, key)
		return nil, mapRepoErr(err, "Advisor")
	}
	if p.ImageKey != "" {
		warnErr(ctx, "admin: delete previous advisor image", s.advisorStore.Delete(ctx, p.ImageKey))
	}
	s.record(ctx, "psychic.image_uploaded", "psychic", id.String(), map[string]any{"bytes": len(img.Data), "width": img.Width, "height": img.Height})
	s.psySvc.PublishStatus(ctx, id)
	return s.psychic(ctx, id)
}

// DeletePsychicImage removes an advisor portrait.
func (s *AdminService) DeletePsychicImage(ctx context.Context, id uuid.UUID) (*AdminPsychicView, error) {
	p, err := s.psychics.GetByUserID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	if err := s.psychics.Updates(ctx, id, map[string]any{"image_url": "", "image_key": ""}); err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	if p.ImageKey != "" && s.advisorStore != nil {
		warnErr(ctx, "admin: delete advisor image", s.advisorStore.Delete(ctx, p.ImageKey))
	}
	s.record(ctx, "psychic.image_removed", "psychic", id.String(), nil)
	return s.psychic(ctx, id)
}

// DeletePack removes a credit pack that no purchase references.
func (s *AdminService) DeletePack(ctx context.Context, id uuid.UUID) error {
	if err := s.credits.DeletePack(ctx, id); err != nil {
		if errors.Is(err, repository.ErrInUse) {
			return apperror.Conflict(apperror.CodeConflict, "Purchases reference this pack. Deactivate it instead.")
		}
		return mapRepoErr(err, "Credit pack")
	}
	s.record(ctx, "pack.deleted", "credit_pack", id.String(), nil)
	return nil
}

// Packs lists all credit packs including inactive ones.
func (s *AdminService) Packs(ctx context.Context) ([]PackView, error) {
	packs, err := s.credits.ListPacks(ctx, false)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]PackView, 0, len(packs))
	for i := range packs {
		out = append(out, packView(&packs[i]))
	}
	return out, nil
}

func validatePack(in PackInput, creating bool) (map[string]any, map[string]string) {
	upd, fields := map[string]any{}, map[string]string{}
	if in.Name != nil {
		v := CleanLine(*in.Name)
		if v == "" || utf8.RuneCountInString(v) > 60 {
			fields["name"] = "Use between 1 and 60 characters."
		} else {
			upd["name"] = v
		}
	} else if creating {
		fields["name"] = "This field is required."
	}
	if in.Credits != nil {
		if *in.Credits < 1 || *in.Credits > 1000000 {
			fields["credits"] = "Use between 1 and 1,000,000 credits."
		} else {
			upd["credits"] = *in.Credits
		}
	} else if creating {
		fields["credits"] = "This field is required."
	}
	if in.PriceCents != nil {
		if *in.PriceCents < 50 || *in.PriceCents > 10000000 {
			fields["price_cents"] = "Use a price between 0.50 and 100,000.00."
		} else {
			upd["price_cents"] = *in.PriceCents
		}
	} else if creating {
		fields["price_cents"] = "This field is required."
	}
	if in.Currency != nil {
		c := strings.ToUpper(strings.TrimSpace(*in.Currency))
		if len(c) != 3 {
			fields["currency"] = "Use a 3-letter ISO currency code."
		} else {
			upd["currency"] = c
		}
	}
	if in.Active != nil {
		upd["active"] = *in.Active
	}
	if in.Featured != nil {
		upd["featured"] = *in.Featured
	}
	if in.SortOrder != nil {
		upd["sort_order"] = *in.SortOrder
	}
	return upd, fields
}

// CreatePack adds a credit pack.
func (s *AdminService) CreatePack(ctx context.Context, in PackInput, defaultCurrency string) (*PackView, error) {
	upd, fields := validatePack(in, true)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	now := s.clock.Now()
	p := &domain.CreditPack{ID: domain.NewID(), Name: upd["name"].(string), Credits: upd["credits"].(int64), PriceCents: upd["price_cents"].(int64),
		Currency: defaultCurrency, Active: true, CreatedAt: now, UpdatedAt: now}
	if v, ok := upd["currency"].(string); ok {
		p.Currency = v
	}
	if v, ok := upd["active"].(bool); ok {
		p.Active = v
	}
	if v, ok := upd["featured"].(bool); ok {
		p.Featured = v
	}
	if v, ok := upd["sort_order"].(int); ok {
		p.SortOrder = v
	}
	if err := s.credits.CreatePack(ctx, p); err != nil {
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "pack.created", "credit_pack", p.ID.String(), map[string]any{"credits": p.Credits, "price_cents": p.PriceCents, "currency": p.Currency})
	v := packView(p)
	return &v, nil
}

// UpdatePack edits a credit pack (pricing changes are audited).
func (s *AdminService) UpdatePack(ctx context.Context, id uuid.UUID, in PackInput) (*PackView, error) {
	upd, fields := validatePack(in, false)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	before, err := s.credits.GetPack(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Credit pack")
	}
	if err := s.credits.UpdatePack(ctx, id, upd); err != nil {
		return nil, mapRepoErr(err, "Credit pack")
	}
	after, err := s.credits.GetPack(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Credit pack")
	}
	s.record(ctx, "pack.updated", "credit_pack", id.String(), map[string]any{
		"credits_before": before.Credits, "credits_after": after.Credits,
		"price_before": before.PriceCents, "price_after": after.PriceCents, "active": after.Active,
	})
	v := packView(after)
	return &v, nil
}

// Sessions lists chat sessions platform-wide (monitoring).
func (s *AdminService) Sessions(ctx context.Context, statuses []domain.SessionStatus, clientID, psychicID *uuid.UUID, cursor string, limit int) (Page[SessionView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[SessionView]{}, err
	}
	p, _ := reqctx.PrincipalFrom(ctx)
	return s.chat.sessionsPage(ctx, repository.SessionFilter{Statuses: statuses, ClientID: clientID, PsychicID: psychicID, Cursor: c, Limit: limit},
		viewer{ID: p.UserID, Admin: true})
}

// Transcript returns a conversation's messages for oversight; every access
// is audited because transcripts are personal data.
func (s *AdminService) Transcript(ctx context.Context, sessionID uuid.UUID, reason, cursor string, limit int) (Page[MessageView], error) {
	reason = CleanLine(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return Page[MessageView]{}, apperror.Validation(map[string]string{"reason": "State why you need to view this transcript."})
	}
	sess, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return Page[MessageView]{}, mapRepoErr(err, "Session")
	}
	p, _ := reqctx.PrincipalFrom(ctx)
	s.record(ctx, "chat.transcript_viewed", "chat_session", sessionID.String(), map[string]any{"reason": reason})
	return s.chat.messagesPage(ctx, p.UserID, sess.ConversationID, cursor, limit)
}

// TerminateSession ends a live session on behalf of the platform.
func (s *AdminService) TerminateSession(ctx context.Context, sessionID uuid.UUID, reason string) (*SessionSummary, error) {
	p, _ := reqctx.PrincipalFrom(ctx)
	sum, err := s.chat.EndSession(ctx, p, sessionID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "chat.terminated", "chat_session", sessionID.String(), map[string]any{"reason": CleanLine(reason)})
	return sum, nil
}

// AuditFilter narrows audit listings.
type AuditFilter struct {
	ActorID    *uuid.UUID
	TargetType string
	TargetID   string
	Action     string
}

// Audit lists audit entries with the acting administrator's email.
func (s *AdminService) Audit(ctx context.Context, f AuditFilter, cursor string, limit int) (Page[AuditView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AuditView]{}, err
	}
	rows, err := s.audit.List(ctx, repository.AuditFilter{ActorID: f.ActorID, TargetType: truncateString(f.TargetType, 64),
		TargetID: truncateString(f.TargetID, 64), Action: truncateString(f.Action, 64), Cursor: c, Limit: limit})
	if err != nil {
		return Page[AuditView]{}, apperror.Internal(err)
	}
	actorIDs := []uuid.UUID{}
	for _, a := range rows {
		if a.ActorID != nil {
			actorIDs = append(actorIDs, *a.ActorID)
		}
	}
	emails := map[uuid.UUID]string{}
	if actors, err := s.users.GetByIDs(ctx, actorIDs); err == nil {
		for _, u := range actors {
			emails[u.ID] = u.Email
		}
	}
	page := Page[AuditView]{Items: make([]AuditView, 0, len(rows))}
	for _, a := range rows {
		v := AuditView{ID: a.ID, ActorID: a.ActorID, Action: a.Action, TargetType: a.TargetType,
			TargetID: a.TargetID, Metadata: a.Metadata, RequestID: a.RequestID, CreatedAt: a.CreatedAt}
		if a.ActorID != nil {
			v.ActorEmail = emails[*a.ActorID]
		}
		page.Items = append(page.Items, v)
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 50, 200) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}
