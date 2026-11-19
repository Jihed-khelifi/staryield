package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// revisions 6.0 (§26.4): the calculators' e-mail report.
//
// A visitor asks for the report they see on screen to be e-mailed. The server
// recalculates it from the calculator's own request (nothing computed by the
// browser is trusted), renders it into the e-mail outbox and records the
// address as a lead. Birth details are never stored: the outbox row carries
// the rendered e-mail sealed and is wiped once the message is sent (the outbox
// does that for every row), and the lead keeps only the address, locale,
// country, calculators asked for, counters and the consent evidence.

const (
	// CalculatorReportConsentVersion is the version date of the consent text next
	// to the marketing box ("Send me horoscopes, offers and news from Staryield
	// by e-mail. You can unsubscribe at any time."), stored as consent evidence.
	CalculatorReportConsentVersion = "2026-11-19"
	// ReportEmailTemplate names the outbox rows of the report.
	ReportEmailTemplate = "calculator_report"

	// reportInputMax bounds the calculator input (the largest valid one is
	// under 200 bytes).
	reportInputMax = 2048
	// reportRuleAddressName is the per-address rate-limit bucket (the per-IP
	// bucket, report_email, is the router's).
	reportRuleAddressName = "report_email_address"
)

// ReportAddressRule is the per-address budget of the report (3 per day).
var ReportAddressRule = ratelimit.Rule{Name: reportRuleAddressName, Rate: 3, Period: 24 * time.Hour, Burst: 3}

// ReportEmailInput is POST /calculators/report-email.
type ReportEmailInput struct {
	Kind string `json:"kind"`
	// Input is the calculator's own request body (natal chart, sun sign or
	// numerology), validated by the calculator's own rules.
	Input               json.RawMessage `json:"input"`
	Email               string          `json:"email"`
	Locale              string          `json:"locale"`
	MarketingOptIn      bool            `json:"marketing_opt_in"`
	MarketingPrechecked bool            `json:"marketing_prechecked"`
	// Website is the honeypot: real visitors never see the field.
	Website string `json:"website"`
	// Country is the trusted edge country of the request (set by the handler,
	// never read from the body).
	Country string `json:"-"`
}

// ReportEmailConfig is GET /calculators/report-email/config.
type ReportEmailConfig struct {
	Enabled bool `json:"enabled"`
	// MarketingPrechecked: the marketing box starts ticked for the request's
	// country (a pre-ticked box is only lawful in some countries).
	MarketingPrechecked bool `json:"marketing_prechecked"`
}

// ReportSettings are the report service's configuration values.
type ReportSettings struct {
	FrontendURL  string
	IPHashSecret string
	// UnsubscribeKey keys the HMAC of the unsubscribe tokens.
	UnsubscribeKey string
	// AddressRule is the per-address budget (ReportAddressRule scaled for tests).
	AddressRule ratelimit.Rule
}

// reportMail is the part of the e-mail outbox the report uses.
type reportMail interface {
	EmailQueue
	AddressHash(email string) string
	Suppressed(ctx context.Context, hash string) (bool, error)
}

// ReportDeps are the report service's collaborators.
type ReportDeps struct {
	Tx      database.TxManager
	Leads   repository.EmailLeadRepository
	Users   repository.UserRepository
	Astro   *AstrologyService
	Mail    reportMail
	Sealer  *security.Sealer
	Audit   repository.AuditRepository
	Limiter ratelimit.Limiter
	Tracker analytics.Tracker
	Clock   clock.Clock
	// Settings returns the cached switch and pre-ticked countries
	// (SiteService.CalculatorReport; unreadable ⇒ disabled).
	Settings func(ctx context.Context) CalculatorReportSettings
	Cfg      ReportSettings
}

// CalculatorReportService e-mails the calculators' reports and manages the
// e-mail leads they create.
type CalculatorReportService struct{ d ReportDeps }

// NewCalculatorReportService wires the service.
func NewCalculatorReportService(d ReportDeps) *CalculatorReportService {
	if d.Cfg.AddressRule.Rate <= 0 {
		d.Cfg.AddressRule = ReportAddressRule
	}
	return &CalculatorReportService{d: d}
}

// Suppressed reports whether an address is on the suppression list (bounces,
// complaints).
func (s *MailService) Suppressed(ctx context.Context, hash string) (bool, error) {
	m, err := s.repo.Suppressed(ctx, []string{hash}, s.clock.Now())
	return m[hash], err
}

