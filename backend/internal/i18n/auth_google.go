package i18n

// Copy of Sign in with Google (revisions 4.0, §23.1): API messages and the
// security e-mails. Merged at start-up like identity.go.

var frAuthGoogle = map[string]string{
	// Errors.
	"Google sign-in is not available right now. Please try again.":          "La connexion avec Google n'est pas disponible pour le moment. Veuillez réessayer.",
	"This sign-in attempt expired or was already used. Please start again.": "Cette tentative de connexion a expiré ou a déjà été utilisée. Veuillez recommencer.",
	"Google sign-in was cancelled.":                                         "La connexion avec Google a été annulée.",
	"Google reports that this email address is not verified.":               "Google indique que cette adresse e-mail n'est pas vérifiée.",
	"This Google account is already connected to a Staryield account.":      "Ce compte Google est déjà connecté à un compte Staryield.",
	"Google sign-in is not available for this account.":                     "La connexion avec Google n'est pas disponible pour ce compte.",
	"This action needs a password, and your account does not have one yet.": "Cette action nécessite un mot de passe, et votre compte n'en a pas encore.",
	"Create a password before disconnecting Google.":                        "Créez un mot de passe avant de déconnecter Google.",
	// Validation and resources.
	"Choose signin, signup or link.": "Choisissez signin, signup ou link.",
	"Sign-in method":                 "Méthode de connexion",
	"Pending sign-in":                "Connexion en attente",
	// E-mails.
	"Google sign-in was added to your account":                                                         "La connexion avec Google a été ajoutée à votre compte",
	"You can now sign in to Staryield with Google.":                                                    "Vous pouvez désormais vous connecter à Staryield avec Google.",
	"You can now sign in to your Staryield account with the Google account %s.":                        "Vous pouvez désormais vous connecter à votre compte Staryield avec le compte Google %s.",
	"Your password, if you have one, still works.":                                                     "Votre mot de passe, si vous en avez un, fonctionne toujours.",
	"Review your sign-in methods":                                                                      "Vérifier vos méthodes de connexion",
	"If you did not make this change, contact our support team immediately and choose a new password.": "Si vous n'êtes pas à l'origine de ce changement, contactez immédiatement notre équipe d'assistance et choisissez un nouveau mot de passe.",
	"Google sign-in was removed from your account":                                                     "La connexion avec Google a été retirée de votre compte",
	"Google can no longer be used to sign in to Staryield.":                                            "Google ne peut plus être utilisé pour se connecter à Staryield.",
	"Google can no longer be used to sign in to your Staryield account.":                               "Google ne peut plus être utilisé pour vous connecter à votre compte Staryield.",
	"You can still sign in with your password.":                                                        "Vous pouvez toujours vous connecter avec votre mot de passe.",
}

var esAuthGoogle = map[string]string{
	// Errors.
	"Google sign-in is not available right now. Please try again.":          "El inicio de sesión con Google no está disponible en este momento. Inténtalo de nuevo.",
	"This sign-in attempt expired or was already used. Please start again.": "Este intento de inicio de sesión ha caducado o ya se ha utilizado. Vuelve a empezar.",
	"Google sign-in was cancelled.":                                         "Se canceló el inicio de sesión con Google.",
	"Google reports that this email address is not verified.":               "Google indica que esta dirección de correo no está verificada.",
	"This Google account is already connected to a Staryield account.":      "Esta cuenta de Google ya está conectada a una cuenta de Staryield.",
	"Google sign-in is not available for this account.":                     "El inicio de sesión con Google no está disponible para esta cuenta.",
	"This action needs a password, and your account does not have one yet.": "Esta acción requiere una contraseña, y tu cuenta todavía no tiene ninguna.",
	"Create a password before disconnecting Google.":                        "Crea una contraseña antes de desconectar Google.",
	// Validation and resources.
	"Choose signin, signup or link.": "Elige signin, signup o link.",
	"Sign-in method":                 "Método de inicio de sesión",
	"Pending sign-in":                "Inicio de sesión pendiente",
	// E-mails.
	"Google sign-in was added to your account":                                                         "Se añadió el inicio de sesión con Google a tu cuenta",
	"You can now sign in to Staryield with Google.":                                                    "Ya puedes iniciar sesión en Staryield con Google.",
	"You can now sign in to your Staryield account with the Google account %s.":                        "Ya puedes iniciar sesión en tu cuenta de Staryield con la cuenta de Google %s.",
	"Your password, if you have one, still works.":                                                     "Tu contraseña, si tienes una, sigue funcionando.",
	"Review your sign-in methods":                                                                      "Revisar tus métodos de inicio de sesión",
	"If you did not make this change, contact our support team immediately and choose a new password.": "Si no hiciste este cambio, contacta de inmediato con nuestro equipo de soporte y elige una nueva contraseña.",
	"Google sign-in was removed from your account":                                                     "Se quitó el inicio de sesión con Google de tu cuenta",
	"Google can no longer be used to sign in to Staryield.":                                            "Ya no se puede usar Google para iniciar sesión en Staryield.",
	"Google can no longer be used to sign in to your Staryield account.":                               "Ya no se puede usar Google para iniciar sesión en tu cuenta de Staryield.",
	"You can still sign in with your password.":                                                        "Todavía puedes iniciar sesión con tu contraseña.",
}

func init() {
	for k, v := range frAuthGoogle {
		fr[k] = v
	}
	for k, v := range esAuthGoogle {
		es[k] = v
	}
	templates = buildTemplates()
}
