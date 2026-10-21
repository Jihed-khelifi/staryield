package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// GrowthSettings configure marketing links.
type GrowthSettings struct {
	FrontendURL   string
	PublicBaseURL string
	AllowedHosts  []string
	Currency      string
	IPHashSecret  string
}

// GrowthService manages campaigns, short links, click tracking, sign-up
// attribution and acquisition reports.
type GrowthService struct {
	repo     repository.GrowthRepository
	audit    repository.AuditRepository
	tracker  analytics.Tracker
	clock    clock.Clock
	cfg      GrowthSettings
	hosts    map[string]bool
	frontend *url.URL
}

// NewGrowthService wires the growth service.
func NewGrowthService(repo repository.GrowthRepository, audit repository.AuditRepository, tracker analytics.Tracker, clk clock.Clock, cfg GrowthSettings) *GrowthService {
	front, err := url.Parse(cfg.FrontendURL)
	if err != nil || front.Host == "" {
		front, _ = url.Parse("http://localhost:3000")
	}
	hosts := map[string]bool{strings.ToLower(front.Host): true}
	for _, h := range cfg.AllowedHosts {
		hosts[strings.ToLower(strings.TrimSpace(h))] = true
	}
	return &GrowthService{repo: repo, audit: audit, tracker: tracker, clock: clk, cfg: cfg, hosts: hosts, frontend: front}
}

func (s *GrowthService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	recordAudit(ctx, s.audit, s.clock, s.cfg.IPHashSecret, action, targetType, targetID, meta)
}

// ---------------------------------------------------------------------------
// Campaigns
// ---------------------------------------------------------------------------

// CampaignMetrics is a campaign's funnel over a window.
type CampaignMetrics struct {
	Clicks       int64  `json:"clicks"`
	UniqueClicks int64  `json:"unique_clicks"`
	Signups      int64  `json:"signups"`
	Purchasers   int64  `json:"purchasers"`
	RevenueCents int64  `json:"revenue_cents"`
	Currency     string `json:"currency"`
}