// Config answers the dialog's config for a request from country.
func (s *CalculatorReportService) Config(ctx context.Context, country string) ReportEmailConfig {
	set := s.d.Settings(ctx)
	return ReportEmailConfig{Enabled: set.Enabled, MarketingPrechecked: set.Enabled && set.Prechecked(country)}
}

// Request validates a report request, recalculates the report, records the
// lead and queues the e-mail. A nil error is the 202: the honeypot and
// suppressed addresses get it too (nothing is stored or sent for them).
func (s *CalculatorReportService) Request(ctx context.Context, in ReportEmailInput) error {
	if strings.TrimSpace(in.Website) != "" {
		s.track(ctx, "calculator.report_blocked", map[string]any{"reason": "honeypot"})
		return nil
	}
	settings := s.d.Settings(ctx)
	if !settings.Enabled {
		return apperror.FeatureUnavailable("The e-mail report is not available right now.")
	}

	fields := map[string]string{}
	kind := domain.ReportKind(strings.TrimSpace(in.Kind))
	if !kind.Valid() {
		fields["kind"] = "Use natal-chart, path-of-life or astrology."
	}
	email, emailOK := NormalizeEmail(in.Email)
	if !emailOK {
		fields["email"] = "Enter a valid email address."
	}
	locale := strings.ToLower(strings.TrimSpace(in.Locale))
	if locale == "" {
		locale = i18n.Normalize(reqctx.MetaFrom(ctx).Locale)
	}
	if locale != "en" && locale != "fr" && locale != "es" {
		fields["locale"] = "Use en, fr or es."
	}
	var built *builtReport
	switch trimmed := bytes.TrimSpace(in.Input); {
	case len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")):
		fields["input"] = "Send the details of the calculation."
	case len(trimmed) > reportInputMax:
		fields["input"] = "The details of the calculation are too long."
	case kind.Valid():
		var err error
		if built, err = s.build(kind, trimmed); err != nil {
			var ae *apperror.Error
			if !errors.As(err, &ae) || ae.Code != apperror.CodeValidation {
				return err
			}
			for k, v := range reportValidationFields(ae) {
				if k != "input" { // the calculator's field names are prefixed: input.birth_date
					k = "input." + k
				}
				fields[k] = v
			}
		}
	}
	if len(fields) > 0 {
		return apperror.Validation(fields)
	}

	hash := s.d.Mail.AddressHash(email)
	suppressed, err := s.d.Mail.Suppressed(ctx, hash)
	if err != nil {
		return apperror.Internal(fmt.Errorf("report: suppression lookup: %w", err))
	}
	if suppressed {
		s.track(ctx, "calculator.report_blocked", map[string]any{"reason": "suppressed"})
		return nil
	}
	if err := s.allowAddress(ctx, hash); err != nil {
		return err
	}

	userID := s.signedInUser(ctx, hash)
	var lead *domain.EmailLead
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if lead, err = s.touchLead(ctx, in, email, hash, locale, kind, userID, settings); err != nil {
			return err
		}
		content, err := s.render(built, locale, lead)
		if err != nil {
			return err
		}
		// The outbox row is sealed and wiped once sent; a failure here rolls
		// the lead's counters back with it. The category is "account": a report
		// the visitor asked for is transactional (always sent), and no account
		// is attached so the per-user daily cap never applies.
		return s.d.Mail.Enqueue(ctx, EmailRequest{To: email, Locale: locale, Category: domain.EmailAccount,
			Template: ReportEmailTemplate, Content: content})
	})
	if err != nil {
		return mapRepoErr(err, "Report")
	}
	s.track(ctx, "calculator.report_requested", map[string]any{"kind": string(kind), "locale": locale,
		"subscribed": lead.MarketingStatus == domain.MarketingSubscribed, "signed_in": userID != nil})
	return nil
}

// reportValidationFields reads the field messages of a validation error.
func reportValidationFields(e *apperror.Error) map[string]string {
	out := map[string]string{}
	fields, _ := e.Details["fields"].(map[string]any)
	for k, v := range fields {
		if msg, ok := v.(string); ok {
			out[k] = msg
		}
	}
	return out
}

