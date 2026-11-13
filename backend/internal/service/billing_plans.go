package service

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Plans lists the memberships on sale: the current version of every active
// plan, localised.
func (s *BillingService) Plans(ctx context.Context, locale string) ([]PlanView, error) {
	if !s.cfg.MembershipsEnabled { // revisions 5.1: nothing is on sale while memberships are off (§25.1)
		return []PlanView{}, nil
	}
	plans, err := s.repo.ListPlans(ctx, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, 0, len(plans))
	for _, p := range plans {
		ids = append(ids, p.ID)
	}
	versions, err := s.repo.ListVersions(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	current := map[uuid.UUID]domain.PlanVersion{}
	for _, v := range versions {
		if v.IsCurrent {
			current[v.PlanID] = v
		}
	}
	out := make([]PlanView, 0, len(plans))
	for i := range plans {
		if v, ok := current[plans[i].ID]; ok {
			out = append(out, planView(&plans[i], &v, locale))
		}
	}
	return out, nil
}

// PlanVersionInput is an immutable price and benefit set.
type PlanVersionInput struct {
	PriceCents         int64               `json:"price_cents"`
	Currency           string              `json:"currency"`
	Interval           string              `json:"interval"`
	IntervalCount      int                 `json:"interval_count"`
	MonthlyCreditGrant int64               `json:"monthly_credit_grant"`
	ChatDiscountBps    int                 `json:"chat_discount_bps"`
	MonthlyFreeMinutes int                 `json:"monthly_free_minutes"`
	WaitlistPriority   bool                `json:"waitlist_priority"`
	Benefits           map[string][]string `json:"benefits"`
	MakeCurrent        *bool               `json:"make_current"`
}

// PlanCreateInput creates a plan with its first version.
type PlanCreateInput struct {
	Code        string            `json:"code"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
	Active      *bool             `json:"active"`
	SortOrder   *int              `json:"sort_order"`
	Version     PlanVersionInput  `json:"version"`
}

// PlanUpdateInput edits a plan's presentation (prices are versions).
type PlanUpdateInput struct {
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
	Active      *bool             `json:"active"`
	SortOrder   *int              `json:"sort_order"`
}

// PlanVersionView is one version of a plan (admin).
type PlanVersionView struct {
	ID                 uuid.UUID           `json:"id"`
	Version            int                 `json:"version"`
	PriceCents         int64               `json:"price_cents"`
	Currency           string              `json:"currency"`
	Interval           string              `json:"interval"`
	IntervalCount      int                 `json:"interval_count"`
	MonthlyCreditGrant int64               `json:"monthly_credit_grant"`
	ChatDiscountBps    int                 `json:"chat_discount_bps"`
	MonthlyFreeMinutes int                 `json:"monthly_free_minutes"`
	WaitlistPriority   bool                `json:"waitlist_priority"`
	Benefits           map[string][]string `json:"benefits"`
	IsCurrent          bool                `json:"is_current"`
	CreatedAt          time.Time           `json:"created_at"`
}

// AdminPlanView is a plan with every version (admin).
type AdminPlanView struct {
	ID                uuid.UUID         `json:"id"`
	Code              string            `json:"code"`
	Name              map[string]string `json:"name"`
	Description       map[string]string `json:"description"`
	Active            bool              `json:"active"`
	SortOrder         int               `json:"sort_order"`
	OpenSubscriptions int64             `json:"open_subscriptions"`
	Versions          []PlanVersionView `json:"versions"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

func versionView(v *domain.PlanVersion) PlanVersionView {
	benefits := map[string][]string(v.Benefits)
	if benefits == nil {
		benefits = map[string][]string{}
	}
	return PlanVersionView{ID: v.ID, Version: v.Version, PriceCents: v.PriceCents, Currency: v.Currency, Interval: string(v.BillingInterval),
		IntervalCount: v.IntervalCount, MonthlyCreditGrant: v.MonthlyCreditGrant, ChatDiscountBps: v.ChatDiscountBps,
		MonthlyFreeMinutes: v.MonthlyFreeMinutes, WaitlistPriority: v.WaitlistPriority, Benefits: benefits, IsCurrent: v.IsCurrent,
		CreatedAt: v.CreatedAt}
}

func (s *BillingService) adminPlanView(ctx context.Context, p *domain.SubscriptionPlan, versions []domain.PlanVersion) AdminPlanView {
	v := AdminPlanView{ID: p.ID, Code: p.Code, Name: map[string]string(p.Name), Description: map[string]string(p.Description),
		Active: p.Active, SortOrder: p.SortOrder, Versions: []PlanVersionView{}, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	for i := range versions {
		if versions[i].PlanID == p.ID {
			v.Versions = append(v.Versions, versionView(&versions[i]))
		}
	}
	if n, err := s.repo.CountOpenSubscriptionsForPlan(ctx, p.ID); err == nil {
		v.OpenSubscriptions = n
	}
	return v
}

var planCode = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

var planLocales = map[string]bool{"en": true, "es": true, "fr": true}

// localized validates a {locale: text} object.
func localized(field string, in map[string]string, maxRunes int, multiline, requireEnglish bool, fields map[string]string) domain.LocalizedText {
	out := domain.LocalizedText{}
	for loc, raw := range in {
		if !planLocales[loc] {
			fields[field+"."+loc] = "Use en, es or fr."
			continue
		}
		v := CleanLine(raw)
		if multiline {
			v = CleanText(raw)
		}
		if utf8.RuneCountInString(v) > maxRunes {
			fields[field+"."+loc] = "This text is too long."
			continue
		}
		if v != "" {
			out[loc] = v
		}
	}
	if requireEnglish && out["en"] == "" {
		fields[field+".en"] = "An English text is required."
	}
	return out
}

func (s *BillingService) validateVersion(in PlanVersionInput, fields map[string]string) *domain.PlanVersion {
	v := &domain.PlanVersion{PriceCents: in.PriceCents, Currency: strings.ToUpper(strings.TrimSpace(in.Currency)),
		BillingInterval: domain.PlanInterval(in.Interval), IntervalCount: in.IntervalCount, MonthlyCreditGrant: in.MonthlyCreditGrant,
		ChatDiscountBps: in.ChatDiscountBps, MonthlyFreeMinutes: in.MonthlyFreeMinutes, WaitlistPriority: in.WaitlistPriority,
		Benefits: domain.LocalizedList{}}
	if v.Currency == "" {
		v.Currency = s.cfg.Currency
	}
	if in.PriceCents < 50 || in.PriceCents > 10_000_000 {
		fields["version.price_cents"] = "Use a price between 0.50 and 100,000.00."
	}
	if !currencyCode.MatchString(v.Currency) {
		fields["version.currency"] = "Use a three-letter currency code."
	}
	switch v.BillingInterval {
	case domain.IntervalMonth:
		if v.IntervalCount == 0 {
			v.IntervalCount = 1
		}
		if v.IntervalCount < 1 || v.IntervalCount > 12 {
			fields["version.interval_count"] = "Use between 1 and 12 months."
		}
	case domain.IntervalYear:
		if v.IntervalCount == 0 {
			v.IntervalCount = 1
		}
		if v.IntervalCount < 1 || v.IntervalCount > 3 {
			fields["version.interval_count"] = "Use between 1 and 3 years."
		}
	default:
		fields["version.interval"] = "Choose month or year."
	}
	if in.MonthlyCreditGrant < 0 || in.MonthlyCreditGrant > 1_000_000 {
		fields["version.monthly_credit_grant"] = "Use between 0 and 1,000,000 credits."
	}
	if in.ChatDiscountBps < 0 || in.ChatDiscountBps > 5000 {
		fields["version.chat_discount_bps"] = "Use a discount between 0 and 50%."
	}
	if in.MonthlyFreeMinutes < 0 || in.MonthlyFreeMinutes > 600 {
		fields["version.monthly_free_minutes"] = "Use between 0 and 600 minutes."
	}
	for loc, lines := range in.Benefits {
		if !planLocales[loc] {
			fields["version.benefits."+loc] = "Use en, es or fr."
			continue
		}
		if len(lines) > 8 {
			fields["version.benefits."+loc] = "List at most 8 benefits."
			continue
		}
		clean := make([]string, 0, len(lines))
		for _, l := range lines {
			l = CleanLine(l)
			if l == "" || utf8.RuneCountInString(l) > 120 {
				fields["version.benefits."+loc] = "Each benefit needs 1 to 120 characters."
				break
			}
			clean = append(clean, l)
		}
		v.Benefits[loc] = clean
	}
	return v
}

// AdminPlans lists every plan with its versions.
func (s *BillingService) AdminPlans(ctx context.Context) ([]AdminPlanView, error) {
	plans, err := s.repo.ListPlans(ctx, false)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, 0, len(plans))
	for _, p := range plans {
		ids = append(ids, p.ID)
	}
	versions, err := s.repo.ListVersions(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]AdminPlanView, 0, len(plans))
	for i := range plans {
		out = append(out, s.adminPlanView(ctx, &plans[i], versions))
	}
	return out, nil
}

func (s *BillingService) adminPlan(ctx context.Context, id uuid.UUID) (*AdminPlanView, error) {
	p, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Plan")
	}
	versions, err := s.repo.ListVersions(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	v := s.adminPlanView(ctx, p, versions)
	return &v, nil
}

// AdminCreatePlan creates a plan and its first (current) version.
func (s *BillingService) AdminCreatePlan(ctx context.Context, in PlanCreateInput) (*AdminPlanView, error) {
	fields := map[string]string{}
	code := strings.ToLower(strings.TrimSpace(in.Code))
	if !planCode.MatchString(code) {
		fields["code"] = "Use 2–40 lowercase letters, digits or hyphens."
	}
	name := localized("name", in.Name, 80, false, true, fields)
	desc := localized("description", in.Description, 500, true, false, fields)
	if in.SortOrder != nil && (*in.SortOrder < -1000 || *in.SortOrder > 1000) {
		fields["sort_order"] = "Use a number between -1000 and 1000."
	}
	v := s.validateVersion(in.Version, fields)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	now := s.clock.Now()
	p := &domain.SubscriptionPlan{ID: domain.NewID(), Code: code, Name: name, Description: desc, Active: true, CreatedAt: now, UpdatedAt: now}
	if in.Active != nil {
		p.Active = *in.Active
	}
	if in.SortOrder != nil {
		p.SortOrder = *in.SortOrder
	}
	v.ID, v.PlanID, v.Version, v.IsCurrent, v.CreatedAt, v.UpdatedAt = domain.NewID(), p.ID, 1, true, now, now
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repo.CreatePlan(ctx, p); err != nil {
			return err
		}
		return s.repo.CreateVersion(ctx, v)
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperror.Validation(map[string]string{"code": "Another plan already uses this code."})
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "billing.plan_created", "subscription_plan", p.ID.String(), map[string]any{"code": code,
		"price_cents": v.PriceCents, "currency": v.Currency, "interval": v.BillingInterval})
	return s.adminPlan(ctx, p.ID)
}

