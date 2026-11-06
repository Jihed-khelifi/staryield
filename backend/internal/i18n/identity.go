package i18n

// Copy of the identity module (account settings, devices, e-mail change,
// consents, data exports, deactivation and closure): API messages, e-mail
// templates and the export README. Merged at start-up like channels.go.

var frIdentity = map[string]string{
	// Resources.
	"Device": "Appareil",
	"Export": "Export",
	// Settings.
	"Account settings are not available.":                              "Les paramètres du compte ne sont pas disponibles.",
	"Choose English, French or Spanish.":                               "Choisissez l'anglais, le français ou l'espagnol.",
	"Choose a house system from the list.":                             "Choisissez un système de maisons dans la liste.",
	"Choose a time zone from the list.":                                "Choisissez un fuseau horaire dans la liste.",
	"Your settings were changed elsewhere. Reload them and try again.": "Vos paramètres ont été modifiés ailleurs. Rechargez-les et réessayez.",
	// Devices.
	"Device management is not available.":              "La gestion des appareils n'est pas disponible.",
	"Use scope=others to sign out your other devices.": "Utilisez scope=others pour déconnecter vos autres appareils.",
	// Re-authentication and one-time links.
	"The password is incorrect.":                                                   "Le mot de passe est incorrect.",
	"This link is invalid or has expired.":                                         "Ce lien n'est pas valide ou a expiré.",
	"This is already your email address.":                                          "C'est déjà votre adresse e-mail.",
	"This email address now belongs to another account. Contact our support team.": "Cette adresse e-mail appartient désormais à un autre compte. Contactez notre équipe d'assistance.",
	// Consents.
	"Privacy preferences are not available.":  "Les préférences de confidentialité ne sont pas disponibles.",
	"Essential storage cannot be turned off.": "Le stockage essentiel ne peut pas être désactivé.",
	"Unknown consent category.":               "Catégorie de consentement inconnue.",
	"Choose at least one category.":           "Choisissez au moins une catégorie.",
	"Use settings, banner or signup.":         "Utilisez settings, banner ou signup.",
	// Exports.
	"Data exports are not available yet.":           "Les exports de données ne sont pas encore disponibles.",
	"You can request up to %d exports a day.":       "Vous pouvez demander jusqu'à %d exports par jour.",
	"Your previous export is still being prepared.": "Votre export précédent est encore en préparation.",
	"This export is not available for download.":    "Cet export n'est pas disponible au téléchargement.",
	"This download link is invalid or has expired.": "Ce lien de téléchargement n'est pas valide ou a expiré.",
	// Deactivation and closure.
	"Administrator accounts are managed by the team.":             "Les comptes administrateur sont gérés par l'équipe.",
	"This account is deactivated. Reactivate it to continue.":     "Ce compte est désactivé. Réactivez-le pour continuer.",
	"This account is scheduled for closure. Keep it to continue.": "La fermeture de ce compte est programmée. Conservez-le pour continuer.",
	"Your account cannot be closed yet.":                          "Votre compte ne peut pas encore être fermé.",
	"Type DELETE to confirm.":                                     "Saisissez DELETE pour confirmer.",
	// E-mails: e-mail change.
	"Email change requested":                                                       "Changement d'adresse e-mail demandé",
	"Someone asked to change the email address of your account.":                   "Quelqu'un a demandé à changer l'adresse e-mail de votre compte.",
	"Someone asked to change the email address of your Staryield account to %s.":   "Quelqu'un a demandé à remplacer l'adresse e-mail de votre compte Staryield par %s.",
	"If it was you, confirm the change from the email we sent to the new address.": "Si c'était vous, confirmez le changement depuis l'e-mail envoyé à la nouvelle adresse.",
	"This was not me": "Ce n'était pas moi",
	"If you did not ask for this, cancel the change and choose a new password.": "Si vous n'êtes pas à l'origine de cette demande, annulez le changement et choisissez un nouveau mot de passe.",
	"Someone tried to use your email address":                                   "Quelqu'un a essayé d'utiliser votre adresse e-mail",
	"Someone asked to use this email address for another Staryield account.":    "Quelqu'un a demandé à utiliser cette adresse e-mail pour un autre compte Staryield.",
	"It already belongs to your account, so nothing was changed.":               "Elle appartient déjà à votre compte : rien n'a été modifié.",
	"Sign in": "Se connecter",
	"If you did not make this change, restore your previous address with this link.": "Si vous n'êtes pas à l'origine de ce changement, rétablissez votre ancienne adresse avec ce lien.",
	"Restore my email address":        "Rétablir mon adresse e-mail",
	"Your email address was restored": "Votre adresse e-mail a été rétablie",
	"Your previous email address is back on your account and every device was signed out.": "Votre ancienne adresse e-mail est rétablie et tous les appareils ont été déconnectés.",
	"Choose a new password within %d minutes to secure your account:":                      "Choisissez un nouveau mot de passe dans les %d minutes pour sécuriser votre compte :",
	// E-mails: deactivation and closure.
	"Your account is deactivated": "Votre compte est désactivé",
	"Your Staryield account is paused: your profile is hidden and you were signed out everywhere.": "Votre compte Staryield est en pause : votre profil est masqué et vous avez été déconnecté partout.",
	"Sign in at any time to reactivate it.":                                                        "Connectez-vous à tout moment pour le réactiver.",
	"Your account was closed":                                                                      "Votre compte a été fermé",
	"As you asked, your Staryield account was closed and your personal data erased.":               "Comme vous l'avez demandé, votre compte Staryield a été fermé et vos données personnelles effacées.",
	"Records we must keep by law, such as payments, are kept without your name.":                   "Les éléments que la loi nous oblige à conserver, comme les paiements, sont gardés sans votre nom.",
	"Thank you for the time you spent with us.":                                                    "Merci pour le temps passé avec nous.",
	// Export README.
	"This archive contains a copy of the personal data of your Staryield account.": "Cette archive contient une copie des données personnelles de votre compte Staryield.",
	"It was generated on %s (UTC).": "Elle a été générée le %s (UTC).",
	"Each JSON file covers one part of your account; manifest.json lists every file with its checksum.": "Chaque fichier JSON couvre une partie de votre compte ; manifest.json liste chaque fichier avec son empreinte.",
	"Images you shared in chats are in the chat/attachments folder.":                                    "Les images partagées dans les chats se trouvent dans le dossier chat/attachments.",
	"Not included:": "Non inclus :",
	"Notes that advisors wrote about you: they are the advisors' working notes, not data you provided.": "Les notes que les conseillers ont écrites à votre sujet : ce sont leurs notes de travail, pas des données que vous avez fournies.",
	"Security secrets such as your password hash, sign-in tokens and encrypted payout details.":         "Les secrets de sécurité comme l'empreinte de votre mot de passe, les jetons de connexion et les coordonnées de paiement chiffrées.",
	"Only the first %d %s are included.":                                             "Seuls les %d premiers %s sont inclus.",
	"%d images were too large to include; they are listed in chat/attachments.json.": "%d images étaient trop volumineuses pour être incluses ; elles sont listées dans chat/attachments.json.",
	"credit movements": "mouvements de crédits",
	"chat messages":    "messages de chat",
	"notifications":    "notifications",
}

