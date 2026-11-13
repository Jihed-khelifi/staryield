package mail

import (
	"fmt"
	"strings"
	"time"
)

// revisions 5.1: confirmation e-mails (§25.5): purchase receipt, refunds and
// memberships. They share a few building blocks: a "statement descriptor"
// sentence, the policy links and receipt-style details.

// policyLinks are the secondary links of receipts (locale-less app paths that
// Render makes absolute).
func policyLinks(t func(string) string) []Action {
	return []Action{
		{Label: t("Refund & Cancellation Policy"), URL: "/refund-policy"},
		{Label: t("Terms of Use"), URL: "/terms"},
	}
}

// descriptorSentence says how the charge appears on a card statement; empty
// when no descriptor is configured.
func descriptorSentence(t func(string) string, data map[string]any) string {
	d := truncateRunes(oneLine(str(data["statement_descriptor"])), 40)
	if d == "" {
		return ""
	}
	return t(fmt.Sprintf("This charge appears on your card statement as %s.", d))
}

// textOf renders ids and other string-like values stored in notification data
// (uuid.UUID values keep their type until the row is read back from the database).
func textOf(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case fmt.Stringer:
		return x.String()
	}
	return ""
}

// timeOf reads a time stored as time.Time (in-process) or as an RFC 3339 string
// (after a database round trip).
func timeOf(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, !x.IsZero()
	case *time.Time:
		if x != nil {
			return *x, !x.IsZero()
		}
	case string:
		if tm, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(x)); err == nil {
			return tm, true
		}
	}
	return time.Time{}, false
}

func dateText(t time.Time) string     { return t.UTC().Format("2006-01-02") }
func dateTimeText(t time.Time) string { return t.UTC().Format("2006-01-02 15:04") + " UTC" }

// cardText is "Visa •••• 4242" (brand and last four digits, never more).
func cardText(data map[string]any) string {
	last4 := oneLine(str(data["card_last4"]))
	if len(last4) != 4 {
		return ""
	}
	brand := strings.ToLower(oneLine(str(data["card_brand"])))
	switch brand {
	case "visa":
		brand = "Visa"
	case "mastercard":
		brand = "Mastercard"
	case "amex":
		brand = "American Express"
	case "discover":
		brand = "Discover"
	case "":
		brand = ""
	default:
		brand = strings.ToUpper(brand[:1]) + brand[1:]
	}
	if brand == "" {
		return "•••• " + last4
	}
	return brand + " •••• " + last4
}

// purchaseReceipt is the purchase.succeeded e-mail (§25.5).
func purchaseReceipt(t func(string) string, data map[string]any, link string) Content {
	credits := intOf(data["credits"])
	c := Content{
		Subject: t(fmt.Sprintf("Your receipt: %d credits", credits)),
		Heading: t("Thank you for your purchase"),
		Paragraphs: []string{
			t(fmt.Sprintf("%d credits were added to your balance.", credits)),
			t("One-time payment. No subscription or automatic top-up."),
		},
		Links: policyLinks(t),
	}
	if p := descriptorSentence(t, data); p != "" {
		c.Paragraphs = append(c.Paragraphs, p)
	}
	if name := truncateRunes(oneLine(str(data["pack_name"])), 80); name != "" {
		c.Details = append(c.Details, Detail{Label: t("Credit pack"), Value: name})
	}
	c.Details = append(c.Details, Detail{Label: t("Credits"), Value: fmt.Sprintf("%d", credits)})
	if _, ok := data["price_cents"]; ok {
		c.Details = append(c.Details, Detail{Label: t("Amount paid"), Value: moneyText(intOf(data["price_cents"]), str(data["currency"]))})
	}
	if at, ok := timeOf(data["completed_at"]); ok {
		c.Details = append(c.Details, Detail{Label: t("Date and time (UTC)"), Value: dateTimeText(at)})
	}
	if ref := truncateRunes(oneLine(textOf(data["purchase_id"])), 40); ref != "" {
		c.Details = append(c.Details, Detail{Label: t("Reference"), Value: ref})
	}
	if card := cardText(data); card != "" {
		c.Details = append(c.Details, Detail{Label: t("Card"), Value: card})
	}
	if link != "" {
		c.Action = &Action{Label: t("View your balance"), URL: link}
	}
	return c
}

// refundDetails are the facts of a refund e-mail: the amount refunded and the
// original purchase (pack, credits, date).
func refundDetails(t func(string) string, data map[string]any, cents int64) []Detail {
	out := []Detail{{Label: t("Amount refunded"), Value: moneyText(cents, str(data["currency"]))}}
	name := truncateRunes(oneLine(str(data["pack_name"])), 80)
	if name == "" {
		name = truncateRunes(oneLine(str(data["description"])), 80)
	}
	if name != "" {
		purchase := name
		if n := intOf(data["credits"]); n > 0 {
			purchase = fmt.Sprintf("%s · %d %s", name, n, t("credits"))
		}
		out = append(out, Detail{Label: t("Original purchase"), Value: purchase})
	}
	if at, ok := timeOf(data["purchased_at"]); ok {
		out = append(out, Detail{Label: t("Purchase date (UTC)"), Value: dateText(at)})
	}
	return out
}

