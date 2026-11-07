package i18n

// Copy of the site configuration module (revisions 3.0: promo banners,
// sign-up offer, free mode): API messages. Merged at start-up like
// advisor.go. Admin-facing, but still localized: the console follows the
// administrator's language.

var frSite = map[string]string{
	// Resources.
	"Banner":  "Bannière",
	"Setting": "Réglage",
	// Banner copy.
	"Enter the banner text in English.":               "Saisissez le texte de la bannière en anglais.",
	"Use between 1 and 140 characters.":               "Utilisez entre 1 et 140 caractères.",
	"Use at most 40 characters.":                      "Utilisez 40 caractères au maximum.",
	"Use plain text without HTML tags.":               "Utilisez du texte brut, sans balises HTML.",
	"The only placeholder allowed is {free_minutes}.": "Le seul champ variable autorisé est {free_minutes}.",
	"Unsupported language. Use en, fr or es.":         "Langue non prise en charge. Utilisez en, fr ou es.",
	// Banner action and audience.
	"Choose link or none.":              "Choisissez « link » (lien) ou « none » (aucun).",
	"Enter a destination for the link.": "Indiquez la destination du lien.",
	"Use a page path such as /signup, or an https address on an approved site.": "Utilisez un chemin de page comme /signup, ou une adresse https d'un site autorisé.",
	"Leave out the language prefix; it is added automatically.":                 "Retirez le préfixe de langue : il est ajouté automatiquement.",
	"Choose all, anonymous or signed_in.":                                       "Choisissez « all » (tout le monde), « anonymous » (visiteurs) ou « signed_in » (membres connectés).",
	// Schedule, limits and order.
	"The end must be after the start.":                            "La fin doit être postérieure au début.",
	"You can have at most 10 banners. Delete one to add another.": "Vous pouvez avoir 10 bannières au maximum. Supprimez-en une pour en ajouter une autre.",
	"Send every banner exactly once.":                             "Envoyez chaque bannière une seule fois, sans en oublier.",
	"Use between 4 and 30 seconds.":                               "Utilisez une durée comprise entre 4 et 30 secondes.",
	// Sign-up offer.
	"Use between 1 and 60 minutes.": "Utilisez une durée comprise entre 1 et 60 minutes.",
	// Free mode.
	"Type ENABLE FREE MODE exactly to confirm.":         "Saisissez exactement ENABLE FREE MODE pour confirmer.",
	"Give a reason between 5 and 500 characters.":       "Indiquez un motif de 5 à 500 caractères.",
	"Use at most 500 characters.":                       "Utilisez 500 caractères au maximum.",
	"Choose an end time in the future, within 90 days.": "Choisissez une date de fin à venir, dans les 90 jours.",
}

var esSite = map[string]string{
	// Resources.
	"Banner":  "Anuncio",
	"Setting": "Ajuste",
	// Banner copy.
	"Enter the banner text in English.":               "Escribe el texto del anuncio en inglés.",
	"Use between 1 and 140 characters.":               "Usa entre 1 y 140 caracteres.",
	"Use at most 40 characters.":                      "Usa 40 caracteres como máximo.",
	"Use plain text without HTML tags.":               "Usa texto sin formato, sin etiquetas HTML.",
	"The only placeholder allowed is {free_minutes}.": "El único marcador permitido es {free_minutes}.",
	"Unsupported language. Use en, fr or es.":         "Idioma no admitido. Usa en, fr o es.",
	// Banner action and audience.
	"Choose link or none.":              "Elige «link» (enlace) o «none» (ninguno).",
	"Enter a destination for the link.": "Indica a dónde lleva el enlace.",
	"Use a page path such as /signup, or an https address on an approved site.": "Usa una ruta de página como /signup, o una dirección https de un sitio autorizado.",
	"Leave out the language prefix; it is added automatically.":                 "Quita el prefijo de idioma: se añade automáticamente.",
	"Choose all, anonymous or signed_in.":                                       "Elige «all» (todos), «anonymous» (visitantes) o «signed_in» (miembros con sesión iniciada).",
	// Schedule, limits and order.
	"The end must be after the start.":                            "El final debe ser posterior al inicio.",
	"You can have at most 10 banners. Delete one to add another.": "Puedes tener 10 anuncios como máximo. Elimina uno para añadir otro.",
	"Send every banner exactly once.":                             "Envía cada anuncio una sola vez, sin dejar ninguno fuera.",
	"Use between 4 and 30 seconds.":                               "Usa un tiempo de entre 4 y 30 segundos.",
	// Sign-up offer.
	"Use between 1 and 60 minutes.": "Usa un tiempo de entre 1 y 60 minutos.",
	// Free mode.
	"Type ENABLE FREE MODE exactly to confirm.":         "Escribe exactamente ENABLE FREE MODE para confirmar.",
	"Give a reason between 5 and 500 characters.":       "Indica un motivo de entre 5 y 500 caracteres.",
	"Use at most 500 characters.":                       "Usa 500 caracteres como máximo.",
	"Choose an end time in the future, within 90 days.": "Elige una fecha de fin futura, dentro de los próximos 90 días.",
}

// Merge the site catalogues into the main ones, then rebuild the %s/%d
// templates (package variables are initialised before init functions run).
func init() {
	for k, v := range frSite {
		fr[k] = v
	}
	for k, v := range esSite {
		es[k] = v
	}
	templates = buildTemplates()
}
