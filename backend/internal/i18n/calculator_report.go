package i18n

// revisions 6.0 (docs/platform-extensions.md §26.4): the calculators' e-mail
// report. Server messages of the report and unsubscribe endpoints, the
// admin's lead and setting messages, and the frame of the report e-mail (the
// result's own copy comes from the web app's generated report-copy.json).
// Merged into the main catalogues at start-up like receipts.go.

var frCalculatorReport = map[string]string{
	// Endpoint messages.
	"The e-mail report is not available right now.":       "L'envoi du rapport par e-mail n'est pas disponible pour le moment.",
	"Send the details of the calculation.":                "Envoyez les informations du calcul.",
	"The details of the calculation are too long.":        "Les informations du calcul sont trop longues.",
	"Check the details of the calculation and try again.": "Vérifiez les informations du calcul et réessayez.",
	"Use none, subscribed or unsubscribed.":               "Utilisez none, subscribed ou unsubscribed.",
	"Enter the full address.":                             "Saisissez l'adresse complète.",
	"Unsubscribe link":                                    "Lien de désinscription",
	"Lead":                                                "Contact",
	"Report":                                              "Rapport",
	"Use two-letter country codes such as US.":            "Utilisez des codes pays à deux lettres, comme US.",
	"Use at most 60 countries.":                           "Utilisez 60 pays au maximum.",
	// The report e-mail.
	"Your Staryield natal chart report":              "Votre rapport de thème natal Staryield",
	"Your Staryield astrology report":                "Votre rapport d'astrologie Staryield",
	"Your Staryield path of life report":             "Votre rapport de chemin de vie Staryield",
	"The report you asked for is ready to read.":     "Le rapport que vous avez demandé est prêt à être lu.",
	"Here is the report you asked us to e-mail you.": "Voici le rapport que vous nous avez demandé de vous envoyer par e-mail.",
	"View your report":                               "Voir votre rapport",
	"The link opens your result again and contains the details you entered. Keep it private.": "Le lien rouvre votre résultat et contient les informations que vous avez saisies. Gardez-le privé.",
	"Unsubscribe from Staryield marketing e-mails":                                            "Se désinscrire des e-mails marketing de Staryield",
}

var esCalculatorReport = map[string]string{
	// Endpoint messages.
	"The e-mail report is not available right now.":       "El envío del informe por correo no está disponible por ahora.",
	"Send the details of the calculation.":                "Envía los datos del cálculo.",
	"The details of the calculation are too long.":        "Los datos del cálculo son demasiado largos.",
	"Check the details of the calculation and try again.": "Revisa los datos del cálculo e inténtalo de nuevo.",
	"Use none, subscribed or unsubscribed.":               "Usa none, subscribed o unsubscribed.",
	"Enter the full address.":                             "Introduce la dirección completa.",
	"Unsubscribe link":                                    "Enlace de baja",
	"Lead":                                                "Contacto",
	"Report":                                              "Informe",
	"Use two-letter country codes such as US.":            "Usa códigos de país de dos letras, como US.",
	"Use at most 60 countries.":                           "Usa 60 países como máximo.",
	// The report e-mail.
	"Your Staryield natal chart report":              "Tu informe de carta natal de Staryield",
	"Your Staryield astrology report":                "Tu informe de astrología de Staryield",
	"Your Staryield path of life report":             "Tu informe de camino de vida de Staryield",
	"The report you asked for is ready to read.":     "El informe que pediste está listo para leer.",
	"Here is the report you asked us to e-mail you.": "Aquí está el informe que nos pediste enviarte por correo.",
	"View your report":                               "Ver tu informe",
	"The link opens your result again and contains the details you entered. Keep it private.": "El enlace vuelve a abrir tu resultado y contiene los datos que introdujiste. Mantenlo en privado.",
	"Unsubscribe from Staryield marketing e-mails":                                            "Darte de baja de los correos de marketing de Staryield",
}

func init() {
	for k, v := range frCalculatorReport {
		fr[k] = v
	}
	for k, v := range esCalculatorReport {
		es[k] = v
	}
	templates = buildTemplates()
}
