package i18n

// Copy of the free-trial hold (revisions 6.0, §26.1): errors and the push
// notification of a reply the client cannot read yet.

var frChatHolds = map[string]string{
	"This message is hidden until you top up.":                            "Ce message est masqué tant que vous n'avez pas rechargé.",
	"%s sent you a message. Top up to read it.":                           "%s vous a envoyé un message. Rechargez pour le lire.",
	"Someone sent you a message. Top up to read it.":                      "Vous avez reçu un message. Rechargez pour le lire.",
	"This reading is not waiting for a top-up.":                           "Cette consultation n'attend pas de recharge.",
	"These hidden messages can only be revealed after the reading ended.": "Ces messages masqués ne peuvent être révélés qu'une fois la consultation terminée.",
	"There are no hidden messages to reveal.":                             "Il n'y a aucun message masqué à révéler.",
	"Use a whole number of seconds between 30 and 300.":                   "Utilisez un nombre entier de secondes entre 30 et 300.",
	"Use a whole number of seconds between 0 and 300.":                    "Utilisez un nombre entier de secondes entre 0 et 300.",
	"Hold not found.": "Attente introuvable.",
}

var esChatHolds = map[string]string{
	"This message is hidden until you top up.":                            "Este mensaje está oculto hasta que recargues.",
	"%s sent you a message. Top up to read it.":                           "%s te ha enviado un mensaje. Recarga para leerlo.",
	"Someone sent you a message. Top up to read it.":                      "Has recibido un mensaje. Recarga para leerlo.",
	"This reading is not waiting for a top-up.":                           "Esta lectura no está esperando una recarga.",
	"These hidden messages can only be revealed after the reading ended.": "Estos mensajes ocultos solo se pueden revelar cuando la lectura haya terminado.",
	"There are no hidden messages to reveal.":                             "No hay mensajes ocultos que revelar.",
	"Use a whole number of seconds between 30 and 300.":                   "Usa un número entero de segundos entre 30 y 300.",
	"Use a whole number of seconds between 0 and 300.":                    "Usa un número entero de segundos entre 0 y 300.",
	"Hold not found.": "Espera no encontrada.",
}

func init() {
	for k, v := range frChatHolds {
		fr[k] = v
	}
	for k, v := range esChatHolds {
		es[k] = v
	}
	templates = buildTemplates()
}
