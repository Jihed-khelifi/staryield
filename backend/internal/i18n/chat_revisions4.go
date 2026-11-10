package i18n

// Copy of the replies, likes and client nickname messages (revisions 4.0,
// §23.2–23.4).

var frRevisions4 = map[string]string{
	"You can only reply to a text or image message of this conversation.": "Vous ne pouvez répondre qu'à un message texte ou image de cette conversation.",
	"System notices can't be liked.":                                      "Les notifications du système ne peuvent pas être aimées.",
	"Use a valid message id.":                                             "Utilisez un identifiant de message valide.",
	"Use 1 to 40 characters, without special characters.":                 "Utilisez de 1 à 40 caractères, sans caractères spéciaux.",
	"Send between 1 and 50 comma-separated client ids.":                   "Envoyez entre 1 et 50 identifiants de clients séparés par des virgules.",
	"Nicknames are not available yet.":                                    "Les surnoms ne sont pas encore disponibles.",
	"State why you need to see this nickname.":                            "Indiquez pourquoi vous devez voir ce surnom.",
}

var esRevisions4 = map[string]string{
	"You can only reply to a text or image message of this conversation.": "Solo puedes responder a un mensaje de texto o de imagen de esta conversación.",
	"System notices can't be liked.":                                      "Los avisos del sistema no se pueden marcar con me gusta.",
	"Use a valid message id.":                                             "Usa un identificador de mensaje válido.",
	"Use 1 to 40 characters, without special characters.":                 "Usa de 1 a 40 caracteres, sin caracteres especiales.",
	"Send between 1 and 50 comma-separated client ids.":                   "Envía entre 1 y 50 identificadores de clientes separados por comas.",
	"Nicknames are not available yet.":                                    "Los apodos aún no están disponibles.",
	"State why you need to see this nickname.":                            "Indica por qué necesitas ver este apodo.",
}

func init() {
	for k, v := range frRevisions4 {
		fr[k] = v
	}
	for k, v := range esRevisions4 {
		es[k] = v
	}
	templates = buildTemplates()
}
