package i18n

// Follow-ups: auto-away after missed requests. The admin endpoint's validation
// message and the advisor's notification (in-app is rendered by the web app;
// e-mail and push text come from here). Merged into the main catalogues at
// start-up like lifecycle.go.

var frAutoAway = map[string]string{
	"Choose between 2 and 10.": "Choisissez une valeur entre 2 et 10.",
	"You were set to Away":     "Votre statut est passé à Absent",
	"You were set to Away after %d missed requests. Set yourself Online when you are ready.": "Votre statut est passé à Absent après %d demandes manquées. Repassez en ligne quand vous serez disponible.",
}

var esAutoAway = map[string]string{
	"Choose between 2 and 10.": "Elige un valor entre 2 y 10.",
	"You were set to Away":     "Tu estado cambió a Ausente",
	"You were set to Away after %d missed requests. Set yourself Online when you are ready.": "Tu estado cambió a Ausente tras %d solicitudes sin responder. Vuelve a ponerte en línea cuando quieras.",
}

func init() {
	for k, v := range frAutoAway {
		fr[k] = v
	}
	for k, v := range esAutoAway {
		es[k] = v
	}
	templates = buildTemplates()
}
