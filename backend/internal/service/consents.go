package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// ConsentPolicyVersion identifies the privacy notice the choices refer to;
// bump it when the categories or their description change (clients then
// re-ask anyone whose choices predate it).
const ConsentPolicyVersion = "2026-11"

// ConsentEssential is the fixed category for storage the service cannot
// work without (sign-in cookies, CSRF, language). It is never stored.
const ConsentEssential = "essential"

// consentDefaults are the choices that apply until the user decides.
//
//   - analytics and marketing_attribution are consent-based (ePrivacy): off
//     until granted; GPC/DNT keep them off for anonymous visitors.
//   - personalization (tailoring readings, including advisors sharing their
//     notes about a client with the client's other advisors for continuity)
//     rests on legitimate interest: on by default, and withdrawing it is an
//     objection honoured at once — shared notes about the user are hidden
//     from other advisors (NoteService sharing policy).
var consentDefaults = map[domain.ConsentCategory]bool{
	domain.ConsentAnalytics:            false,
	domain.ConsentMarketingAttribution: false,
	domain.ConsentPersonalization:      true,
}

var consentBasis = map[string]string{
	ConsentEssential:                           "contract",
	string(domain.ConsentAnalytics):            "consent",
	string(domain.ConsentMarketingAttribution): "consent",
	string(domain.ConsentPersonalization):      "legitimate_interest",
}

// ConsentSources a choice may come from.
var consentSources = map[string]bool{"settings": true, "signup": true, "banner": true}

// ConsentCategoryPolicy describes one category of the policy.
type ConsentCategoryPolicy struct {
	Category string `json:"category"`
	// Required categories cannot be refused (essential only).
	Required bool `json:"required"`
	// Default applies until the user chooses.
	Default bool `json:"default"`
	// LegalBasis is contract, consent or legitimate_interest.
	LegalBasis string `json:"legal_basis"`
}

// ConsentPolicyView is the public consent policy (GET /consent-policy).
type ConsentPolicyView struct {
	Version    string                  `json:"version"`
	Categories []ConsentCategoryPolicy `json:"categories"`
	// HonorGPC: browsers sending Global Privacy Control or Do Not Track get
	// analytics and attribution off unless the user opts in explicitly.
	HonorGPC bool `json:"honor_gpc"`
}

