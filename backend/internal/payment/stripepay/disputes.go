package stripepay

import (
	"bytes"
	"context"
	"strings"
	"unicode/utf8"

	"github.com/stripe/stripe-go/v87"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// maxEvidenceText is Stripe's limit for one text evidence field.
const maxEvidenceText = 20000

// SubmitDisputeEvidence implements payment.DisputeManager: the evidence text
// fields (and the invoice PDF, uploaded as the receipt) are saved on the
// dispute; submit=true sends them to the card network (final).
func (p *Provider) SubmitDisputeEvidence(ctx context.Context, disputeRef string, ev payment.DisputeEvidence, submit bool) error {
	if !strings.HasPrefix(disputeRef, "dp_") && !strings.HasPrefix(disputeRef, "du_") {
		return payment.ErrNotFound
	}
	evidence := &stripe.DisputeUpdateEvidenceParams{}
	set := func(dst **string, v string) {
		if v = strings.TrimSpace(v); v != "" {
			*dst = stripe.String(clipRunes(v, maxEvidenceText))
		}
	}
	set(&evidence.ProductDescription, ev.ProductDescription)
	set(&evidence.CustomerName, ev.CustomerName)
	set(&evidence.CustomerEmailAddress, ev.CustomerEmail)
	set(&evidence.BillingAddress, ev.BillingCountry)
	set(&evidence.ServiceDate, ev.ServiceDate)
	set(&evidence.AccessActivityLog, ev.AccessActivityLog)
	set(&evidence.RefundPolicyDisclosure, ev.RefundPolicyDisclosure)
	set(&evidence.CancellationRebuttal, ev.CancellationRebuttal)
	set(&evidence.UncategorizedText, ev.UncategorizedText)
	if len(ev.ReceiptPDF) > 0 {
		name := ev.ReceiptFilename
		if name == "" {
			name = "receipt.pdf"
		}
		f, err := p.client.V1Files.Create(ctx, &stripe.FileCreateParams{
			Purpose: stripe.String("dispute_evidence"), FileReader: bytes.NewReader(ev.ReceiptPDF), Filename: stripe.String(name)})
		if err != nil {
			return p.mapError("upload dispute receipt", err)
		}
		evidence.Receipt = stripe.String(f.ID)
	}
	params := &stripe.DisputeUpdateParams{Evidence: evidence, Submit: stripe.Bool(submit)}
	if _, err := p.client.V1Disputes.Update(ctx, disputeRef, params); err != nil {
		return p.mapError("update dispute", err)
	}
	return nil
}

// AcceptDispute implements payment.DisputeManager: closing a dispute concedes it.
func (p *Provider) AcceptDispute(ctx context.Context, disputeRef string) error {
	if !strings.HasPrefix(disputeRef, "dp_") && !strings.HasPrefix(disputeRef, "du_") {
		return payment.ErrNotFound
	}
	if _, err := p.client.V1Disputes.Close(ctx, disputeRef, nil); err != nil {
		return p.mapError("close dispute", err)
	}
	return nil
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