// allowAddress spends one of the address's daily reports.
func (s *CalculatorReportService) allowAddress(ctx context.Context, hash string) error {
	if s.d.Limiter == nil {
		return nil
	}
	res, err := s.d.Limiter.Allow(ctx, "report_email:address:"+hash, s.d.Cfg.AddressRule)
	if err != nil {
		logger.FromContext(ctx).Error("report: rate limiter unavailable", zap.Error(err))
		return apperror.Unavailable(err)
	}
	if !res.Allowed {
		return apperror.RateLimited(res.RetryAfter)
	}
	return nil
}

// signedInUser returns the signed-in account when it is the one that uses the
// address (the dialog prefills the account's e-mail); nil otherwise.
func (s *CalculatorReportService) signedInUser(ctx context.Context, hash string) *uuid.UUID {
	p, ok := reqctx.PrincipalFrom(ctx)
	if !ok || s.d.Users == nil {
		return nil
	}
	u, err := s.d.Users.GetByID(ctx, p.UserID)
	if err != nil {
		warnErr(ctx, "report: load the signed-in account", err)
		return nil
	}
	if s.d.Mail.AddressHash(u.Email) != hash {
		return nil
	}
	id := u.ID
	return &id
}

func leadAD(id uuid.UUID) string { return "email_lead:" + id.String() }

// touchLead creates or updates the address's lead inside tx: counters, kinds,
// locale, country, the account link and the marketing consent.
func (s *CalculatorReportService) touchLead(ctx context.Context, in ReportEmailInput, email, hash, locale string,
	kind domain.ReportKind, userID *uuid.UUID, settings CalculatorReportSettings) (*domain.EmailLead, error) {
	now := s.d.Clock.Now()
	for attempt := 0; attempt < 2; attempt++ {
		lead, err := s.d.Leads.ForUpdateByHash(ctx, hash)
		created := false
		switch {
		case errors.Is(err, repository.ErrNotFound):
			id := domain.NewID()
			sealed, serr := s.d.Sealer.Seal(email, leadAD(id))
			if serr != nil {
				return nil, serr
			}
			lead = &domain.EmailLead{ID: id, EmailHash: hash, EmailSealed: sealed, Source: string(kind), Kinds: domain.StringList{},
				MarketingStatus: domain.MarketingNone, Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now}}
			created = true
		case err != nil:
			return nil, err
		}
		lead.Locale, lead.LastRequestAt = locale, now
		if in.Country != "" {
			lead.Country = in.Country
		}
		if !lead.HasKind(kind) {
			lead.Kinds = append(lead.Kinds, string(kind))
		}
		lead.ReportsSent++
		if userID != nil {
			lead.UserID = userID
		}
		s.applyOptIn(ctx, lead, in, settings, now)
		if !created {
			return lead, s.d.Leads.Save(ctx, lead)
		}
		switch err := s.d.Leads.Create(ctx, lead); {
		case err == nil:
			return lead, nil
		case errors.Is(err, repository.ErrLeadExists) && attempt == 0:
			// A concurrent request created the lead first: update it instead.
			continue
		default:
			return nil, err
		}
	}
	return nil, errors.New("report: lead upsert did not settle")
}

// applyOptIn records the marketing consent of this request on the lead.
//
//   - Unticking the box later never unsubscribes: only the unsubscribe link does.
//   - A box that started ticked is only valid consent where the setting says
//     so (the visitor's country): elsewhere the request still gets its report
//     but no subscription is recorded.
//   - An address that unsubscribed is only subscribed again by a box the
//     visitor ticked themselves, never by one that started ticked.
func (s *CalculatorReportService) applyOptIn(ctx context.Context, lead *domain.EmailLead, in ReportEmailInput,
	settings CalculatorReportSettings, now time.Time) {
	if !in.MarketingOptIn || lead.MarketingStatus == domain.MarketingSubscribed {
		return
	}
	if in.MarketingPrechecked && (!settings.Prechecked(in.Country) || lead.MarketingStatus == domain.MarketingUnsubscribed) {
		return
	}
	meta := reqctx.MetaFrom(ctx)
	version, prechecked := CalculatorReportConsentVersion, in.MarketingPrechecked
	lead.MarketingStatus, lead.UnsubscribedAt = domain.MarketingSubscribed, nil
	lead.ConsentAt, lead.ConsentPrechecked, lead.ConsentTextVersion = &now, &prechecked, &version
	lead.ConsentIPHash, lead.ConsentUserAgent = nil, nil
	if meta.IP != "" {
		h := securityKeyed(s.d.Cfg.IPHashSecret, meta.IP)
		lead.ConsentIPHash = &h
	}
	if ua := truncateRunes(CleanLine(meta.UserAgent), consentUserAgentMax); ua != "" {
		lead.ConsentUserAgent = &ua
	}
}

