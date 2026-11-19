package i18n

// Copy of the Stripe card payments (revisions 6.0, §26.5): server messages of
// billing. Merged at start-up like refunds.go.

var frStripe = map[string]string{
	"Choose credits, subscription or method.":                             "Choisissez credits, subscription ou method.",
	"The payment processor did not answer. Please try again in a moment.": "Le prestataire de paiement n'a pas répondu. Veuillez réessayer dans un instant.",
}

var esStripe = map[string]string{
	"Choose credits, subscription or method.":                             "Elige credits, subscription o method.",
	"The payment processor did not answer. Please try again in a moment.": "El procesador de pagos no respondió. Inténtalo de nuevo en un momento.",
}

// Merge the Stripe catalogues into the main ones, then rebuild the %s/%d templates.
func init() {
	for k, v := range frStripe {
		fr[k] = v
	}
	for k, v := range esStripe {
		es[k] = v
	}
	templates = buildTemplates()
}