// CampaignView is a campaign as seen by administrators.
type CampaignView struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	Slug        string           `json:"slug"`
	Channel     string           `json:"channel"`
	Status      string           `json:"status"`
	BudgetCents int64            `json:"budget_cents"`
	Currency    string           `json:"currency"`
	StartsAt    *time.Time       `json:"starts_at,omitempty"`
	EndsAt      *time.Time       `json:"ends_at,omitempty"`
	Notes       string           `json:"notes"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Metrics     *CampaignMetrics `json:"metrics,omitempty"`
}

func campaignView(c *domain.Campaign) CampaignView {
	return CampaignView{ID: c.ID, Name: c.Name, Slug: c.Slug, Channel: c.Channel, Status: c.Status, BudgetCents: c.BudgetCents,
		Currency: c.Currency, StartsAt: c.StartsAt, EndsAt: c.EndsAt, Notes: c.Notes, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

// CampaignInput creates or updates a campaign.
type CampaignInput struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Channel     *string `json:"channel"`
	Status      *string `json:"status"`
	BudgetCents *int64  `json:"budget_cents"`
	Currency    *string `json:"currency"`
	// StartsAt / EndsAt accept null to clear the date.
	StartsAt Optional[time.Time] `json:"starts_at"`
	EndsAt   Optional[time.Time] `json:"ends_at"`
	Notes    *string             `json:"notes"`
}

var campaignSlug = regexp.MustCompile(`^[a-z0-9]+([-_][a-z0-9]+)*$`)

var campaignStatuses = map[string]bool{"draft": true, "active": true, "paused": true, "ended": true, "archived": true}

func validateCampaign(in CampaignInput, creating bool) (map[string]any, map[string]string) {
	upd, fields := map[string]any{}, map[string]string{}
	if in.Name != nil {
		v := CleanLine(*in.Name)
		if v == "" || utf8.RuneCountInString(v) > 120 {
			fields["name"] = "Use between 1 and 120 characters."
		} else {
			upd["name"] = v
		}
	} else if creating {
		fields["name"] = "This field is required."
	}
	if in.Slug != nil {
		v := strings.TrimSpace(*in.Slug)
		if !campaignSlug.MatchString(v) || len(v) > 64 {
			fields["slug"] = "Use lowercase letters, digits, hyphens and underscores."
		} else {
			upd["slug"] = v
		}
	} else if creating {
		fields["slug"] = "This field is required."
	}
	if in.Channel != nil {
		v := CleanLine(*in.Channel)
		if utf8.RuneCountInString(v) > 40 {
			fields["channel"] = "Use at most 40 characters."
		} else {
			upd["channel"] = v
		}
	}
	if in.Status != nil {
		if !campaignStatuses[*in.Status] {
			fields["status"] = "Choose draft, active, paused, ended or archived."
		} else {
			upd["status"] = *in.Status
		}
	}
	if in.BudgetCents != nil {
		if *in.BudgetCents < 0 || *in.BudgetCents > 100_000_000_000 {
			fields["budget_cents"] = "Use a budget between 0 and 1,000,000,000.00."
		} else {
			upd["budget_cents"] = *in.BudgetCents
		}
	}
	if in.Currency != nil {
		c := strings.ToUpper(strings.TrimSpace(*in.Currency))
		if len(c) != 3 {
			fields["currency"] = "Use a 3-letter ISO currency code."
		} else {
			upd["currency"] = c
		}
	}
	for col, opt := range map[string]Optional[time.Time]{"starts_at": in.StartsAt, "ends_at": in.EndsAt} {
		if !opt.Set {
			continue
		}
		if opt.Value == nil {
			upd[col] = nil // cleared
		} else {
			upd[col] = opt.Value.UTC()
		}
	}
	if in.StartsAt.Value != nil && in.EndsAt.Value != nil && in.EndsAt.Value.Before(*in.StartsAt.Value) {
		fields["ends_at"] = "The end date must be after the start date."
	}
	if in.Notes != nil {
		v := CleanText(*in.Notes)
		if utf8.RuneCountInString(v) > 2000 {
			fields["notes"] = "Use at most 2000 characters."
		} else {
			upd["notes"] = v
		}
	}
	return upd, fields
}

// CreateCampaign adds a campaign.
func (s *GrowthService) CreateCampaign(ctx context.Context, in CampaignInput) (*CampaignView, error) {
	upd, fields := validateCampaign(in, true)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	now := s.clock.Now()
	p, _ := reqctx.PrincipalFrom(ctx)
	c := &domain.Campaign{ID: domain.NewID(), Name: upd["name"].(string), Slug: upd["slug"].(string), Status: "draft",
		Currency: s.cfg.Currency, CreatedAt: now, UpdatedAt: now}
	if p.UserID != uuid.Nil {
		c.CreatedBy = &p.UserID
	}
	if v, ok := upd["channel"].(string); ok {
		c.Channel = v
	}
	if v, ok := upd["status"].(string); ok {
		c.Status = v
	}
	if v, ok := upd["budget_cents"].(int64); ok {
		c.BudgetCents = v
	}
	if v, ok := upd["currency"].(string); ok {
		c.Currency = v
	}
	if v, ok := upd["starts_at"].(time.Time); ok {
		c.StartsAt = &v
	}
	if v, ok := upd["ends_at"].(time.Time); ok {
		c.EndsAt = &v
	}
	if v, ok := upd["notes"].(string); ok {
		c.Notes = v
	}
	if err := s.repo.CreateCampaign(ctx, c); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.Conflict(apperror.CodeConflict, "Another campaign already uses this slug.")
		}
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "campaign.created", "campaign", c.ID.String(), map[string]any{"slug": c.Slug, "budget_cents": c.BudgetCents})
	v := campaignView(c)
	return &v, nil
}

// Campaign returns one campaign.
func (s *GrowthService) Campaign(ctx context.Context, id uuid.UUID) (*CampaignView, error) {
	c, err := s.repo.GetCampaign(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Campaign")
	}
	v := campaignView(c)
	return &v, nil
}

// UpdateCampaign edits a campaign.
func (s *GrowthService) UpdateCampaign(ctx context.Context, id uuid.UUID, in CampaignInput) (*CampaignView, error) {
	upd, fields := validateCampaign(in, false)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"name": "Nothing to update."})
	}
	before, err := s.repo.GetCampaign(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Campaign")
	}
	starts, ends := before.StartsAt, before.EndsAt
	if in.StartsAt.Set {
		starts = in.StartsAt.Value
	}
	if in.EndsAt.Set {
		ends = in.EndsAt.Value
	}
	if starts != nil && ends != nil && ends.Before(*starts) {
		return nil, apperror.Validation(map[string]string{"ends_at": "The end date must be after the start date."})
	}
	if err := s.repo.UpdateCampaign(ctx, id, upd); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.Conflict(apperror.CodeConflict, "Another campaign already uses this slug.")
		}
		return nil, mapRepoErr(err, "Campaign")
	}
	meta := map[string]any{}
	for k, v := range upd {
		if k != "updated_at" && k != "notes" {
			meta[k] = v
		}
	}
	s.record(ctx, "campaign.updated", "campaign", id.String(), meta)
	return s.Campaign(ctx, id)
}

// DeleteCampaign deletes an unused campaign, or archives one with history.
func (s *GrowthService) DeleteCampaign(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetCampaign(ctx, id); err != nil {
		return mapRepoErr(err, "Campaign")
	}
	used, err := s.repo.CampaignInUse(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if used {
		if err := s.repo.UpdateCampaign(ctx, id, map[string]any{"status": "archived"}); err != nil {
			return mapRepoErr(err, "Campaign")
		}
		s.record(ctx, "campaign.archived", "campaign", id.String(), nil)
		return nil
	}
	if err := s.repo.DeleteCampaign(ctx, id); err != nil {
		return mapRepoErr(err, "Campaign")
	}
	s.record(ctx, "campaign.deleted", "campaign", id.String(), nil)
	return nil
}

// Campaigns lists campaigns with their last-30-days funnel.
func (s *GrowthService) Campaigns(ctx context.Context, status string) ([]CampaignView, error) {
	if status != "" && !campaignStatuses[status] {
		return nil, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	rows, err := s.repo.ListCampaigns(ctx, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// "Last 30 days" includes events recorded this very instant.
	to := s.clock.Now().Add(time.Minute)
	counts, err := s.repo.ByCampaign(ctx, to.AddDate(0, 0, -30), to, s.cfg.Currency)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byID := map[uuid.UUID]repository.CampaignCounts{}
	for _, c := range counts {
		byID[c.CampaignID] = c
	}
	out := make([]CampaignView, 0, len(rows))
	for i := range rows {
		v := campaignView(&rows[i])
		c := byID[rows[i].ID]
		v.Metrics = &CampaignMetrics{Clicks: c.Clicks, UniqueClicks: c.UniqueClicks, Signups: c.Signups, Purchasers: c.Purchasers,
			RevenueCents: c.RevenueCents, Currency: s.cfg.Currency}
		out = append(out, v)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Short links
// ---------------------------------------------------------------------------

// LinkView is a short link as seen by administrators.
type LinkView struct {
	ID           uuid.UUID  `json:"id"`
	Code         string     `json:"code"`
	ShortURL     string     `json:"short_url"`
	Destination  string     `json:"destination"`
	FullURL      string     `json:"full_url"`
	CampaignID   *uuid.UUID `json:"campaign_id,omitempty"`
	CampaignName string     `json:"campaign_name,omitempty"`
	UTMSource    string     `json:"utm_source"`
	UTMMedium    string     `json:"utm_medium"`
	UTMCampaign  string     `json:"utm_campaign"`
	UTMContent   string     `json:"utm_content"`
	UTMTerm      string     `json:"utm_term"`
	Label        string     `json:"label"`
	Active       bool       `json:"active"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Clicks       int64      `json:"clicks"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// LinkInput creates or updates a short link.
type LinkInput struct {
	Code        *string `json:"code"`
	Destination *string `json:"destination"`
	// CampaignID accepts null to detach the link from its campaign.
	CampaignID  Optional[uuid.UUID] `json:"campaign_id"`
	UTMSource   *string             `json:"utm_source"`
	UTMMedium   *string             `json:"utm_medium"`
	UTMCampaign *string             `json:"utm_campaign"`
	UTMContent  *string             `json:"utm_content"`
	UTMTerm     *string             `json:"utm_term"`
	Label       *string             `json:"label"`
	// ExpiresAt accepts null to remove the expiry.
	ExpiresAt Optional[time.Time] `json:"expires_at"`
	Active    *bool               `json:"active"`
}

var (
	linkCode = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)
	utmValue = regexp.MustCompile(`^[A-Za-z0-9._\-+ ]{1,100}$`)
)

// ValidLinkCode reports whether code is a well-formed short code.
func ValidLinkCode(code string) bool { return linkCode.MatchString(code) }

// NormalizeDestination validates a link destination: a path on the main
// site or an absolute https URL on an allowed host. It returns the stored
// form (paths stay relative).
func (s *GrowthService) NormalizeDestination(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, " \t\r\n\\") {
		return "", false
	}
	if strings.HasPrefix(raw, "/") {
		if strings.HasPrefix(raw, "//") {
			return "", false
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host != "" || u.Scheme != "" {
			return "", false
		}
		return u.String(), true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !s.hosts[strings.ToLower(u.Host)] {
		return "", false
	}
	return u.String(), true
}

// FullURL builds the landing URL with UTM parameters (no click id).
func (s *GrowthService) FullURL(l *domain.ShortLink) string {
	var u *url.URL
	if strings.HasPrefix(l.Destination, "/") {
		rel, _ := url.Parse(l.Destination)
		u = s.frontend.ResolveReference(rel)
	} else {
		u, _ = url.Parse(l.Destination)
	}
	if u == nil {
		return s.frontend.String()
	}
	q := u.Query()
	for k, v := range map[string]string{"utm_source": l.UTMSource, "utm_medium": l.UTMMedium, "utm_campaign": l.UTMCampaign,
		"utm_content": l.UTMContent, "utm_term": l.UTMTerm} {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// LinkSettings tells the admin console how short links are built.
type LinkSettings struct {
	PublicBaseURL string   `json:"public_base_url"`
	FrontendURL   string   `json:"frontend_url"`
	AllowedHosts  []string `json:"allowed_hosts"`
	Currency      string   `json:"currency"`
}

// Settings returns the short-link configuration.
func (s *GrowthService) Settings() LinkSettings {
	hosts := make([]string, 0, len(s.hosts))
	for h := range s.hosts {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return LinkSettings{PublicBaseURL: s.cfg.PublicBaseURL, FrontendURL: strings.TrimRight(s.frontend.String(), "/"), AllowedHosts: hosts,
		Currency: s.cfg.Currency}
}

// ShortURL returns the public short URL of a code.
func (s *GrowthService) ShortURL(code string) string { return s.cfg.PublicBaseURL + "/" + code }

func (s *GrowthService) linkView(l *domain.ShortLink, campaignName string) LinkView {
	return LinkView{ID: l.ID, Code: l.Code, ShortURL: s.ShortURL(l.Code), Destination: l.Destination, FullURL: s.FullURL(l),
		CampaignID: l.CampaignID, CampaignName: campaignName, UTMSource: l.UTMSource, UTMMedium: l.UTMMedium, UTMCampaign: l.UTMCampaign,
		UTMContent: l.UTMContent, UTMTerm: l.UTMTerm, Label: l.Label, Active: l.Active, ExpiresAt: l.ExpiresAt, Clicks: l.Clicks,
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt}
}

func (s *GrowthService) validateLink(ctx context.Context, in LinkInput, creating bool) (map[string]any, map[string]string, *domain.Campaign) {
	upd, fields := map[string]any{}, map[string]string{}
	if in.Destination != nil {
		if d, ok := s.NormalizeDestination(*in.Destination); ok {
			upd["destination"] = d
		} else {
			fields["destination"] = "Use a site path such as /en/psychics or an https link on an allowed domain."
		}
	} else if creating {
		fields["destination"] = "This field is required."
	}
	utm := func(p *string, col string, required bool) {
		if p == nil {
			if required {
				fields[col] = "This field is required."
			}
			return
		}
		v := strings.TrimSpace(*p)
		if v == "" {
			if required {
				fields[col] = "This field is required."
			} else {
				upd[col] = ""
			}
			return
		}
		if !utmValue.MatchString(v) {
			fields[col] = "Use letters, digits, spaces, dots, dashes, underscores or plus signs (up to 100)."
			return
		}
		upd[col] = v
	}
	utm(in.UTMSource, "utm_source", creating)
	utm(in.UTMMedium, "utm_medium", creating)
	utm(in.UTMCampaign, "utm_campaign", false)
	utm(in.UTMContent, "utm_content", false)
	utm(in.UTMTerm, "utm_term", false)
	if in.Label != nil {
		v := CleanLine(*in.Label)
		if utf8.RuneCountInString(v) > 120 {
			fields["label"] = "Use at most 120 characters."
		} else {
			upd["label"] = v
		}
	}
	if in.ExpiresAt.Set {
		switch {
		case in.ExpiresAt.Value == nil:
			upd["expires_at"] = nil
		case !in.ExpiresAt.Value.After(s.clock.Now()):
			fields["expires_at"] = "The expiry must be in the future."
		default:
			upd["expires_at"] = in.ExpiresAt.Value.UTC()
		}
	}
	if in.Active != nil {
		upd["active"] = *in.Active
	}
	var campaign *domain.Campaign
	if in.CampaignID.Set && in.CampaignID.Value == nil {
		upd["campaign_id"] = nil
	} else if in.CampaignID.Value != nil {
		c, err := s.repo.GetCampaign(ctx, *in.CampaignID.Value)
		if err != nil {
			fields["campaign_id"] = "Unknown campaign."
		} else {
			campaign = c
			upd["campaign_id"] = c.ID
			if _, set := upd["utm_campaign"]; !set || upd["utm_campaign"] == "" {
				upd["utm_campaign"] = c.Slug
			}
		}
	}
	return upd, fields, campaign
}

// randomCode returns a 7-character base62 code (~3.5e12 combinations).
func randomCode() string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 7)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[n.Int64()]
	}
	return string(b)
}

// reservedCodes cannot be chosen as custom short codes.
var reservedCodes = map[string]bool{"admin": true, "api": true, "login": true, "signup": true, "go": true, "static": true}

// CreateLink adds a short link.
func (s *GrowthService) CreateLink(ctx context.Context, in LinkInput) (*LinkView, error) {
	upd, fields, campaign := s.validateLink(ctx, in, true)
	custom := ""
	if in.Code != nil && strings.TrimSpace(*in.Code) != "" {
		custom = strings.TrimSpace(*in.Code)
		if !ValidLinkCode(custom) || reservedCodes[strings.ToLower(custom)] {
			fields["code"] = "Use 3–32 letters, digits, hyphens or underscores."
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	now := s.clock.Now()
	l := &domain.ShortLink{ID: domain.NewID(), Destination: upd["destination"].(string), UTMSource: upd["utm_source"].(string),
		UTMMedium: upd["utm_medium"].(string), Active: true, CreatedAt: now, UpdatedAt: now}
	if p, ok := reqctx.PrincipalFrom(ctx); ok {
		l.CreatedBy = &p.UserID
	}
	if campaign != nil {
		l.CampaignID = &campaign.ID
	}
	for col, dst := range map[string]*string{"utm_campaign": &l.UTMCampaign, "utm_content": &l.UTMContent, "utm_term": &l.UTMTerm, "label": &l.Label} {
		if v, ok := upd[col].(string); ok {
			*dst = v
		}
	}
	if v, ok := upd["expires_at"].(time.Time); ok {
		l.ExpiresAt = &v
	}
	if v, ok := upd["active"].(bool); ok {
		l.Active = v
	}
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		l.Code = custom
		if custom == "" {
			l.Code = randomCode()
		}
		if err = s.repo.CreateLink(ctx, l); !errors.Is(err, repository.ErrDuplicate) {
			break
		}
		if custom != "" {
			return nil, apperror.Conflict(apperror.CodeConflict, "This short code is already taken.")
		}
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "link.created", "short_link", l.ID.String(), map[string]any{"code": l.Code, "destination": l.Destination,
		"utm_source": l.UTMSource, "utm_medium": l.UTMMedium, "utm_campaign": l.UTMCampaign})
	name := ""
	if campaign != nil {
		name = campaign.Name
	}
	v := s.linkView(l, name)
	return &v, nil
}

// Link returns one short link.
func (s *GrowthService) Link(ctx context.Context, id uuid.UUID) (*LinkView, error) {
	l, err := s.repo.GetLink(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Link")
	}
	name := ""
	if l.CampaignID != nil {
		if c, err := s.repo.GetCampaign(ctx, *l.CampaignID); err == nil {
			name = c.Name
		}
	}
	v := s.linkView(l, name)
	return &v, nil
}

// UpdateLink edits a short link (the code is immutable).
func (s *GrowthService) UpdateLink(ctx context.Context, id uuid.UUID, in LinkInput) (*LinkView, error) {
	if in.Code != nil {
		return nil, apperror.Validation(map[string]string{"code": "The short code cannot be changed."})
	}
	upd, fields, _ := s.validateLink(ctx, in, false)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) == 0 {
		return nil, apperror.Validation(map[string]string{"destination": "Nothing to update."})
	}
	if err := s.repo.UpdateLink(ctx, id, upd); err != nil {
		return nil, mapRepoErr(err, "Link")
	}
	meta := map[string]any{}
	for k, v := range upd {
		if k != "updated_at" {
			meta[k] = v
		}
	}
	s.record(ctx, "link.updated", "short_link", id.String(), meta)
	return s.Link(ctx, id)
}

// DeleteLink deletes an unused link or deactivates one with clicks.
func (s *GrowthService) DeleteLink(ctx context.Context, id uuid.UUID) error {
	l, err := s.repo.GetLink(ctx, id)
	if err != nil {
		return mapRepoErr(err, "Link")
	}
	if l.Clicks > 0 {
		if err := s.repo.UpdateLink(ctx, id, map[string]any{"active": false}); err != nil {
			return mapRepoErr(err, "Link")
		}
		s.record(ctx, "link.deactivated", "short_link", id.String(), map[string]any{"code": l.Code})
		return nil
	}
	if err := s.repo.DeleteLink(ctx, id); err != nil {
		if errors.Is(err, repository.ErrInUse) {
			return apperror.Conflict(apperror.CodeConflict, "This link has history. Deactivate it instead.")
		}
		// Bot clicks or attributions may still reference it.
		if err2 := s.repo.UpdateLink(ctx, id, map[string]any{"active": false}); err2 == nil {
			s.record(ctx, "link.deactivated", "short_link", id.String(), map[string]any{"code": l.Code})
			return nil
		}
		return mapRepoErr(err, "Link")
	}
	s.record(ctx, "link.deleted", "short_link", id.String(), map[string]any{"code": l.Code})
	return nil
}

// Links lists short links.
func (s *GrowthService) Links(ctx context.Context, f repository.LinkFilter, cursor string) (Page[LinkView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[LinkView]{}, err
	}
	f.Cursor = c
	f.Query = truncateString(strings.TrimSpace(f.Query), 100)
	rows, err := s.repo.ListLinks(ctx, f)
	if err != nil {
		return Page[LinkView]{}, apperror.Internal(err)
	}
	ids := []uuid.UUID{}
	for _, l := range rows {
		if l.CampaignID != nil {
			ids = append(ids, *l.CampaignID)
		}
	}
	names, _ := s.repo.CampaignNames(ctx, ids)
	page := Page[LinkView]{Items: make([]LinkView, 0, len(rows))}
	for i := range rows {
		name := ""
		if rows[i].CampaignID != nil {
			name = names[*rows[i].CampaignID]
		}
		page.Items = append(page.Items, s.linkView(&rows[i], name))
	}
	if len(rows) > 0 && len(rows) == clampPage(f.Limit, 50, 200) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// ---------------------------------------------------------------------------
// Click tracking
// ---------------------------------------------------------------------------

var (
	botUA    = regexp.MustCompile(`(?i)(bot\b|bot/|crawl|spider|slurp|facebookexternalhit|facebot|twitterbot|slackbot|discordbot|whatsapp|telegrambot|linkedinbot|embedly|pinterestbot|redditbot|skypeuripreview|bingpreview|googleother|headlesschrome|preview|curl/|wget/|python-requests|go-http-client|okhttp|axios/)`)
	tabletUA = regexp.MustCompile(`(?i)(ipad|tablet|kindle|silk|playbook)`)
	mobileUA = regexp.MustCompile(`(?i)(mobi|iphone|ipod|android|blackberry|opera mini|iemobile)`)
)

// ClassifyDevice maps a user agent to bot, tablet, mobile or desktop.
func ClassifyDevice(ua string) string {
	switch {
	case strings.TrimSpace(ua) == "" || botUA.MatchString(ua):
		return "bot"
	case tabletUA.MatchString(ua):
		return "tablet"
	case strings.Contains(strings.ToLower(ua), "android") && !strings.Contains(strings.ToLower(ua), "mobile"):
		return "tablet"
	case mobileUA.MatchString(ua):
		return "mobile"
	}
	return "desktop"
}

// referrerHost returns the lowercased host of a referrer URL.
func referrerHost(ref string) string {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil || u.Host == "" {
		return ""
	}
	return truncateString(strings.ToLower(u.Hostname()), 255)
}

// Click records a short-link visit and returns where to redirect. Unknown,
// inactive or expired codes send visitors to the home page.
func (s *GrowthService) Click(ctx context.Context, code, ip, userAgent, referrer string) string {
	home := s.frontend.String()
	if !ValidLinkCode(code) {
		return home
	}
	l, err := s.repo.GetLinkByCode(ctx, code)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "growth: link lookup", err)
		}
		return home
	}
	now := s.clock.Now()
	if !l.Active || (l.ExpiresAt != nil && !now.Before(*l.ExpiresAt)) {
		return home
	}
	device := ClassifyDevice(userAgent)
	click := &domain.LinkClick{ID: domain.NewID(), LinkID: l.ID, CampaignID: l.CampaignID,
		VisitorHash: securityKeyed(s.cfg.IPHashSecret, ip+"|"+userAgent), ReferrerHost: referrerHost(referrer),
		Device: device, IsBot: device == "bot", CreatedAt: now}
	if err := s.repo.RecordClick(ctx, click); err != nil {
		warnErr(ctx, "growth: record click", err)
		return s.FullURL(l)
	}
	if !click.IsBot {
		s.tracker.Track(ctx, "growth.link_clicked", map[string]any{"link_id": l.ID, "campaign_id": uuidString(l.CampaignID),
			"utm_source": l.UTMSource, "utm_medium": l.UTMMedium, "device": device})
	}
	u, err := url.Parse(s.FullURL(l))
	if err != nil {
		return home
	}
	q := u.Query()
	q.Set("sy_click", click.ID.String())
	u.RawQuery = q.Encode()
	return u.String()
}

// ---------------------------------------------------------------------------
// Attribution
// ---------------------------------------------------------------------------

// AttributionInput is the first-touch data a browser captured.
type AttributionInput struct {
	ClickID     string     `json:"click_id"`
	UTMSource   string     `json:"utm_source"`
	UTMMedium   string     `json:"utm_medium"`
	UTMCampaign string     `json:"utm_campaign"`
	UTMContent  string     `json:"utm_content"`
	UTMTerm     string     `json:"utm_term"`
	Referrer    string     `json:"referrer"`
	LandingPath string     `json:"landing_path"`
	FirstSeenAt *time.Time `json:"first_seen_at"`
}

// AttributionView is a user's acquisition source.
type AttributionView struct {
	CampaignID   *uuid.UUID `json:"campaign_id,omitempty"`
	CampaignName string     `json:"campaign_name,omitempty"`
	LinkID       *uuid.UUID `json:"link_id,omitempty"`
	LinkCode     string     `json:"link_code,omitempty"`
	UTMSource    string     `json:"utm_source"`
	UTMMedium    string     `json:"utm_medium"`
	UTMCampaign  string     `json:"utm_campaign"`
	UTMContent   string     `json:"utm_content"`
	UTMTerm      string     `json:"utm_term"`
	ReferrerHost string     `json:"referrer_host"`
	LandingPath  string     `json:"landing_path"`
	FirstSeenAt  *time.Time `json:"first_seen_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func attributionView(ctx context.Context, repo repository.GrowthRepository, a *domain.Attribution) AttributionView {
	v := AttributionView{CampaignID: a.CampaignID, LinkID: a.LinkID, UTMSource: a.UTMSource, UTMMedium: a.UTMMedium,
		UTMCampaign: a.UTMCampaign, UTMContent: a.UTMContent, UTMTerm: a.UTMTerm, ReferrerHost: a.ReferrerHost,
		LandingPath: a.LandingPath, FirstSeenAt: a.FirstSeenAt, CreatedAt: a.CreatedAt}
	if a.CampaignID != nil {
		if c, err := repo.GetCampaign(ctx, *a.CampaignID); err == nil {
			v.CampaignName = c.Name
		}
	}
	if a.LinkID != nil {
		if l, err := repo.GetLink(ctx, *a.LinkID); err == nil {
			v.LinkCode = l.Code
		}
	}
	return v
}

