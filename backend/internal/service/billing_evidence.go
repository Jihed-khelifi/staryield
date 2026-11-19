package service

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): dispute evidence. The draft is composed from data the
// platform already keeps: the pack, the account, the purchase time, the
// consent record, the usage log (when, with whom, how long, what it cost: never
// what was said), the refund-policy disclosure and the invoice. An
// administrator reviews and submits it; submitting is final.

// DisputeEvidenceFields is the evidence form of a dispute.
type DisputeEvidenceFields struct {
	ProductDescription     string `json:"product_description"`
	CustomerName           string `json:"customer_name"`
	CustomerEmail          string `json:"customer_email"`
	BillingCountry         string `json:"billing_country"`
	ServiceDate            string `json:"service_date"`
	AccessActivityLog      string `json:"access_activity_log"`
	RefundPolicyDisclosure string `json:"refund_policy_disclosure"`
	CancellationRebuttal   string `json:"cancellation_rebuttal"`
	UncategorizedText      string `json:"uncategorized_text"`
	AttachInvoice          bool   `json:"attach_invoice"`
}

// Evidence limits (the card networks' text fields hold 20,000 characters).
const (
	evidenceTextMax = 20000
	evidenceNameMax = 200
	// evidenceUsageRows bounds the usage log of the draft.
	evidenceUsageRows = 50
)

// evidenceInput is everything the draft is composed from.
type evidenceInput struct {
	Kind            domain.TransactionKind
	ProductName     string // the pack or plan
	Credits         int64
	AmountCents     int64
	Currency        string
	PurchasedAt     time.Time
	CustomerName    string
	CustomerEmail   string
	CardCountry     string
	ThreeDS         string
	AccountCreated  time.Time
	Consent         *PurchaseConsentView
	Usage           []repository.UsageRow
	RefundPolicyURL string
	HasInvoice      bool
}

const evidenceStamp = "2006-01-02 15:04 UTC"

// composeEvidenceDraft builds the draft. It never contains message text: the
// usage log lists readings by date, advisor, duration and cost only.
func composeEvidenceDraft(in evidenceInput) DisputeEvidenceFields {
	purchased := in.PurchasedAt.UTC()
	amount := money(in.AmountCents, in.Currency)
	f := DisputeEvidenceFields{CustomerName: in.CustomerName, CustomerEmail: in.CustomerEmail, BillingCountry: in.CardCountry,
		ServiceDate: purchased.Format("2006-01-02"), AttachInvoice: in.HasInvoice}

	if in.Kind == domain.TxCreditPurchase {
		f.ProductDescription = fmt.Sprintf("%s: %d digital credits for live psychic chat, for entertainment purposes only. "+
			"The credits are added to the customer's Staryield account as soon as the payment succeeds and are spent on live chat readings, by the minute.",
			in.ProductName, in.Credits)
		f.CancellationRebuttal = "This was a one-time purchase of digital credits. It is not a subscription and there is nothing to cancel."
	} else {
		f.ProductDescription = fmt.Sprintf("%s: Staryield Premium Membership (monthly access to member benefits on a live psychic chat platform, "+
			"for entertainment purposes only).", in.ProductName)
		f.CancellationRebuttal = "The membership renews monthly until the customer cancels it from the account settings; the customer accepted the terms before subscribing."
	}

	var log strings.Builder
	fmt.Fprintf(&log, "Account: %s, created %s UTC.\n", in.CustomerEmail, in.AccountCreated.UTC().Format("2006-01-02"))
	fmt.Fprintf(&log, "Payment of %s on %s.\n", amount, purchased.Format(evidenceStamp))
	if len(in.Usage) == 0 {
		log.WriteString("No reading has been started since the purchase.")
	} else {
		log.WriteString("Readings started after the purchase (date | advisor | minutes | credits):\n")
		for _, u := range in.Usage {
			minutes := u.BilledMinutes + u.FreeMinutes
			fmt.Fprintf(&log, "%s | %s | %d | %d\n", u.StartedAt.UTC().Format(evidenceStamp), u.Advisor, minutes, u.CreditsCharged)
		}
		if len(in.Usage) >= evidenceUsageRows {
			fmt.Fprintf(&log, "(the first %d readings are listed)", evidenceUsageRows)
		}
	}
	f.AccessActivityLog = strings.TrimSpace(log.String())

	if c := in.Consent; c != nil {
		f.RefundPolicyDisclosure = fmt.Sprintf("Before paying, the customer ticked the box accepting the Terms of Use (version %s), the Privacy Policy (version %s) "+
			"and the Refund & Cancellation Policy (version %s) on %s. The policy is published at %s and linked next to the payment button.",
			c.Terms, c.Privacy, c.Refund, c.At.UTC().Format(evidenceStamp), in.RefundPolicyURL)
	} else {
		f.RefundPolicyDisclosure = fmt.Sprintf("The Refund & Cancellation Policy is published at %s and linked next to the payment button.", in.RefundPolicyURL)
	}

	summary := fmt.Sprintf("%s (%s) paid %s on %s.", in.CustomerName, in.CustomerEmail, amount, purchased.Format(evidenceStamp))
	if in.CardCountry != "" {
		summary += " Card issued in " + in.CardCountry + "."
	}
	if in.ThreeDS != "" {
		summary += " 3-D Secure: " + strings.ReplaceAll(in.ThreeDS, "_", " ") + "."
	}
	f.UncategorizedText = summary
	return f
}