// ConsentView is the user's current choice for one category.
type ConsentView struct {
	Category string `json:"category"`
	Granted  bool   `json:"granted"`
	Required bool   `json:"required"`
	// Explicit is false while the default applies.
	Explicit      bool       `json:"explicit"`
	PolicyVersion string     `json:"policy_version,omitempty"`
	Source        string     `json:"source,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// ConsentsView lists the user's choices (essential first).
type ConsentsView struct {
	PolicyVersion string        `json:"policy_version"`
	Items         []ConsentView `json:"items"`
	// Outdated is set when a recorded choice predates the current policy.
	Outdated bool `json:"outdated"`
}

// ConsentInput changes choices; categories left out are unchanged.
type ConsentInput struct {
	Consents map[string]bool `json:"consents"`
	// Source is settings (default), banner or signup.
	Source string `json:"source"`
}

// ConsentService records consent choices with an append-only history.
type ConsentService struct {
	tx       database.TxManager
	repo     repository.ConsentRepository
	audit    repository.AuditRepository
	clock    clock.Clock
	ipSecret string
}

// NewConsentService wires the consent service.
func NewConsentService(tx database.TxManager, repo repository.ConsentRepository, audit repository.AuditRepository, clk clock.Clock,
	ipSecret string) *ConsentService {
	return &ConsentService{tx: tx, repo: repo, audit: audit, clock: clk, ipSecret: ipSecret}
}

// Policy returns the consent policy.
func (s *ConsentService) Policy() ConsentPolicyView {
	out := ConsentPolicyView{Version: ConsentPolicyVersion, HonorGPC: true,
		Categories: []ConsentCategoryPolicy{{Category: ConsentEssential, Required: true, Default: true, LegalBasis: consentBasis[ConsentEssential]}}}
	for _, c := range domain.ConsentCategories {
		out.Categories = append(out.Categories, ConsentCategoryPolicy{Category: string(c), Default: consentDefaults[c],
			LegalBasis: consentBasis[string(c)]})
	}
	return out
}

// ParseConsentChoices validates category → granted choices. Problems are
// reported under prefix + category (e.g. "privacy.consents.analytics").
func ParseConsentChoices(in map[string]bool, prefix string, fields map[string]string) map[domain.ConsentCategory]bool {
	out := make(map[domain.ConsentCategory]bool, len(in))
	for k, v := range in {
		switch {
		case k == ConsentEssential:
			if !v {
				fields[prefix+k] = "Essential storage cannot be turned off."
			}
		case domain.ConsentCategory(k).Valid():
			out[domain.ConsentCategory(k)] = v
		default:
			fields[prefix+k] = "Unknown consent category."
		}
	}
	return out
}

func (s *ConsentService) view(rows []domain.Consent) *ConsentsView {
	byCat := make(map[domain.ConsentCategory]domain.Consent, len(rows))
	for _, r := range rows {
		byCat[r.Category] = r
	}
	out := &ConsentsView{PolicyVersion: ConsentPolicyVersion,
		Items: []ConsentView{{Category: ConsentEssential, Granted: true, Required: true, Explicit: true}}}
	for _, c := range domain.ConsentCategories {
		v := ConsentView{Category: string(c), Granted: consentDefaults[c]}
		if r, ok := byCat[c]; ok {
			at := r.UpdatedAt
			v.Granted, v.Explicit, v.PolicyVersion, v.Source, v.UpdatedAt = r.Granted, true, r.PolicyVersion, r.Source, &at
			if r.PolicyVersion != ConsentPolicyVersion {
				out.Outdated = true
			}
		}
		out.Items = append(out.Items, v)
	}
	return out
}

// Get returns the user's choices (defaults where none was recorded).
func (s *ConsentService) Get(ctx context.Context, userID uuid.UUID) (*ConsentsView, error) {
	rows, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return s.view(rows), nil
}

// Effective returns the current choice per category (defaults applied).
func (s *ConsentService) Effective(ctx context.Context, userID uuid.UUID) (map[domain.ConsentCategory]bool, error) {
	rows, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.ConsentCategory]bool, len(consentDefaults))
	for c, v := range consentDefaults {
		out[c] = v
	}
	for _, r := range rows {
		out[r.Category] = r.Granted
	}
	return out, nil
}

// Allows reports whether the user's current choice permits a category.
func (s *ConsentService) Allows(ctx context.Context, userID uuid.UUID, c domain.ConsentCategory) (bool, error) {
	r, err := s.repo.Get(ctx, userID, c)
	if errors.Is(err, repository.ErrNotFound) {
		return consentDefaults[c], nil
	}
	if err != nil {
		return false, err
	}
	return r.Granted, nil
}

// NoteSharing is the NoteService sharing policy: advisors see notes other
// advisors shared about a client only while the client's personalization
// choice allows it.
func (s *ConsentService) NoteSharing(ctx context.Context, clientID uuid.UUID) (bool, error) {
	return s.Allows(ctx, clientID, domain.ConsentPersonalization)
}

// Set validates and records new choices (PUT /me/consents).
func (s *ConsentService) Set(ctx context.Context, userID uuid.UUID, in ConsentInput) (*ConsentsView, error) {
	fields := map[string]string{}
	source := in.Source
	if source == "" {
		source = "settings"
	}
	if !consentSources[source] {
		fields["source"] = "Use settings, banner or signup."
	}
	if len(in.Consents) == 0 {
		fields["consents"] = "Choose at least one category."
	}
	choices := ParseConsentChoices(in.Consents, "consents.", fields)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var changed []string
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		changed, err = s.apply(ctx, userID, choices, source)
		return err
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(changed) > 0 {
		s.record(ctx, userID, choices, changed, source)
	}
	return s.Get(ctx, userID)
}

// apply writes the choices that differ from the recorded ones (or that
// were never recorded) and appends them to the history; it must run in a
// transaction. It returns the changed categories.
func (s *ConsentService) apply(ctx context.Context, userID uuid.UUID, choices map[domain.ConsentCategory]bool, source string) ([]string, error) {
	if len(choices) == 0 {
		return nil, nil
	}
	rows, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	current := make(map[domain.ConsentCategory]domain.Consent, len(rows))
	for _, r := range rows {
		current[r.Category] = r
	}
	now := s.clock.Now()
	ipHash := securityKeyed(s.ipSecret, reqctx.MetaFrom(ctx).IP)
	var changed []string
	for _, c := range domain.ConsentCategories {
		granted, asked := choices[c]
		if !asked {
			continue
		}
		if r, ok := current[c]; ok && r.Granted == granted && r.PolicyVersion == ConsentPolicyVersion {
			continue
		}
		if err := s.repo.Upsert(ctx, &domain.Consent{UserID: userID, Category: c, Granted: granted, PolicyVersion: ConsentPolicyVersion,
			Source: source, CreatedAt: now, UpdatedAt: now}); err != nil {
			return nil, err
		}
		if err := s.repo.AddEvent(ctx, &domain.ConsentEvent{ID: domain.NewID(), UserID: userID, Category: c, Granted: granted,
			PolicyVersion: ConsentPolicyVersion, Source: source, IPHash: ipHash, CreatedAt: now}); err != nil {
			return nil, err
		}
		changed = append(changed, string(c))
	}
	sort.Strings(changed)
	return changed, nil
}

func (s *ConsentService) record(ctx context.Context, userID uuid.UUID, choices map[domain.ConsentCategory]bool, changed []string, source string) {
	meta := map[string]any{"source": source, "policy_version": ConsentPolicyVersion}
	for _, c := range changed {
		meta[c] = choices[domain.ConsentCategory(c)]
	}
	recordAudit(ctx, s.audit, s.clock, s.ipSecret, "consent.updated", "user", userID.String(), meta)
}
