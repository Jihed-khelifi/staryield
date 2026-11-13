package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 5.1: consent evidence on every credit purchase (§25.2).
//
// A buyer accepts the Terms of Use, the Privacy Policy and the Refund &
// Cancellation Policy before paying. The versions they accepted (the policies'
// YYYY-MM-DD version dates), the server time and two correlation fields (a keyed
// hash of the IP and the cleaned user agent) are stored with the purchase as
// chargeback evidence. The IP hash uses the same keyed hash and secret as
// refund requests and the audit trail (securityKeyed with the analytics secret).

// consentUserAgentMax bounds the stored user agent (runes).
const consentUserAgentMax = 300

// PurchaseConsent is the consent a purchase request carries: the version dates
// of the three documents ("YYYY-MM-DD").
type PurchaseConsent struct {
	Terms   string `json:"terms"`
	Privacy string `json:"privacy"`
	Refund  string `json:"refund"`
}

// PurchaseConsentView is the consent record of a purchase (admin console).
type PurchaseConsentView struct {
	Terms   string    `json:"terms"`
	Privacy string    `json:"privacy"`
	Refund  string    `json:"refund"`
	At      time.Time `json:"at"`
}

// errConsentRequired is the 422 of a missing or invalid consent.
func errConsentRequired() error {
	return apperror.Validation(map[string]string{"consent": "Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue."})
}

// consentDate parses one version date: a real calendar date in strict
// YYYY-MM-DD form that is not after today (UTC).
func consentDate(v string, now time.Time) (string, bool) {
	v = strings.TrimSpace(v)
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		return "", false
	}
	n := now.UTC()
	if d.After(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)) {
		return "", false
	}
	return v, true
}

// validatePurchaseConsent checks a purchase's consent and returns the record to
// store (version columns, server time, IP hash and user agent from the request).
func (s *CreditService) validatePurchaseConsent(ctx context.Context, c *PurchaseConsent, now time.Time) (map[string]any, error) {
	if c == nil {
		return nil, errConsentRequired()
	}
	terms, ok1 := consentDate(c.Terms, now)
	privacy, ok2 := consentDate(c.Privacy, now)
	refund, ok3 := consentDate(c.Refund, now)
	if !ok1 || !ok2 || !ok3 {
		return nil, errConsentRequired()
	}
	meta := reqctx.MetaFrom(ctx)
	rec := map[string]any{"consent_terms_version": terms, "consent_privacy_version": privacy, "consent_refund_version": refund, "consent_at": now}
	if meta.IP != "" {
		rec["consent_ip_hash"] = securityKeyed(s.settings.IPHashSecret, meta.IP)
	}
	if ua := truncateRunes(CleanLine(meta.UserAgent), consentUserAgentMax); ua != "" {
		rec["consent_user_agent"] = ua
	}
	return rec, nil
}

// applyConsent sets the consent record on a purchase about to be created.
func applyConsent(p *domain.CreditPurchase, rec map[string]any) {
	str := func(k string) *string {
		if v, ok := rec[k].(string); ok {
			return &v
		}
		return nil
	}
	p.ConsentTermsVersion, p.ConsentPrivacyVersion, p.ConsentRefundVersion = str("consent_terms_version"), str("consent_privacy_version"), str("consent_refund_version")
	p.ConsentIPHash, p.ConsentUserAgent = str("consent_ip_hash"), str("consent_user_agent")
	if at, ok := rec["consent_at"].(time.Time); ok {
		p.ConsentAt = &at
	}
}

// purchaseConsentOf is the consent record of a purchase, nil for purchases made
// before revisions 5.1 (or without one).
func purchaseConsentOf(p *domain.CreditPurchase) *PurchaseConsentView {
	if p == nil || p.ConsentAt == nil || p.ConsentTermsVersion == nil || p.ConsentPrivacyVersion == nil || p.ConsentRefundVersion == nil {
		return nil
	}
	return &PurchaseConsentView{Terms: *p.ConsentTermsVersion, Privacy: *p.ConsentPrivacyVersion, Refund: *p.ConsentRefundVersion, At: *p.ConsentAt}
}

// consentsOf loads the consent records of the credit purchases among rows in
// one query (no per-row lookups), keyed by purchase id.
func (s *BillingService) consentsOf(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]*PurchaseConsentView, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for i := range rows {
		t := &rows[i]
		if t.Kind == domain.TxCreditPurchase && t.CreditPurchaseID != nil && !seen[*t.CreditPurchaseID] {
			seen[*t.CreditPurchaseID] = true
			ids = append(ids, *t.CreditPurchaseID)
		}
	}
	out := map[uuid.UUID]*PurchaseConsentView{}
	if len(ids) == 0 || s.credits == nil {
		return out, nil
	}
	purchases, err := s.credits.PurchasesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range purchases {
		if c := purchaseConsentOf(&purchases[i]); c != nil {
			out[purchases[i].ID] = c
		}
	}
	return out, nil
}
