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
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// AdminUserView is a user as seen by administrators.
type AdminUserView struct {
	ID          uuid.UUID   `json:"id"`
	Email       string      `json:"email"`
	Role        string      `json:"role"`
	Status      string      `json:"status"`
	DisplayName string      `json:"display_name"`
	Locale      string      `json:"locale"`
	CreatedAt   time.Time   `json:"created_at"`
	LastLoginAt *time.Time  `json:"last_login_at,omitempty"`
	Wallet      *WalletView `json:"wallet,omitempty"`
}

// AuditView is an audit log entry.
type AuditView struct {
	ID         uuid.UUID      `json:"id"`
	ActorID    *uuid.UUID     `json:"actor_id,omitempty"`
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
	Status *domain.UserStatus `json:"status"`
	Role   *domain.Role       `json:"role"`
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

// AdminService implements the administrative console API (FR-13, FR-14).
type AdminService struct {
	tx       database.TxManager
	users    repository.UserRepository
	wallets  repository.WalletRepository
	tokens   repository.TokenRepository
	psychics repository.PsychicRepository
	credits  repository.CreditRepository
	audit    repository.AuditRepository
	chatRepo repository.ChatRepository
	wallet   *WalletService
	auth     *AuthService
	chat     *ChatService
	psySvc   *PsychicService
	events   EventPublisher
	hasher   *security.PasswordHasher
	clock    clock.Clock
	ipSecret string
	maxAdj   int64
}

// NewAdminService wires the admin service.
func NewAdminService(tx database.TxManager, users repository.UserRepository, wallets repository.WalletRepository,
	tokens repository.TokenRepository, psychics repository.PsychicRepository, credits repository.CreditRepository,
	audit repository.AuditRepository, chatRepo repository.ChatRepository, wallet *WalletService, auth *AuthService,
	chat *ChatService, psySvc *PsychicService, events EventPublisher, hasher *security.PasswordHasher, clk clock.Clock,
	ipSecret string, maxAdjustment int64) *AdminService {
	return &AdminService{tx: tx, users: users, wallets: wallets, tokens: tokens, psychics: psychics, credits: credits,
		audit: audit, chatRepo: chatRepo, wallet: wallet, auth: auth, chat: chat, psySvc: psySvc, events: events,
		hasher: hasher, clock: clk, ipSecret: ipSecret, maxAdj: maxAdjustment}
}

// record writes an audit entry; failures are logged loudly but do not undo
// the action that already happened.
func (s *AdminService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	p, _ := reqctx.PrincipalFrom(ctx)
	m := reqctx.MetaFrom(ctx)
	var actor *uuid.UUID
	if p.UserID != uuid.Nil {
		id := p.UserID
		actor = &id
	}
	if err := s.audit.Create(ctx, &domain.AuditLog{
		ID: domain.NewID(), ActorID: actor, Action: action, TargetType: targetType, TargetID: targetID,
		Metadata: domain.JSONMap(meta), IPHash: security.Keyed(s.ipSecret, m.IP, 32), RequestID: m.RequestID, CreatedAt: s.clock.Now(),
	}); err != nil {
		logger.FromContext(ctx).Error("admin: audit write failed", zap.Error(err), zap.String("action", action))
	}
}

func adminUserView(u *domain.User, w *domain.Wallet) AdminUserView {
	v := AdminUserView{ID: u.ID, Email: u.Email, Role: string(u.Role), Status: string(u.Status), DisplayName: u.DisplayName,
		Locale: u.Locale, CreatedAt: u.CreatedAt, LastLoginAt: u.LastLoginAt}
	if w != nil {
		wv := walletView(w)
		v.Wallet = &wv
	}
	return v
}

// Users lists accounts with optional search and filters.
func (s *AdminService) Users(ctx context.Context, q, role, status, cursor string, limit int) (Page[AdminUserView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminUserView]{}, err
	}
	if role != "" && !domain.Role(role).Valid() {
		return Page[AdminUserView]{}, apperror.Validation(map[string]string{"role": "Unknown role."})
	}
	rows, err := s.users.List(ctx, repository.UserFilter{Query: truncateString(strings.TrimSpace(q), 100), Role: domain.Role(role),
		Status: domain.UserStatus(status), Cursor: c, Limit: limit})
	if err != nil {
		return Page[AdminUserView]{}, apperror.Internal(err)
	}
	page := Page[AdminUserView]{Items: make([]AdminUserView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, adminUserView(&rows[i], nil))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 50, 200) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// User returns one account with its wallet.
func (s *AdminService) User(ctx context.Context, id uuid.UUID) (*AdminUserView, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	w, _ := s.wallets.Get(ctx, id)
	v := adminUserView(u, w)
	return &v, nil
}

// UpdateUser changes status or role. Deactivation revokes all sessions,
// ends live chats and closes realtime connections immediately.
func (s *AdminService) UpdateUser(ctx context.Context, adminID, id uuid.UUID, in UserUpdate) (*AdminUserView, error) {
	if id == adminID {
		return nil, apperror.Forbidden("Administrators cannot change their own account here.")
	}
	fields := map[string]any{}
	if in.Status != nil {
		if *in.Status != domain.UserActive && *in.Status != domain.UserDeactivated {
			return nil, apperror.Validation(map[string]string{"status": "Use active or deactivated."})
		}
		fields["status"] = *in.Status
	}
	if in.Role != nil {
		if !in.Role.Valid() {
			return nil, apperror.Validation(map[string]string{"role": "Unknown role."})
		}
		fields["role"] = *in.Role
	}
	if len(fields) == 0 {
		return nil, apperror.Validation(map[string]string{"status": "Nothing to update."})
	}
	before, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	now := s.clock.Now()
	deactivating := in.Status != nil && *in.Status == domain.UserDeactivated && before.Status != domain.UserDeactivated
	if deactivating || in.Role != nil {
		fields["token_version"] = gorm.Expr("token_version + 1")
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Updates(ctx, id, fields); err != nil {
			return err
		}
		if deactivating {
			_, err := s.tokens.RevokeAllForUser(ctx, id, "account_deactivated", now)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	s.auth.InvalidateStatus(ctx, id)
	if deactivating || in.Role != nil {
		warnErr(ctx, "admin: realtime revoke", s.events.Revoke(ctx, id))
	}
	if deactivating {
		s.chat.EndAllForUser(ctx, id)
		if before.Role == domain.RolePsychic {
			s.psySvc.PublishStatus(ctx, id)
		}
	}
	meta := map[string]any{}
	for k, v := range fields {
		if k != "token_version" {
			meta[k] = fmt.Sprint(v)
		}
	}
	s.record(ctx, "user.updated", "user", id.String(), meta)
	return s.User(ctx, id)
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
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		w, _, err = s.wallet.Apply(ctx, Movement{UserID: userID, Type: domain.LedgerAdminAdjustment, Amount: amount,
			ReferenceType: "admin", IdempotencyKey: "admin_adjust:" + key, Description: reason, ActorID: &adminID})
		return err
	})
	if errors.Is(err, ErrInsufficientFunds) {
		return nil, apperror.Validation(map[string]string{"amount": "The adjustment would make the balance negative."})
	}
	if err != nil {
		return nil, mapRepoErr(err, "Wallet")
	}
	s.record(ctx, "wallet.adjusted", "user", userID.String(), map[string]any{"amount": amount, "reason": reason})
	wv := walletView(w)
	warnErr(ctx, "admin: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, wv, userID))
	return &wv, nil
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
	return upd, fields
}

// CreatePsychic creates an advisor account and profile.
func (s *AdminService) CreatePsychic(ctx context.Context, in PsychicInput) (*PsychicView, error) {
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

func (s *AdminService) psychic(ctx context.Context, id uuid.UUID) (*PsychicView, error) {
	st, p, err := s.psySvc.Status(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Services, _ = s.psychics.ListServices(ctx, id, false)
	v := psychicView(p, st)
	return &v, nil
}

// Psychics lists all advisors including unlisted ones.
func (s *AdminService) Psychics(ctx context.Context) ([]PsychicView, error) {
	list, err := s.psychics.List(ctx, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	st, err := s.psySvc.statuses(ctx, list)
	if err != nil {
		return nil, err
	}
	out := make([]PsychicView, 0, len(list))
	for i := range list {
		out = append(out, psychicView(&list[i], st[list[i].UserID]))
	}
	return out, nil
}

// UpdatePsychic edits an advisor, including pricing (BR-09).
func (s *AdminService) UpdatePsychic(ctx context.Context, id uuid.UUID, in PsychicInput) (*PsychicView, error) {
	upd, fields := validatePsychicInput(in, false)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	if err := s.psychics.Updates(ctx, id, upd); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.Conflict(apperror.CodeConflict, "This slug is already used by another advisor.")
		}
		return nil, mapRepoErr(err, "Advisor")
	}
	meta := map[string]any{}
	for k, v := range upd {
		if k == "rate_per_minute" || k == "listed" || k == "slug" || k == "name" {
			meta[k] = v
		}
	}
	s.record(ctx, "psychic.updated", "psychic", id.String(), meta)
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

// Audit lists audit entries.
func (s *AdminService) Audit(ctx context.Context, actorID *uuid.UUID, targetType, targetID, cursor string, limit int) (Page[AuditView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AuditView]{}, err
	}
	rows, err := s.audit.List(ctx, actorID, targetType, targetID, c, limit)
	if err != nil {
		return Page[AuditView]{}, apperror.Internal(err)
	}
	page := Page[AuditView]{Items: make([]AuditView, 0, len(rows))}
	for _, a := range rows {
		page.Items = append(page.Items, AuditView{ID: a.ID, ActorID: a.ActorID, Action: a.Action, TargetType: a.TargetType,
			TargetID: a.TargetID, Metadata: a.Metadata, RequestID: a.RequestID, CreatedAt: a.CreatedAt})
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 50, 200) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}