var esIdentity = map[string]string{
	// Resources.
	"Device": "Dispositivo",
	"Export": "Exportación",
	// Settings.
	"Account settings are not available.":                              "La configuración de la cuenta no está disponible.",
	"Choose English, French or Spanish.":                               "Elige inglés, francés o español.",
	"Choose a house system from the list.":                             "Elige un sistema de casas de la lista.",
	"Choose a time zone from the list.":                                "Elige una zona horaria de la lista.",
	"Your settings were changed elsewhere. Reload them and try again.": "Tu configuración se modificó en otro lugar. Vuelve a cargarla e inténtalo de nuevo.",
	// Devices.
	"Device management is not available.":              "La gestión de dispositivos no está disponible.",
	"Use scope=others to sign out your other devices.": "Usa scope=others para cerrar la sesión en tus otros dispositivos.",
	// Re-authentication and one-time links.
	"The password is incorrect.":                                                   "La contraseña no es correcta.",
	"This link is invalid or has expired.":                                         "Este enlace no es válido o ha caducado.",
	"This is already your email address.":                                          "Ya es tu dirección de correo electrónico.",
	"This email address now belongs to another account. Contact our support team.": "Esta dirección de correo pertenece ahora a otra cuenta. Contacta con nuestro equipo de soporte.",
	// Consents.
	"Privacy preferences are not available.":  "Las preferencias de privacidad no están disponibles.",
	"Essential storage cannot be turned off.": "El almacenamiento esencial no se puede desactivar.",
	"Unknown consent category.":               "Categoría de consentimiento desconocida.",
	"Choose at least one category.":           "Elige al menos una categoría.",
	"Use settings, banner or signup.":         "Usa settings, banner o signup.",
	// Exports.
	"Data exports are not available yet.":           "Las exportaciones de datos aún no están disponibles.",
	"You can request up to %d exports a day.":       "Puedes solicitar hasta %d exportaciones al día.",
	"Your previous export is still being prepared.": "Tu exportación anterior todavía se está preparando.",
	"This export is not available for download.":    "Esta exportación no está disponible para descargar.",
	"This download link is invalid or has expired.": "Este enlace de descarga no es válido o ha caducado.",
	// Deactivation and closure.
	"Administrator accounts are managed by the team.":             "Las cuentas de administrador las gestiona el equipo.",
	"This account is deactivated. Reactivate it to continue.":     "Esta cuenta está desactivada. Reactívala para continuar.",
	"This account is scheduled for closure. Keep it to continue.": "El cierre de esta cuenta está programado. Consérvala para continuar.",
	"Your account cannot be closed yet.":                          "Todavía no se puede cerrar tu cuenta.",
	"Type DELETE to confirm.":                                     "Escribe DELETE para confirmar.",
	// E-mails: e-mail change.
	"Email change requested":                                                       "Cambio de correo electrónico solicitado",
	"Someone asked to change the email address of your account.":                   "Alguien pidió cambiar la dirección de correo electrónico de tu cuenta.",
	"Someone asked to change the email address of your Staryield account to %s.":   "Alguien pidió cambiar la dirección de correo de tu cuenta de Staryield a %s.",
	"If it was you, confirm the change from the email we sent to the new address.": "Si fuiste tú, confirma el cambio desde el correo que enviamos a la nueva dirección.",
	"This was not me": "No fui yo",
	"If you did not ask for this, cancel the change and choose a new password.": "Si no lo solicitaste, cancela el cambio y elige una contraseña nueva.",
	"Someone tried to use your email address":                                   "Alguien intentó usar tu dirección de correo electrónico",
	"Someone asked to use this email address for another Staryield account.":    "Alguien pidió usar esta dirección de correo para otra cuenta de Staryield.",
	"It already belongs to your account, so nothing was changed.":               "Ya pertenece a tu cuenta, así que no se cambió nada.",
	"Sign in": "Iniciar sesión",
	"If you did not make this change, restore your previous address with this link.": "Si no hiciste este cambio, restablece tu dirección anterior con este enlace.",
	"Restore my email address":        "Restablecer mi correo electrónico",
	"Your email address was restored": "Tu dirección de correo se restableció",
	"Your previous email address is back on your account and every device was signed out.": "Tu dirección de correo anterior se restableció y se cerró la sesión en todos los dispositivos.",
	"Choose a new password within %d minutes to secure your account:":                      "Elige una contraseña nueva en un plazo de %d minutos para proteger tu cuenta:",
	// E-mails: deactivation and closure.
	"Your account is deactivated": "Tu cuenta está desactivada",
	"Your Staryield account is paused: your profile is hidden and you were signed out everywhere.": "Tu cuenta de Staryield está en pausa: tu perfil está oculto y se cerró tu sesión en todas partes.",
	"Sign in at any time to reactivate it.":                                                        "Inicia sesión cuando quieras para reactivarla.",
	"Your account was closed":                                                                      "Tu cuenta se cerró",
	"As you asked, your Staryield account was closed and your personal data erased.":               "Como pediste, tu cuenta de Staryield se cerró y se borraron tus datos personales.",
	"Records we must keep by law, such as payments, are kept without your name.":                   "Los registros que debemos conservar por ley, como los pagos, se guardan sin tu nombre.",
	"Thank you for the time you spent with us.":                                                    "Gracias por el tiempo que pasaste con nosotros.",
	// Export README.
	"This archive contains a copy of the personal data of your Staryield account.": "Este archivo contiene una copia de los datos personales de tu cuenta de Staryield.",
	"It was generated on %s (UTC).": "Se generó el %s (UTC).",
	"Each JSON file covers one part of your account; manifest.json lists every file with its checksum.": "Cada archivo JSON cubre una parte de tu cuenta; manifest.json enumera todos los archivos con su suma de verificación.",
	"Images you shared in chats are in the chat/attachments folder.":                                    "Las imágenes que compartiste en los chats están en la carpeta chat/attachments.",
	"Not included:": "No incluido:",
	"Notes that advisors wrote about you: they are the advisors' working notes, not data you provided.": "Las notas que los asesores escribieron sobre ti: son notas de trabajo de los asesores, no datos que tú proporcionaste.",
	"Security secrets such as your password hash, sign-in tokens and encrypted payout details.":         "Secretos de seguridad como el hash de tu contraseña, los tokens de inicio de sesión y los datos de pago cifrados.",
	"Only the first %d %s are included.":                                             "Solo se incluyen los primeros %d %s.",
	"%d images were too large to include; they are listed in chat/attachments.json.": "%d imágenes eran demasiado grandes para incluirlas; aparecen en chat/attachments.json.",
	"credit movements": "movimientos de créditos",
	"chat messages":    "mensajes de chat",
	"notifications":    "notificaciones",
}

func init() {
	for k, v := range frIdentity {
		fr[k] = v
	}
	for k, v := range esIdentity {
		es[k] = v
	}
	templates = buildTemplates()
}
