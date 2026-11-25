package i18n

// Growth: e-mail verification by secure link. API messages and the welcome
// e-mail that carries the link. Merged into the main catalogues at start-up
// like calculator_report.go.

var frEmailVerification = map[string]string{
	"Your email address is already verified.":                "Votre adresse e-mail est déjà confirmée.",
	"Only active accounts can receive a verification email.": "Seuls les comptes actifs peuvent recevoir un e-mail de confirmation.",
	"Use true or false.":                                     "Utilisez true ou false.",
	"Welcome to Staryield — confirm your email":              "Bienvenue sur Staryield — confirmez votre e-mail",
	"One click confirms your email address.":                 "Un clic suffit pour confirmer votre adresse e-mail.",
	"Welcome to Staryield":                                   "Bienvenue sur Staryield",
	"Your account is ready. Confirm your email address to keep it secure and to receive receipts and account notices.": "Votre compte est prêt. Confirmez votre adresse e-mail pour le sécuriser et recevoir vos reçus et les avis concernant votre compte.",
	"The link works once and expires in %d hours.":                                                                     "Le lien ne fonctionne qu'une fois et expire dans %d heures.",
	"Confirm my email": "Confirmer mon e-mail",
	"If you did not create an account, ignore this email.": "Si vous n'avez pas créé de compte, ignorez cet e-mail.",
	"We can't deliver email to this address: earlier messages bounced or were refused. Check the address or change it in your account settings.": "Nous ne pouvons pas envoyer d'e-mail à cette adresse : nos précédents messages ont été rejetés ou refusés. Vérifiez l'adresse ou modifiez-la dans les paramètres de votre compte.",
	"This address has received the maximum number of verification emails for today. Please try again later.":                                     "Cette adresse a reçu le nombre maximal d'e-mails de confirmation pour aujourd'hui. Veuillez réessayer plus tard.",
}

var esEmailVerification = map[string]string{
	"Your email address is already verified.":                "Tu dirección de correo ya está confirmada.",
	"Only active accounts can receive a verification email.": "Solo las cuentas activas pueden recibir un correo de confirmación.",
	"Use true or false.":                                     "Usa true o false.",
	"Welcome to Staryield — confirm your email":              "Te damos la bienvenida a Staryield: confirma tu correo",
	"One click confirms your email address.":                 "Un clic basta para confirmar tu dirección de correo.",
	"Welcome to Staryield":                                   "Te damos la bienvenida a Staryield",
	"Your account is ready. Confirm your email address to keep it secure and to receive receipts and account notices.": "Tu cuenta está lista. Confirma tu dirección de correo para mantenerla segura y recibir tus recibos y avisos de la cuenta.",
	"The link works once and expires in %d hours.":                                                                     "El enlace funciona una sola vez y caduca en %d horas.",
	"Confirm my email": "Confirmar mi correo",
	"If you did not create an account, ignore this email.": "Si no creaste una cuenta, ignora este correo.",
	"We can't deliver email to this address: earlier messages bounced or were refused. Check the address or change it in your account settings.": "No podemos enviar correos a esta dirección: nuestros mensajes anteriores fueron devueltos o rechazados. Comprueba la dirección o cámbiala en los ajustes de tu cuenta.",
	"This address has received the maximum number of verification emails for today. Please try again later.":                                     "Esta dirección ya recibió el número máximo de correos de confirmación por hoy. Vuelve a intentarlo más tarde.",
}

func init() {
	for k, v := range frEmailVerification {
		fr[k] = v
	}
	for k, v := range esEmailVerification {
		es[k] = v
	}
	templates = buildTemplates()
}
