package i18n

// Lifecycle messaging: the e-mail and push text of the lifecycle notification
// kinds and the messages of the admin settings. Merged into the main
// catalogues at start-up like calculator_report.go.

var frLifecycle = map[string]string{
	// Admin endpoints.
	"Choose a delay between 1 hour and 90 days.": "Choisissez un délai entre 1 heure et 90 jours.",
	"Unknown journey.":                           "Parcours inconnu.",
	"Choose between 1 and 30 days.":              "Choisissez entre 1 et 30 jours.",
	"Choose a window between 1 and 90 days.":     "Choisissez une période entre 1 et 90 jours.",
	"Change at least one setting.":               "Modifiez au moins un réglage.",
	// Messages to clients.
	"Your free minute is waiting":                                            "Votre minute offerte vous attend",
	"You have 1 free minute to use on your first reading.":                   "Vous avez 1 minute offerte à utiliser pour votre première consultation.",
	"Choose an advisor":                                                      "Choisir un conseiller",
	"Your free minutes are waiting":                                          "Vos minutes offertes vous attendent",
	"You have %d free minutes to use on your first reading.":                 "Vous avez %d minutes offertes à utiliser pour votre première consultation.",
	"Advisors are online now":                                                "Des conseillers sont en ligne",
	"Choose an advisor and start your first reading.":                        "Choisissez un conseiller et commencez votre première consultation.",
	"Your first reading is waiting":                                          "Votre première consultation vous attend",
	"Choose an advisor and start your first reading whenever you are ready.": "Choisissez un conseiller et commencez votre première consultation quand vous êtes prêt.",
	"Enjoyed your reading?":                                                  "Votre consultation vous a plu ?",
	"Invite a friend to Staryield and share the experience.":                 "Invitez un ami sur Staryield et partagez l'expérience.",
	"Invite a friend":                                                        "Inviter un ami",
	"An advisor is online now":                                               "Un conseiller est en ligne",
	"1 advisor is online now. Pick up where you left off.":                   "1 conseiller est en ligne. Reprenez là où vous vous étiez arrêté.",
	"%d advisors are online now. Pick up where you left off.":                "%d conseillers sont en ligne. Reprenez là où vous vous étiez arrêté.",
	"Your daily horoscope is ready":                                          "Votre horoscope du jour est prêt",
	"Read today's horoscope for your sign.":                                  "Lisez l'horoscope du jour pour votre signe.",
	"You receive this email because you chose to receive tips, offers and reminders from Staryield.": "Vous recevez cet e-mail parce que vous avez choisi de recevoir des conseils, des offres et des rappels de Staryield.",
	"Read your horoscope": "Lire votre horoscope",
}

var esLifecycle = map[string]string{
	// Admin endpoints.
	"Choose a delay between 1 hour and 90 days.": "Elige un plazo entre 1 hora y 90 días.",
	"Unknown journey.":                           "Recorrido desconocido.",
	"Choose between 1 and 30 days.":              "Elige entre 1 y 30 días.",
	"Choose a window between 1 and 90 days.":     "Elige un periodo entre 1 y 90 días.",
	"Change at least one setting.":               "Cambia al menos un ajuste.",
	// Messages to clients.
	"Your free minute is waiting":                                            "Tu minuto gratis te espera",
	"You have 1 free minute to use on your first reading.":                   "Tienes 1 minuto gratis para usar en tu primera lectura.",
	"Choose an advisor":                                                      "Elegir un asesor",
	"Your free minutes are waiting":                                          "Tus minutos gratis te esperan",
	"You have %d free minutes to use on your first reading.":                 "Tienes %d minutos gratis para usar en tu primera lectura.",
	"Advisors are online now":                                                "Hay asesores en línea",
	"Choose an advisor and start your first reading.":                        "Elige un asesor y empieza tu primera lectura.",
	"Your first reading is waiting":                                          "Tu primera lectura te espera",
	"Choose an advisor and start your first reading whenever you are ready.": "Elige un asesor y empieza tu primera lectura cuando quieras.",
	"Enjoyed your reading?":                                                  "¿Te gustó tu lectura?",
	"Invite a friend to Staryield and share the experience.":                 "Invita a un amigo a Staryield y comparte la experiencia.",
	"Invite a friend":                                                        "Invitar a un amigo",
	"An advisor is online now":                                               "Hay un asesor en línea",
	"1 advisor is online now. Pick up where you left off.":                   "Hay 1 asesor en línea. Retoma donde lo dejaste.",
	"%d advisors are online now. Pick up where you left off.":                "Hay %d asesores en línea. Retoma donde lo dejaste.",
	"Your daily horoscope is ready":                                          "Tu horóscopo del día está listo",
	"Read today's horoscope for your sign.":                                  "Lee el horóscopo de hoy para tu signo.",
	"You receive this email because you chose to receive tips, offers and reminders from Staryield.": "Recibes este correo porque elegiste recibir consejos, ofertas y recordatorios de Staryield.",
	"Read your horoscope": "Leer tu horóscopo",
}

func init() {
	for k, v := range frLifecycle {
		fr[k] = v
	}
	for k, v := range esLifecycle {
		es[k] = v
	}
	templates = buildTemplates()
}