// AdminUpdatePlan edits a plan's names, visibility and order.
func (s *BillingService) AdminUpdatePlan(ctx context.Context, id uuid.UUID, in PlanUpdateInput) (*AdminPlanView, error) {
	fields := map[string]string{}
	upd := map[string]any{}
	if in.Name != nil {
		upd["name"] = localized("name", in.Name, 80, false, true, fields)
	}
	if in.Description != nil {
		upd["description"] = localized("description", in.Description, 500, true, false, fields)
	}
	if in.Active != nil {
		upd["active"] = *in.Active
	}
	if in.SortOrder != nil {
		if *in.SortOrder < -1000 || *in.SortOrder > 1000 {
			fields["sort_order"] = "Use a number between -1000 and 1000."
		}
		upd["sort_order"] = *in.SortOrder
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	upd["updated_at"] = s.clock.Now()
	if err := s.repo.UpdatePlan(ctx, id, upd); err != nil {
		return nil, mapRepoErr(err, "Plan")
	}
	changed := make([]string, 0, len(upd))
	for k := range upd {
		if k != "updated_at" {
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	s.record(ctx, "billing.plan_updated", "subscription_plan", id.String(), map[string]any{"fields": changed})
	return s.adminPlan(ctx, id)
}

// AdminDeletePlan retires a plan nobody is subscribed to.
func (s *BillingService) AdminDeletePlan(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetPlan(ctx, id); err != nil {
		return mapRepoErr(err, "Plan")
	}
	n, err := s.repo.CountOpenSubscriptionsForPlan(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if n > 0 {
		return apperror.InvalidState("Members still use this plan. Deactivate it instead.")
	}
	if err := s.repo.SoftDeletePlan(ctx, id, s.clock.Now()); err != nil {
		return mapRepoErr(err, "Plan")
	}
	s.record(ctx, "billing.plan_deleted", "subscription_plan", id.String(), nil)
	return nil
}

// AdminCreatePlanVersion adds a price/benefit version. Existing members
// keep their version; new subscriptions use the current one.
func (s *BillingService) AdminCreatePlanVersion(ctx context.Context, planID uuid.UUID, in PlanVersionInput) (*AdminPlanView, error) {
	fields := map[string]string{}
	v := s.validateVersion(in, fields)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	makeCurrent := in.MakeCurrent == nil || *in.MakeCurrent
	now := s.clock.Now()
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.repo.GetPlan(ctx, planID); err != nil {
			return err
		}
		n, err := s.repo.MaxVersion(ctx, planID)
		if err != nil {
			return err
		}
		v.ID, v.PlanID, v.Version, v.CreatedAt, v.UpdatedAt = domain.NewID(), planID, n+1, now, now
		if err := s.repo.CreateVersion(ctx, v); err != nil {
			return err
		}
		if makeCurrent {
			return s.repo.SetCurrentVersion(ctx, planID, v.ID, now)
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Plan")
	}
	s.record(ctx, "billing.plan_version_created", "subscription_plan", planID.String(), map[string]any{"version": v.Version,
		"price_cents": v.PriceCents, "currency": v.Currency, "interval": v.BillingInterval, "interval_count": v.IntervalCount,
		"monthly_credit_grant": v.MonthlyCreditGrant, "chat_discount_bps": v.ChatDiscountBps,
		"monthly_free_minutes": v.MonthlyFreeMinutes, "waitlist_priority": v.WaitlistPriority, "current": makeCurrent})
	return s.adminPlan(ctx, planID)
}