func cleanUTM(v string) string {
	v = strings.TrimSpace(v)
	if !utmValue.MatchString(v) {
		return ""
	}
	return v
}

// Attribute stores a new user's first-touch attribution. Invalid parts are
// dropped (never trusted); failures never block sign-up.
func (s *GrowthService) Attribute(ctx context.Context, userID uuid.UUID, in *AttributionInput) {
	if in == nil {
		return
	}
	now := s.clock.Now()
	a := &domain.Attribution{UserID: userID, UTMSource: cleanUTM(in.UTMSource), UTMMedium: cleanUTM(in.UTMMedium),
		UTMCampaign: cleanUTM(in.UTMCampaign), UTMContent: cleanUTM(in.UTMContent), UTMTerm: cleanUTM(in.UTMTerm),
		ReferrerHost: referrerHost(in.Referrer), LandingPath: stripQuery(in.LandingPath, 300), CreatedAt: now}
	if !strings.HasPrefix(a.LandingPath, "/") {
		a.LandingPath = ""
	}
	if in.FirstSeenAt != nil && in.FirstSeenAt.Before(now.Add(time.Minute)) && in.FirstSeenAt.After(now.AddDate(0, 0, -90)) {
		t := in.FirstSeenAt.UTC()
		a.FirstSeenAt = &t
	}
	if id, err := uuid.Parse(strings.TrimSpace(in.ClickID)); err == nil {
		// Only clicks made in the last 90 days count; the link supplies the
		// campaign and fills UTM values the browser did not keep.
		if click, err := s.repo.GetClick(ctx, id); err == nil && click.CreatedAt.After(now.AddDate(0, 0, -90)) {
			a.ClickID, a.LinkID, a.CampaignID = &click.ID, &click.LinkID, click.CampaignID
			if l, err := s.repo.GetLink(ctx, click.LinkID); err == nil {
				if a.UTMSource == "" {
					a.UTMSource = l.UTMSource
				}
				if a.UTMMedium == "" {
					a.UTMMedium = l.UTMMedium
				}
				if a.UTMCampaign == "" {
					a.UTMCampaign = l.UTMCampaign
				}
				if a.UTMContent == "" {
					a.UTMContent = l.UTMContent
				}
			}
		}
	}
	if a.CampaignID == nil && a.UTMCampaign != "" {
		if c, err := s.repo.GetCampaignBySlug(ctx, strings.ToLower(a.UTMCampaign)); err == nil {
			a.CampaignID = &c.ID
		}
	}
	if a.ClickID == nil && a.UTMSource == "" && a.ReferrerHost == "" && a.LandingPath == "" {
		return
	}
	if err := s.repo.CreateAttribution(ctx, a); err != nil {
		warnErr(ctx, "growth: store attribution", err)
		return
	}
	s.tracker.Track(ctx, "growth.signup_attributed", map[string]any{"user_id": userID, "campaign_id": uuidString(a.CampaignID),
		"utm_source": a.UTMSource, "utm_medium": a.UTMMedium, "has_click": a.ClickID != nil})
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// GrowthTotals is the funnel summary of a report.
type GrowthTotals struct {
	Clicks                int64    `json:"clicks"`
	UniqueClicks          int64    `json:"unique_clicks"`
	BotClicks             int64    `json:"bot_clicks"`
	Signups               int64    `json:"signups"`
	Purchasers            int64    `json:"purchasers"`
	Purchases             int64    `json:"purchases"`
	RevenueCents          int64    `json:"revenue_cents"`
	Currency              string   `json:"currency"`
	ClickToSignupRate     float64  `json:"click_to_signup_rate"`
	ClickToPurchaseRate   float64  `json:"click_to_purchase_rate"`
	SignupToPurchaseRate  float64  `json:"signup_to_purchase_rate"`
	CostPerSignupCents    *int64   `json:"cost_per_signup_cents,omitempty"`
	CostPerPurchaserCents *int64   `json:"cost_per_purchaser_cents,omitempty"`
	ROAS                  *float64 `json:"roas,omitempty"`
}

// GrowthPoint is one series bucket.
type GrowthPoint struct {
	Bucket       time.Time `json:"bucket"`
	Clicks       int64     `json:"clicks"`
	UniqueClicks int64     `json:"unique_clicks"`
	Signups      int64     `json:"signups"`
	Purchasers   int64     `json:"purchasers"`
	RevenueCents int64     `json:"revenue_cents"`
}

// LinkFunnel is one link's funnel.
type LinkFunnel struct {
	LinkID       uuid.UUID `json:"link_id"`
	Code         string    `json:"code"`
	Label        string    `json:"label"`
	Clicks       int64     `json:"clicks"`
	UniqueClicks int64     `json:"unique_clicks"`
	Signups      int64     `json:"signups"`
	Purchasers   int64     `json:"purchasers"`
	RevenueCents int64     `json:"revenue_cents"`
}

// SourceFunnel is one source/medium funnel.
type SourceFunnel struct {
	Source       string `json:"source"`
	Medium       string `json:"medium"`
	Clicks       int64  `json:"clicks"`
	Signups      int64  `json:"signups"`
	Purchasers   int64  `json:"purchasers"`
	RevenueCents int64  `json:"revenue_cents"`
}

// GrowthReport is a campaign, link or platform acquisition report.
type GrowthReport struct {
	Totals    GrowthTotals          `json:"totals"`
	Series    []GrowthPoint         `json:"series"`
	ByLink    []LinkFunnel          `json:"by_link"`
	BySource  []SourceFunnel        `json:"by_source"`
	Devices   []repository.KeyCount `json:"devices"`
	Referrers []repository.KeyCount `json:"referrers"`
}

// CampaignFunnel is one campaign's funnel in the acquisition overview.
type CampaignFunnel struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Clicks       int64     `json:"clicks"`
	Signups      int64     `json:"signups"`
	Purchasers   int64     `json:"purchasers"`
	RevenueCents int64     `json:"revenue_cents"`
}