// validateEvidence checks the fields an administrator submits and returns them
// cleaned. A submission (final) must at least describe the product.
func validateEvidence(in DisputeEvidenceFields, submit bool) (DisputeEvidenceFields, error) {
	fields := map[string]string{}
	long := func(name, value string) string {
		v := CleanText(value)
		if utf8.RuneCountInString(v) > evidenceTextMax {
			fields[name] = fmt.Sprintf("Use at most %d characters.", evidenceTextMax)
		}
		return v
	}
	out := DisputeEvidenceFields{
		ProductDescription:     long("product_description", in.ProductDescription),
		AccessActivityLog:      long("access_activity_log", in.AccessActivityLog),
		RefundPolicyDisclosure: long("refund_policy_disclosure", in.RefundPolicyDisclosure),
		CancellationRebuttal:   long("cancellation_rebuttal", in.CancellationRebuttal),
		UncategorizedText:      long("uncategorized_text", in.UncategorizedText),
		AttachInvoice:          in.AttachInvoice,
	}
	if out.CustomerName = CleanLine(in.CustomerName); utf8.RuneCountInString(out.CustomerName) > evidenceNameMax {
		fields["customer_name"] = "Use at most 200 characters."
	}
	if e := strings.TrimSpace(in.CustomerEmail); e != "" {
		if n, ok := NormalizeEmail(e); ok {
			out.CustomerEmail = n
		} else {
			fields["customer_email"] = "Enter a valid email address."
		}
	}
	if c := strings.ToUpper(strings.TrimSpace(in.BillingCountry)); c != "" {
		if countryCode.MatchString(c) {
			out.BillingCountry = c
		} else {
			fields["billing_country"] = "Use a two-letter country code."
		}
	}
	if d := strings.TrimSpace(in.ServiceDate); d != "" {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			fields["service_date"] = "Use the format YYYY-MM-DD."
		} else {
			out.ServiceDate = d
		}
	}
	if submit && out.ProductDescription == "" {
		fields["product_description"] = "Describe the product to submit the evidence."
	}
	if len(fields) > 0 {
		return DisputeEvidenceFields{}, apperror.Validation(fields)
	}
	return out, nil
}

// providerEvidence maps the validated form to what the provider receives.
func providerEvidence(f DisputeEvidenceFields, pdf []byte, filename string) payment.DisputeEvidence {
	return payment.DisputeEvidence{ProductDescription: f.ProductDescription, CustomerName: f.CustomerName, CustomerEmail: f.CustomerEmail,
		BillingCountry: f.BillingCountry, ServiceDate: f.ServiceDate, AccessActivityLog: f.AccessActivityLog,
		RefundPolicyDisclosure: f.RefundPolicyDisclosure, CancellationRebuttal: f.CancellationRebuttal,
		UncategorizedText: f.UncategorizedText, ReceiptPDF: pdf, ReceiptFilename: filename}
}