func (s *CalculatorReportService) track(ctx context.Context, name string, props map[string]any) {
	if s.d.Tracker != nil {
		s.d.Tracker.Track(ctx, name, props)
	}
}

// --- unsubscribe -----------------------------------------------------------------

// UnsubscribeToken is the opaque token of a lead's unsubscribe link: the lead
// id and its HMAC, both base64url.
func (s *CalculatorReportService) UnsubscribeToken(id uuid.UUID) string {
	return unsubscribeToken(s.d.Cfg.UnsubscribeKey, id)
}

func unsubscribeToken(key string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(id[:]) + "." + base64.RawURLEncoding.EncodeToString(unsubscribeMAC(key, id))
}

func unsubscribeMAC(key string, id uuid.UUID) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("marketing-unsubscribe:" + id.String()))
	return mac.Sum(nil)
}

// parseUnsubscribeToken returns the lead id of a valid token (constant-time
// comparison of the HMAC).
func parseUnsubscribeToken(key, token string) (uuid.UUID, bool) {
	idPart, macPart, found := strings.Cut(strings.TrimSpace(token), ".")
	if !found || len(token) > 128 {
		return uuid.Nil, false
	}
	// Strict: a token has one spelling (the decoder would otherwise ignore stray trailing bits).
	b64 := base64.RawURLEncoding.Strict()
	raw, err := b64.DecodeString(idPart)
	if err != nil || len(raw) != len(uuid.UUID{}) {
		return uuid.Nil, false
	}
	var id uuid.UUID
	copy(id[:], raw)
	got, err := b64.DecodeString(macPart)
	if err != nil || !hmac.Equal(got, unsubscribeMAC(key, id)) {
		return uuid.Nil, false
	}
	return id, true
}

var errUnsubscribeLink = apperror.NotFound("Unsubscribe link")

// Unsubscribe applies an unsubscribe link. It is idempotent; an invalid token,
// or a lead that no longer exists, is a 404.
func (s *CalculatorReportService) Unsubscribe(ctx context.Context, token string) error {
	id, ok := parseUnsubscribeToken(s.d.Cfg.UnsubscribeKey, token)
	if !ok {
		return errUnsubscribeLink
	}
	changed := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		lead, err := s.d.Leads.ForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if lead.MarketingStatus == domain.MarketingUnsubscribed {
			return nil
		}
		now := s.d.Clock.Now()
		lead.MarketingStatus, lead.UnsubscribedAt = domain.MarketingUnsubscribed, &now
		changed = true
		return s.d.Leads.Save(ctx, lead)
	})
	if errors.Is(err, repository.ErrNotFound) {
		return errUnsubscribeLink
	}
	if err != nil {
		return mapRepoErr(err, "Unsubscribe link")
	}
	if changed {
		s.track(ctx, "marketing.unsubscribed", map[string]any{"source": "link"})
	}
	return nil
}

// --- registration and erasure ----------------------------------------------------

// LinkRegistration links the lead of a new account's address to it (called
// after the account is created; failures are logged, never returned).
func (s *CalculatorReportService) LinkRegistration(ctx context.Context, u *domain.User) {
	if u == nil || s.d.Leads == nil {
		return
	}
	if _, err := s.d.Leads.LinkUser(ctx, s.d.Mail.AddressHash(u.Email), u.ID); err != nil {
		warnErr(ctx, "report: link the e-mail lead to the new account", err)
	}
}

// PurgeForUser deletes the leads linked to an erased account (UserDataPurger).
func (s *CalculatorReportService) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.d.Leads.PurgeForUser(ctx, userID)
}

var _ UserDataPurger = (*CalculatorReportService)(nil)

// PurgeAll combines user-data purgers into one (account erasure runs the
// calculator analytics purger and the e-mail lead purger through one slot).
func PurgeAll(purgers ...UserDataPurger) UserDataPurger { return multiPurger(purgers) }

type multiPurger []UserDataPurger

func (m multiPurger) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var total int64
	for _, p := range m {
		n, err := p.PurgeForUser(ctx, userID)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
