package i18n

// Copy of the advisor settings module (contact details, photo review and the
// custom offering): API messages. Merged at start-up like channels.go.

var frAdvisor = map[string]string{
	// Resources.
	"Custom offering":  "Offre personnalisée",
	"Photo":            "Photo",
	"Photo submission": "Photo soumise",
	// Settings.
	"Enter a phone number in international format, e.g. +14155552671.": "Saisissez un numéro de téléphone au format international, par ex. +14155552671.",
	"Phone numbers cannot be saved right now.":                         "Les numéros de téléphone ne peuvent pas être enregistrés pour le moment.",
	// Custom offering.
	"Use a price between 10 and 50,000 credits.":                        "Utilisez un prix compris entre 10 et 50 000 crédits.",
	"Use between 1 and 60 days.":                                        "Utilisez entre 1 et 60 jours.",
	"Your offering was changed at the same time. Reload and try again.": "Votre offre a été modifiée au même moment. Rechargez la page et réessayez.",
	// Photo review.
	"Use a photo at least 256 pixels wide and 256 pixels high.":          "Utilisez une photo d'au moins 256 pixels de large et 256 pixels de haut.",
	"Another photo was uploaded at the same time. Reload and try again.": "Une autre photo a été envoyée au même moment. Rechargez la page et réessayez.",
	"This photo has already been reviewed.":                              "Cette photo a déjà été examinée.",
	"This photo is no longer available.":                                 "Cette photo n'est plus disponible.",
	"A reason (up to 500 characters) is required.":                       "Un motif (500 caractères maximum) est obligatoire.",
	"Choose pending, approved or rejected.":                              "Choisissez en attente, approuvée ou refusée.",
}

var esAdvisor = map[string]string{
	// Resources.
	"Custom offering":  "Oferta personalizada",
	"Photo":            "Foto",
	"Photo submission": "Foto enviada",
	// Settings.
	"Enter a phone number in international format, e.g. +14155552671.": "Introduce un número de teléfono en formato internacional, p. ej. +14155552671.",
	"Phone numbers cannot be saved right now.":                         "Ahora mismo no se pueden guardar números de teléfono.",
	// Custom offering.
	"Use a price between 10 and 50,000 credits.":                        "Usa un precio entre 10 y 50.000 créditos.",
	"Use between 1 and 60 days.":                                        "Usa entre 1 y 60 días.",
	"Your offering was changed at the same time. Reload and try again.": "Tu oferta se modificó al mismo tiempo. Recarga la página y vuelve a intentarlo.",
	// Photo review.
	"Use a photo at least 256 pixels wide and 256 pixels high.":          "Usa una foto de al menos 256 píxeles de ancho y 256 píxeles de alto.",
	"Another photo was uploaded at the same time. Reload and try again.": "Se subió otra foto al mismo tiempo. Recarga la página y vuelve a intentarlo.",
	"This photo has already been reviewed.":                              "Esta foto ya ha sido revisada.",
	"This photo is no longer available.":                                 "Esta foto ya no está disponible.",
	"A reason (up to 500 characters) is required.":                       "Es obligatorio indicar un motivo (500 caracteres como máximo).",
	"Choose pending, approved or rejected.":                              "Elige pendiente, aprobada o rechazada.",
}

// Merge the advisor catalogues into the main ones, then rebuild the %s/%d
// templates (package variables are initialised before init functions run).
func init() {
	for k, v := range frAdvisor {
		fr[k] = v
	}
	for k, v := range esAdvisor {
		es[k] = v
	}
	templates = buildTemplates()
}
