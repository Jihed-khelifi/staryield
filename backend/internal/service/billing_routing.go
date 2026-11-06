package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// ProviderView describes a configured payment provider (admin).
type ProviderView struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Sandbox      bool     `json:"sandbox"`
}

// Providers lists the configured payment providers and what they support.
func (s *BillingService) Providers() []ProviderView {
	out := []ProviderView{}
	for _, name := range s.registry.Names() {
		p, _ := s.registry.Get(name)
		out = append(out, ProviderView{Name: name, Capabilities: payment.CapabilitiesOf(p).Names(), Sandbox: payment.IsSandbox(name)})
	}
	return out
}

// RoutingRuleView is a routing rule (admin).
type RoutingRuleView struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Priority        int       `json:"priority"`
	Enabled         bool      `json:"enabled"`
	Purpose         string    `json:"purpose"`
	Providers       []string  `json:"providers"`
	MatchCountries  []string  `json:"match_countries"`
	MatchCurrencies []string  `json:"match_currencies"`
	MatchPlanIDs    []string  `json:"match_plan_ids"`
	MatchUserIDs    []string  `json:"match_user_ids"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func ruleView(r *domain.RoutingRule) RoutingRuleView {
	list := func(l domain.StringList) []string {
		if l == nil {
			return []string{}
		}
		return []string(l)
	}
	return RoutingRuleView{ID: r.ID, Name: r.Name, Priority: r.Priority, Enabled: r.Enabled, Purpose: string(r.Purpose),
		Providers: list(r.Providers), MatchCountries: list(r.MatchCountries), MatchCurrencies: list(r.MatchCurrencies),
		MatchPlanIDs: list(r.MatchPlanIDs), MatchUserIDs: list(r.MatchUserIDs), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

// RoutingRuleInput creates or edits a routing rule (nil = unchanged).
type RoutingRuleInput struct {
	Name            *string   `json:"name"`
	Priority        *int      `json:"priority"`
	Enabled         *bool     `json:"enabled"`
	Purpose         *string   `json:"purpose"`
	Providers       *[]string `json:"providers"`
	MatchCountries  *[]string `json:"match_countries"`
	MatchCurrencies *[]string `json:"match_currencies"`
	MatchPlanIDs    *[]string `json:"match_plan_ids"`
	MatchUserIDs    *[]string `json:"match_user_ids"`
}

// RoutingRules lists every routing rule by priority.
func (s *BillingService) RoutingRules(ctx context.Context) ([]RoutingRuleView, error) {
	rows, err := s.repo.ListRules(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]RoutingRuleView, 0, len(rows))
	for i := range rows {
		out = append(out, ruleView(&rows[i]))
	}
	return out, nil
}

// cleanList normalises a match list (case, duplicates, format).
func cleanList(field string, in []string, maxLen int, normalise func(string) (string, bool), fields map[string]string) domain.StringList {
	if len(in) > maxLen {
		fields[field] = "This list is too long."
		return nil
	}
	out := domain.StringList{}
	seen := map[string]bool{}
	for _, raw := range in {
		v, ok := normalise(strings.TrimSpace(raw))
		if !ok {
			fields[field] = "This list contains an invalid value."
			return nil
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func upperCode(re func(string) bool) func(string) (string, bool) {
	return func(v string) (string, bool) {
		v = strings.ToUpper(v)
		return v, re(v)
	}
}

func uuidText(v string) (string, bool) {
	id, err := uuid.Parse(v)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

func (s *BillingService) validateRule(in RoutingRuleInput, create bool, fields map[string]string) map[string]any {
	upd := map[string]any{}
	if in.Name != nil || create {
		name := ""
		if in.Name != nil {
			name = CleanLine(*in.Name)
		}
		if name == "" || utf8.RuneCountInString(name) > 80 {
			fields["name"] = "Use 1 to 80 characters."
		}
		upd["name"] = name
	}
	if in.Priority != nil || create {
		p := 100
		if in.Priority != nil {
			p = *in.Priority
		}
		if p < 0 || p > 100000 {
			fields["priority"] = "Use a number between 0 and 100000."
		}
		upd["priority"] = p
	}
	if in.Enabled != nil {
		upd["enabled"] = *in.Enabled
	} else if create {
		upd["enabled"] = true
	}
	if in.Purpose != nil || create {
		purpose := domain.PurposeAny
		if in.Purpose != nil {
			purpose = domain.RoutingPurpose(*in.Purpose)
		}
		if !purpose.Valid() {
			fields["purpose"] = "Choose any, credits or subscription."
		}
		upd["purpose"] = purpose
	}
	if in.Providers != nil || create {
		var list []string
		if in.Providers != nil {
			list = *in.Providers
		}
		providers := domain.StringList{}
		seen := map[string]bool{}
		for _, p := range list {
			p = strings.TrimSpace(p)
			if !payment.KnownProviders[p] && !s.knownProvider(p) {
				fields["providers"] = "Choose configured payment providers."
				break
			}
			if !seen[p] {
				seen[p] = true
				providers = append(providers, p)
			}
		}
		if len(providers) == 0 || len(providers) > 5 {
			fields["providers"] = "Choose between 1 and 5 providers."
		}
		upd["providers"] = providers
	}
	if in.MatchCountries != nil || create {
		var l []string
		if in.MatchCountries != nil {
			l = *in.MatchCountries
		}
		upd["match_countries"] = cleanList("match_countries", l, 250, upperCode(countryCode.MatchString), fields)
	}
	if in.MatchCurrencies != nil || create {
		var l []string
		if in.MatchCurrencies != nil {
			l = *in.MatchCurrencies
		}
		upd["match_currencies"] = cleanList("match_currencies", l, 50, upperCode(currencyCode.MatchString), fields)
	}
	if in.MatchPlanIDs != nil || create {
		var l []string
		if in.MatchPlanIDs != nil {
			l = *in.MatchPlanIDs
		}
		upd["match_plan_ids"] = cleanList("match_plan_ids", l, 100, uuidText, fields)
	}
	if in.MatchUserIDs != nil || create {
		var l []string
		if in.MatchUserIDs != nil {
			l = *in.MatchUserIDs
		}
		upd["match_user_ids"] = cleanList("match_user_ids", l, 1000, uuidText, fields)
	}
	return upd
}

func (s *BillingService) knownProvider(name string) bool {
	_, ok := s.registry.Get(name)
	return ok
}

// CreateRoutingRule adds a routing rule.
func (s *BillingService) CreateRoutingRule(ctx context.Context, in RoutingRuleInput) (*RoutingRuleView, error) {
	fields := map[string]string{}
	upd := s.validateRule(in, true, fields)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	now := s.clock.Now()
	r := &domain.RoutingRule{ID: domain.NewID(), Name: upd["name"].(string), Priority: upd["priority"].(int), Enabled: upd["enabled"].(bool),
		Purpose: upd["purpose"].(domain.RoutingPurpose), Providers: upd["providers"].(domain.StringList),
		MatchCountries: upd["match_countries"].(domain.StringList), MatchCurrencies: upd["match_currencies"].(domain.StringList),
		MatchPlanIDs: upd["match_plan_ids"].(domain.StringList), MatchUserIDs: upd["match_user_ids"].(domain.StringList),
		CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateRule(ctx, r); err != nil {
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "billing.routing_rule_created", "payment_routing_rule", r.ID.String(), map[string]any{"name": r.Name,
		"priority": r.Priority, "providers": []string(r.Providers), "users": len(r.MatchUserIDs), "countries": []string(r.MatchCountries)})
	v := ruleView(r)
	return &v, nil
}

// UpdateRoutingRule edits a routing rule.
func (s *BillingService) UpdateRoutingRule(ctx context.Context, id uuid.UUID, in RoutingRuleInput) (*RoutingRuleView, error) {
	fields := map[string]string{}
	upd := s.validateRule(in, false, fields)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	changed := make([]string, 0, len(upd))
	for k := range upd {
		changed = append(changed, k)
	}
	sort.Strings(changed)
	upd["updated_at"] = s.clock.Now()
	if err := s.repo.UpdateRule(ctx, id, upd); err != nil {
		return nil, mapRepoErr(err, "Routing rule")
	}
	r, err := s.repo.GetRule(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Routing rule")
	}
	s.record(ctx, "billing.routing_rule_updated", "payment_routing_rule", id.String(), map[string]any{"fields": changed})
	v := ruleView(r)
	return &v, nil
}

// DeleteRoutingRule removes a routing rule.
func (s *BillingService) DeleteRoutingRule(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.SoftDeleteRule(ctx, id, s.clock.Now()); err != nil {
		return mapRepoErr(err, "Routing rule")
	}
	s.record(ctx, "billing.routing_rule_deleted", "payment_routing_rule", id.String(), nil)
	return nil
}

// SimulateRouteInput describes a hypothetical payment.
type SimulateRouteInput struct {
	UserID   *uuid.UUID `json:"user_id"`
	Country  string     `json:"country"`
	Currency string     `json:"currency"`
	Purpose  string     `json:"purpose"`
	PlanID   *uuid.UUID `json:"plan_id"`
}

// RouteSimulation is where a payment would go.
type RouteSimulation struct {
	Providers []string `json:"providers"`
	RuleID    string   `json:"rule_id,omitempty"`
}

// SimulateRoute shows which providers a payment would use.
func (s *BillingService) SimulateRoute(ctx context.Context, in SimulateRouteInput) (*RouteSimulation, error) {
	fields := map[string]string{}
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if country != "" && !countryCode.MatchString(country) {
		fields["country"] = "Use a two-letter country code."
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = s.cfg.Currency
	}
	if !currencyCode.MatchString(currency) {
		fields["currency"] = "Use a three-letter currency code."
	}
	purpose := in.Purpose
	if purpose == "" {
		purpose = payment.PurposeCredits
	}
	if purpose != payment.PurposeCredits && purpose != payment.PurposeSubscription {
		fields["purpose"] = "Choose credits or subscription."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	route := payment.RouteInput{Countries: countries(country), Currency: currency, Purpose: purpose}
	if in.UserID != nil {
		route.UserID = in.UserID.String()
	}
	if in.PlanID != nil {
		route.PlanID = in.PlanID.String()
	}
	providers, ruleID, err := s.route(ctx, route)
	out := &RouteSimulation{Providers: []string{}, RuleID: ruleID}
	if err != nil {
		if s.registry.Empty() || errors.Is(err, payment.ErrNoProvider) {
			return out, nil
		}
		return nil, apperror.Internal(err)
	}
	for _, p := range providers {
		out.Providers = append(out.Providers, p.Name())
	}
	return out, nil
}