// bankPostingSentence says how the refund appears on the card statement.
func bankPostingSentence(t func(string) string, data map[string]any) string {
	d := truncateRunes(oneLine(str(data["statement_descriptor"])), 40)
	if d == "" {
		return t("Your bank may take 5–10 business days to show it.")
	}
	return t(fmt.Sprintf("It appears as a credit from %s; your bank may take 5–10 business days to show it.", d))
}

// refundIssued is the payment.refunded e-mail: a refund issued from the admin
// console or reported by the provider.
func refundIssued(t func(string) string, data map[string]any, link string) Content {
	cents := intOf(data["amount_cents"])
	c := Content{
		Subject: t("Your refund was issued"),
		Heading: t("Your refund was issued"),
		Paragraphs: []string{
			t(fmt.Sprintf("We refunded %s to your original payment method.", moneyText(cents, str(data["currency"])))),
			bankPostingSentence(t, data),
		},
		Details: refundDetails(t, data, cents),
		Links:   policyLinks(t),
	}
	if link != "" {
		c.Action = &Action{Label: t("View your billing"), URL: link}
	}
	return c
}

// membershipConfirmation is subscription.started (first period paid) and
// subscription.renewed: plan, amount, billing period, next renewal, the cancel
// link and the descriptor. They are only sent while memberships are on.
func membershipConfirmation(locale string, t func(string) string, started bool, data map[string]any, link string) Content {
	plan := truncateRunes(oneLine(localizedPair(locale, data["plan_names"])), 80)
	if plan == "" {
		plan = truncateRunes(oneLine(str(data["plan_name"])), 80)
	}
	c := Content{Links: policyLinks(t)}
	switch {
	case started && plan != "":
		c.Subject, c.Heading = t("Your membership has started"), t("Your membership has started")
		c.Paragraphs = append(c.Paragraphs, t(fmt.Sprintf("Your %s membership has started. Thank you for joining us.", plan)))
	case started:
		c.Subject, c.Heading = t("Your membership has started"), t("Your membership has started")
		c.Paragraphs = append(c.Paragraphs, t("Your membership has started. Thank you for joining us."))
	case plan != "":
		c.Subject, c.Heading = t("Your membership was renewed"), t("Your membership was renewed")
		c.Paragraphs = append(c.Paragraphs, t(fmt.Sprintf("Your %s membership was renewed. Thank you for staying with us.", plan)))
	default:
		c.Subject, c.Heading = t("Your membership was renewed"), t("Your membership was renewed")
		c.Paragraphs = append(c.Paragraphs, t("Your membership was renewed. Thank you for staying with us."))
	}
	if _, ok := data["amount_cents"]; ok {
		money := moneyText(intOf(data["amount_cents"]), str(data["currency"]))
		c.Paragraphs = append(c.Paragraphs, t(fmt.Sprintf("You were charged %s.", money)))
		c.Details = append(c.Details, Detail{Label: t("Amount charged"), Value: money})
	}
	switch months := intOf(data["period_months"]); {
	case months == 1:
		c.Paragraphs = append(c.Paragraphs, t("It renews automatically every month until you cancel."))
	case months > 1:
		c.Paragraphs = append(c.Paragraphs, t(fmt.Sprintf("It renews automatically every %d months until you cancel.", months)))
	}
	if p := descriptorSentence(t, data); p != "" {
		c.Paragraphs = append(c.Paragraphs, p)
	}
	if plan != "" {
		c.Details = append([]Detail{{Label: t("Membership plan"), Value: plan}}, c.Details...)
	}
	start, okStart := timeOf(data["period_start"])
	end, okEnd := timeOf(data["period_end"])
	if okStart && okEnd {
		c.Details = append(c.Details, Detail{Label: t("Billing period"), Value: dateText(start) + " – " + dateText(end)})
	}
	if next, ok := timeOf(data["next_renewal_at"]); ok {
		c.Details = append(c.Details, Detail{Label: t("Next renewal date"), Value: dateText(next)})
	} else if okEnd {
		c.Details = append(c.Details, Detail{Label: t("Next renewal date"), Value: dateText(end)})
	}
	c.Note = t("You can cancel online at any time from Settings, in the Billing section.")
	if link != "" {
		c.Action = &Action{Label: t("Cancel membership"), URL: link}
	}
	return c
}