// AcquisitionOverview is the platform-wide acquisition dashboard.
type AcquisitionOverview struct {
	GrowthReport
	Campaigns      []CampaignFunnel `json:"campaigns"`
	OrganicSignups int64            `json:"organic_signups"`
}

func rate(n, d int64) float64 {
	if d <= 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func (s *GrowthService) report(ctx context.Context, scope repository.GrowthScope, r Range, interval string, budget int64) (*GrowthReport, error) {
	if interval == "" {
		interval = "day"
	}
	if _, ok := map[string]bool{"hour": true, "day": true, "week": true, "month": true}[interval]; !ok {
		return nil, apperror.Validation(map[string]string{"interval": "Use hour, day, week or month."})
	}
	if interval == "hour" && r.To.Sub(r.From) > 14*24*time.Hour {
		return nil, apperror.Validation(map[string]string{"interval": "Hourly series cover at most 14 days."})
	}
	t, err := s.repo.Totals(ctx, scope, r.From, r.To, s.cfg.Currency)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rep := &GrowthReport{Totals: GrowthTotals{Clicks: t.Clicks, UniqueClicks: t.UniqueClicks, BotClicks: t.BotClicks, Signups: t.Signups,
		Purchasers: t.Purchasers, Purchases: t.Purchases, RevenueCents: t.RevenueCents, Currency: s.cfg.Currency,
		ClickToSignupRate: rate(t.Signups, t.UniqueClicks), ClickToPurchaseRate: rate(t.Purchasers, t.UniqueClicks),
		SignupToPurchaseRate: rate(t.Purchasers, t.Signups)}}
	if budget > 0 {
		if t.Signups > 0 {
			v := budget / t.Signups
			rep.Totals.CostPerSignupCents = &v
		}
		if t.Purchasers > 0 {
			v := budget / t.Purchasers
			rep.Totals.CostPerPurchaserCents = &v
		}
		roas := float64(t.RevenueCents) / float64(budget)
		rep.Totals.ROAS = &roas
	}
	series, err := s.repo.Series(ctx, scope, r.From, r.To, interval, s.cfg.Currency)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rep.Series = make([]GrowthPoint, 0, len(series))
	for _, b := range series {
		rep.Series = append(rep.Series, GrowthPoint{Bucket: b.Bucket, Clicks: b.Clicks, UniqueClicks: b.UniqueClicks, Signups: b.Signups,
			Purchasers: b.Purchasers, RevenueCents: b.RevenueCents})
	}
	links, err := s.repo.ByLink(ctx, scope, r.From, r.To, s.cfg.Currency, 100)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rep.ByLink = make([]LinkFunnel, 0, len(links))
	for _, l := range links {
		rep.ByLink = append(rep.ByLink, LinkFunnel(l))
	}
	sources, err := s.repo.BySource(ctx, scope, r.From, r.To, s.cfg.Currency, 50)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rep.BySource = make([]SourceFunnel, 0, len(sources))
	for _, v := range sources {
		rep.BySource = append(rep.BySource, SourceFunnel(v))
	}
	if rep.Devices, err = s.repo.Devices(ctx, scope, r.From, r.To); err != nil {
		return nil, apperror.Internal(err)
	}
	if rep.Referrers, err = s.repo.Referrers(ctx, scope, r.From, r.To, 10); err != nil {
		return nil, apperror.Internal(err)
	}
	if rep.Devices == nil {
		rep.Devices = []repository.KeyCount{}
	}
	if rep.Referrers == nil {
		rep.Referrers = []repository.KeyCount{}
	}
	return rep, nil
}

// CampaignReport is a campaign's acquisition funnel.
func (s *GrowthService) CampaignReport(ctx context.Context, id uuid.UUID, r Range, interval string) (*GrowthReport, error) {
	c, err := s.repo.GetCampaign(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Campaign")
	}
	budget := int64(0)
	if c.Currency == s.cfg.Currency {
		budget = c.BudgetCents
	}
	return s.report(ctx, repository.GrowthScope{CampaignID: &id}, r, interval, budget)
}

// LinkReport is a link's acquisition funnel.
func (s *GrowthService) LinkReport(ctx context.Context, id uuid.UUID, r Range, interval string) (*GrowthReport, error) {
	if _, err := s.repo.GetLink(ctx, id); err != nil {
		return nil, mapRepoErr(err, "Link")
	}
	return s.report(ctx, repository.GrowthScope{LinkID: &id}, r, interval, 0)
}

// Acquisition is the platform-wide acquisition overview.
func (s *GrowthService) Acquisition(ctx context.Context, r Range, interval string) (*AcquisitionOverview, error) {
	rep, err := s.report(ctx, repository.GrowthScope{}, r, interval, 0)
	if err != nil {
		return nil, err
	}
	out := &AcquisitionOverview{GrowthReport: *rep, Campaigns: []CampaignFunnel{}}
	counts, err := s.repo.ByCampaign(ctx, r.From, r.To, s.cfg.Currency)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	campaigns, err := s.repo.ListCampaigns(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byID := map[uuid.UUID]domain.Campaign{}
	for _, c := range campaigns {
		byID[c.ID] = c
	}
	for _, c := range counts {
		meta, ok := byID[c.CampaignID]
		if !ok || (c.Clicks == 0 && c.Signups == 0 && c.Purchasers == 0) {
			continue
		}
		out.Campaigns = append(out.Campaigns, CampaignFunnel{ID: c.CampaignID, Name: meta.Name, Slug: meta.Slug, Clicks: c.Clicks,
			Signups: c.Signups, Purchasers: c.Purchasers, RevenueCents: c.RevenueCents})
	}
	if out.OrganicSignups, err = s.repo.OrganicSignups(ctx, r.From, r.To); err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}
