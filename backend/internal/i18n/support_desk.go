package i18n

// revisions 3.0: support desk (section 16). Server messages, e-mail templates
// and push copy of the ticketing system. Merged at start-up like channels.

var frSupportDesk = map[string]string{
	// revisions 3.1: conversation reports
	"Read the transcript in the console (step-up, audited). Do not forward this message outside the team.": "Lisez la transcription dans la console (step-up, journalisée). Ne transférez pas ce message en dehors de l'équipe.",
	"Choose what went wrong.":                                                "Choisissez ce qui s'est mal passé.",
	"Describe the problem in 10 to 2,000 characters.":                        "Décrivez le problème en 10 à 2 000 caractères.",
	"Use at most 2,000 characters.":                                          "Utilisez 2 000 caractères au maximum.",
	"This session is not part of the conversation.":                          "Cette séance ne fait pas partie de la conversation.",
	"Only clients can report a conversation.":                                "Seuls les clients peuvent signaler une conversation.",
	"You already reported this conversation.":                                "Vous avez déjà signalé cette conversation.",
	"Choose contact or chat report.":                                         "Choisissez contact ou signalement de conversation.",
	"Support request":                                                        "Demande d'assistance",
	"Choose an active administrator.":                                        "Choisissez un administrateur actif.",
	"Invalid administrator.":                                                 "Administrateur invalide.",
	"Choose low, normal, high or urgent.":                                    "Choisissez faible, normale, élevée ou urgente.",
	"Choose newest, oldest or waiting.":                                      "Choisissez les plus récentes, les plus anciennes ou en attente.",
	"Choose open, pending, resolved or closed.":                              "Choisissez ouverte, en attente, résolue ou fermée.",
	"Choose public or internal.":                                             "Choisissez public ou interne.",
	"Use me, unassigned or an administrator's identifier.":                   "Utilisez me, unassigned ou l'identifiant d'un administrateur.",
	"Use at most 100 characters.":                                            "Utilisez 100 caractères au maximum.",
	"Write a message between 1 and 5,000 characters.":                        "Écrivez un message de 1 à 5 000 caractères.",
	"This request is closed. Please send a new request to contact us again.": "Cette demande est fermée. Envoyez une nouvelle demande pour nous contacter à nouveau.",
	"This request has reached the maximum number of messages.":               "Cette demande a atteint le nombre maximal de messages.",
	"This request was changed by someone else. Reload it and try again.":     "Cette demande a été modifiée par quelqu'un d'autre. Rechargez-la et réessayez.",
	"Closed automatically after 7 days without a reply.":                     "Fermée automatiquement après 7 jours sans réponse.",
	"support requests":                                                       "demandes d'assistance",
	"Use the button below to follow your request and answer our team. Keep this link private: anyone who has it can read your request.": "Utilisez le bouton ci-dessous pour suivre votre demande et répondre à notre équipe. Gardez ce lien privé : toute personne qui le possède peut lire votre demande.",
	"View your request":                                               "Voir votre demande",
	"Our support team replied to your request %s:":                    "Notre équipe d'assistance a répondu à votre demande %s :",
	"New reply to your request %s":                                    "Nouvelle réponse à votre demande %s",
	"Our team replied to your request":                                "Notre équipe a répondu à votre demande",
	"Use the button below to read the whole conversation and answer.": "Utilisez le bouton ci-dessous pour lire toute la conversation et répondre.",
	"Replies to this email are not read. Keep the link private: anyone who has it can read your request.": "Les réponses à cet e-mail ne sont pas lues. Gardez le lien privé : toute personne qui le possède peut lire votre demande.",
	"Our support team replied to your request %s.":                                                        "Notre équipe d'assistance a répondu à votre demande %s.",
	"Our support team replied to your request.":                                                           "Notre équipe d'assistance a répondu à votre demande.",
	"Read the reply":             "Lire la réponse",
	"The request stays open.":    "La demande reste ouverte.",
	"The request was re-opened.": "La demande a été rouverte.",
	"Reply from the support console. Do not forward this message outside the team.": "Répondez depuis la console d'assistance. Ne transférez pas ce message en dehors de l'équipe.",
	"Open in the console": "Ouvrir dans la console",
}

