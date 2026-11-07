package i18n

// Copy of the chat message push notification (revisions 3.0: chat unread).

var frChatUnread = map[string]string{
	"New message from %s":           "Nouveau message de %s",
	"New message":                   "Nouveau message",
	"Open the chatroom to read it.": "Ouvrez le salon de chat pour le lire.",
}

var esChatUnread = map[string]string{
	"New message from %s":           "Nuevo mensaje de %s",
	"New message":                   "Nuevo mensaje",
	"Open the chatroom to read it.": "Abre la sala de chat para leerlo.",
}

func init() {
	for k, v := range frChatUnread {
		fr[k] = v
	}
	for k, v := range esChatUnread {
		es[k] = v
	}
	templates = buildTemplates()
}
