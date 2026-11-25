package i18n

// growth: e-mail delivery. The admin console's delivery view (its validation
// message) and the test e-mail it sends. Merged into the main catalogues at
// start-up like receipts.go.

var frMailDelivery = map[string]string{
	"Use a number of days between 1 and 30.": "Utilisez un nombre de jours compris entre 1 et 30.",
	"Staryield test e-mail":                  "E-mail de test Staryield",
	"E-mail delivery works.":                 "L'envoi des e-mails fonctionne.",
	"E-mail delivery works":                  "L'envoi des e-mails fonctionne",
	"This test e-mail was sent from the Staryield admin console. If you can read it, the e-mail provider delivers to this inbox.": "Cet e-mail de test a été envoyé depuis la console d'administration de Staryield. Si vous pouvez le lire, le fournisseur d'e-mails livre bien cette boîte de réception.",
}

var esMailDelivery = map[string]string{
	"Use a number of days between 1 and 30.": "Usa un número de días entre 1 y 30.",
	"Staryield test e-mail":                  "Correo de prueba de Staryield",
	"E-mail delivery works.":                 "El envío de correos funciona.",
	"E-mail delivery works":                  "El envío de correos funciona",
	"This test e-mail was sent from the Staryield admin console. If you can read it, the e-mail provider delivers to this inbox.": "Este correo de prueba se envió desde la consola de administración de Staryield. Si puedes leerlo, el proveedor de correo entrega en esta bandeja de entrada.",
}

func init() {
	for k, v := range frMailDelivery {
		fr[k] = v
	}
	for k, v := range esMailDelivery {
		es[k] = v
	}
}
