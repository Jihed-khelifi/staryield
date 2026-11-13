package mail

import (
	"html"
	"strings"
	"testing"
)

// revisions 5.1: confirmation e-mails (§25.5) and the company footer (§25.4).

const testSupport = "help@staryield.example"

func renderKind(t *testing.T, locale, kind string) Rendered {
	t.Helper()
	data, ok := samples[kind]
	if !ok {
		t.Fatalf("no sample for %s", kind)
	}
	c, ok := Notification(locale, kind, data, "https://staryield.net/"+locale+"/profile/settings#billing")
	if !ok {
		t.Fatalf("%s has no e-mail", kind)
	}
	r, err := Render(c, Layout{Locale: locale, FrontendURL: "https://staryield.net", Category: "billing", SupportEmail: testSupport})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// facts must all appear in both bodies (HTML text is escaped: & becomes &amp;, ' becomes &#39;).
func requireFacts(t *testing.T, label string, r Rendered, facts ...string) {
	t.Helper()
	for _, f := range facts {
		if !strings.Contains(r.Text, f) {
			t.Errorf("%s: text body lacks %q", label, f)
		}
		if !strings.Contains(r.HTML, html.EscapeString(f)) {
			t.Errorf("%s: HTML body lacks %q", label, f)
		}
	}
}

var hoursByLocale = map[string]string{
	"en": "Support is available 24/7. We reply within 1–2 days.",
	"fr": "Le support est disponible 24 h/24 et 7 j/7. Nous répondons sous 1 à 2 jours.",
	"es": "El soporte está disponible 24/7. Respondemos en 1–2 días.",
}

// Every e-mail (not only confirmations) carries the company footer.
func TestEveryEmailHasTheCompanyFooter(t *testing.T) {
	for _, locale := range []string{"en", "fr", "es"} {
		for kind := range samples {
			if kind == "broadcast" {
				continue
			}
			r := renderKind(t, locale, kind)
			requireFacts(t, locale+"/"+kind, r, "Staryield Media LLC", "30 N Gould St, Ste N, Sheridan, WY 82801, United States", testSupport, hoursByLocale[locale])
			if !strings.Contains(r.HTML, `href="mailto:`+testSupport+`"`) {
				t.Errorf("%s/%s: the support address is not a mailto link", locale, kind)
			}
		}
		// Account e-mails sent outside the notification catalogue too.
		r, err := Render(PasswordReset(locale, "https://staryield.net/x", 30), Layout{Locale: locale, FrontendURL: "https://staryield.net", Category: "security"})
		if err != nil {
			t.Fatal(err)
		}
		requireFacts(t, locale+"/password_reset", r, "Staryield Media LLC", DefaultSupportEmail, hoursByLocale[locale])
	}
}

func TestPurchaseReceiptCarriesTheRequiredFacts(t *testing.T) {
	policy := map[string]string{"en": "Refund & Cancellation Policy", "fr": "Politique de remboursement et d'annulation", "es": "Política de reembolso y cancelación"}
	oneTime := map[string]string{"en": "One-time payment. No subscription or automatic top-up.",
		"fr": "Paiement unique. Aucun abonnement ni rechargement automatique.", "es": "Pago único. Sin suscripción ni recarga automática."}
	statement := map[string]string{"en": "This charge appears on your card statement as STARYIELD MEDIA LLC.",
		"fr": "Ce débit apparaît sur votre relevé de carte sous le nom STARYIELD MEDIA LLC.", "es": "Este cargo aparece en el extracto de tu tarjeta como STARYIELD MEDIA LLC."}
	subject := map[string]string{"en": "Your receipt: 600 credits", "fr": "Votre reçu : 600 crédits", "es": "Tu recibo: 600 créditos"}
	for _, locale := range []string{"en", "fr", "es"} {
		r := renderKind(t, locale, "purchase.succeeded")
		if r.Subject != subject[locale] {
			t.Errorf("%s subject: %q", locale, r.Subject)
		}
		requireFacts(t, locale, r, "19.99 USD", "Starter", "600", "2026-11-12 10:15 UTC", "0b6d7c1e-0000-4000-8000-000000000001",
			"Visa •••• 4242", oneTime[locale], statement[locale], policy[locale],
			"https://staryield.net/"+locale+"/refund-policy", "https://staryield.net/"+locale+"/terms", testSupport)
	}
}

// A receipt built from older notification data (credits only) still renders.
func TestPurchaseReceiptWithoutDetails(t *testing.T) {
	c, ok := Notification("en", "purchase.succeeded", map[string]any{"credits": float64(50)}, "https://staryield.net/en/profile")
	if !ok || c.Subject != "Your receipt: 50 credits" || len(c.Details) != 1 { // only the credits row
		t.Fatalf("minimal receipt: %+v", c)
	}
	for _, p := range c.Paragraphs {
		if strings.Contains(p, "card statement") {
			t.Fatalf("no descriptor was given: %q", p)
		}
	}
}

func TestRefundEmailsNameAmountPurchaseAndDescriptor(t *testing.T) {
	posting := map[string]string{
		"en": "It appears as a credit from STARYIELD MEDIA LLC; your bank may take 5–10 business days to show it.",
		"fr": "Il apparaît comme un crédit de STARYIELD MEDIA LLC ; votre banque peut mettre 5 à 10 jours ouvrés pour l'afficher.",
		"es": "Aparece como un abono de STARYIELD MEDIA LLC; tu banco puede tardar de 5 a 10 días hábiles en mostrarlo.",
	}
	for _, kind := range []string{"payment.refunded", "refund_request.resolved"} {
		for _, locale := range []string{"en", "fr", "es"} {
			r := renderKind(t, locale, kind)
			requireFacts(t, locale+"/"+kind, r, "19.99 USD", "Starter", "600", posting[locale], testSupport, "Staryield Media LLC")
		}
	}
	// Without a configured descriptor the bank sentence stays honest.
	c, _ := Notification("en", "payment.refunded", map[string]any{"amount_cents": float64(500), "currency": "USD"}, "")
	if !strings.Contains(strings.Join(c.Paragraphs, " "), "Your bank may take 5–10 business days to show it.") {
		t.Fatalf("paragraphs: %v", c.Paragraphs)
	}
}

func TestMembershipEmailsCarryPlanPeriodRenewalAndCancelLink(t *testing.T) {
	cancel := map[string]string{"en": "Cancel membership", "fr": "Résilier l'abonnement", "es": "Cancelar la membresía"}
	for _, kind := range []string{"subscription.started", "subscription.renewed"} {
		for _, locale := range []string{"en", "fr", "es"} {
			r := renderKind(t, locale, kind)
			plan := "Premium"
			if kind == "subscription.started" && locale == "fr" {
				plan = "Premium FR" // the plan's French name
			}
			requireFacts(t, locale+"/"+kind, r, plan, "9.99 USD", "2026-11-12 – 2026-12-12", "2026-12-12", "STARYIELD MEDIA LLC",
				cancel[locale], "https://staryield.net/"+locale+"/profile/settings#billing", testSupport)
		}
	}
}