var esSupportDesk = map[string]string{
	// revisions 3.1: conversation reports
	"Read the transcript in the console (step-up, audited). Do not forward this message outside the team.": "Lee la transcripción en la consola (step-up, auditada). No reenvíes este mensaje fuera del equipo.",
	"Choose what went wrong.":                                                "Elige qué salió mal.",
	"Describe the problem in 10 to 2,000 characters.":                        "Describe el problema en 10 a 2.000 caracteres.",
	"Use at most 2,000 characters.":                                          "Usa 2.000 caracteres como máximo.",
	"This session is not part of the conversation.":                          "Esta sesión no forma parte de la conversación.",
	"Only clients can report a conversation.":                                "Solo los clientes pueden denunciar una conversación.",
	"You already reported this conversation.":                                "Ya has denunciado esta conversación.",
	"Choose contact or chat report.":                                         "Elige contacto o denuncia de conversación.",
	"Support request":                                                        "Solicitud de soporte",
	"Choose an active administrator.":                                        "Elige un administrador activo.",
	"Invalid administrator.":                                                 "Administrador no válido.",
	"Choose low, normal, high or urgent.":                                    "Elige baja, normal, alta o urgente.",
	"Choose newest, oldest or waiting.":                                      "Elige las más recientes, las más antiguas o en espera.",
	"Choose open, pending, resolved or closed.":                              "Elige abierta, pendiente, resuelta o cerrada.",
	"Choose public or internal.":                                             "Elige pública o interna.",
	"Use me, unassigned or an administrator's identifier.":                   "Usa me, unassigned o el identificador de un administrador.",
	"Use at most 100 characters.":                                            "Usa 100 caracteres como máximo.",
	"Write a message between 1 and 5,000 characters.":                        "Escribe un mensaje de 1 a 5.000 caracteres.",
	"This request is closed. Please send a new request to contact us again.": "Esta solicitud está cerrada. Envía una nueva solicitud para volver a contactarnos.",
	"This request has reached the maximum number of messages.":               "Esta solicitud ha alcanzado el número máximo de mensajes.",
	"This request was changed by someone else. Reload it and try again.":     "Otra persona ha modificado esta solicitud. Vuelve a cargarla e inténtalo de nuevo.",
	"Closed automatically after 7 days without a reply.":                     "Cerrada automáticamente tras 7 días sin respuesta.",
	"support requests":                                                       "solicitudes de soporte",
	"Use the button below to follow your request and answer our team. Keep this link private: anyone who has it can read your request.": "Usa el botón de abajo para seguir tu solicitud y responder a nuestro equipo. Mantén este enlace en privado: cualquiera que lo tenga puede leer tu solicitud.",
	"View your request":                                               "Ver tu solicitud",
	"Our support team replied to your request %s:":                    "Nuestro equipo de soporte ha respondido a tu solicitud %s:",
	"New reply to your request %s":                                    "Nueva respuesta a tu solicitud %s",
	"Our team replied to your request":                                "Nuestro equipo ha respondido a tu solicitud",
	"Use the button below to read the whole conversation and answer.": "Usa el botón de abajo para leer toda la conversación y responder.",
	"Replies to this email are not read. Keep the link private: anyone who has it can read your request.": "Las respuestas a este correo no se leen. Mantén el enlace en privado: cualquiera que lo tenga puede leer tu solicitud.",
	"Our support team replied to your request %s.":                                                        "Nuestro equipo de soporte ha respondido a tu solicitud %s.",
	"Our support team replied to your request.":                                                           "Nuestro equipo de soporte ha respondido a tu solicitud.",
	"Read the reply":             "Leer la respuesta",
	"The request stays open.":    "La solicitud sigue abierta.",
	"The request was re-opened.": "La solicitud se ha reabierto.",
	"Reply from the support console. Do not forward this message outside the team.": "Responde desde la consola de soporte. No reenvíes este mensaje fuera del equipo.",
	"Open in the console": "Abrir en la consola",
}

func init() {
	for k, v := range frSupportDesk {
		fr[k] = v
	}
	for k, v := range esSupportDesk {
		es[k] = v
	}
	templates = buildTemplates()
}
