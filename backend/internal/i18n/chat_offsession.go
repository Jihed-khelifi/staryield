package i18n

// Copy of messages outside a live chat (revisions 3.3, §22.3).

var frOffSession = map[string]string{
	"Messages outside a live chat are turned off right now.":                                 "Les messages en dehors d'un chat en direct sont désactivés pour le moment.",
	"Start a live chat first. You can keep writing here afterwards.":                         "Lancez d'abord un chat en direct. Vous pourrez ensuite continuer à écrire ici.",
	"This person can't receive messages right now.":                                          "Cette personne ne peut pas recevoir de messages pour le moment.",
	"You have used today's messages outside a live chat. Start a live chat to keep talking.": "Vous avez utilisé vos messages du jour en dehors d'un chat en direct. Lancez un chat en direct pour continuer à discuter.",
	"Please try again.":                     "Veuillez réessayer.",
	"Use a whole number between 0 and 500.": "Utilisez un nombre entier entre 0 et 500.",
}

var esOffSession = map[string]string{
	"Messages outside a live chat are turned off right now.":                                 "Los mensajes fuera de un chat en vivo están desactivados por ahora.",
	"Start a live chat first. You can keep writing here afterwards.":                         "Inicia primero un chat en vivo. Después podrás seguir escribiendo aquí.",
	"This person can't receive messages right now.":                                          "Esta persona no puede recibir mensajes por ahora.",
	"You have used today's messages outside a live chat. Start a live chat to keep talking.": "Has usado los mensajes de hoy fuera de un chat en vivo. Inicia un chat en vivo para seguir hablando.",
	"Please try again.":                     "Inténtalo de nuevo.",
	"Use a whole number between 0 and 500.": "Usa un número entero entre 0 y 500.",
}

func init() {
	for k, v := range frOffSession {
		fr[k] = v
	}
	for k, v := range esOffSession {
		es[k] = v
	}
	templates = buildTemplates()
}
