package mail

// revisions 6.0 (docs/platform-extensions.md §26.5): the alert the support
// inbox receives when a payment needs a human: a dispute opened or lost, an
// early fraud warning, a refund the processor could not complete. English only
// (staff copy); the lines carry ids and amounts, never card data or message text.

// PaymentAlertForTeam builds the alert. heading is the subject and title,
// lines the facts, link the console page that handles it ("" for none).
func PaymentAlertForTeam(heading string, lines []string, link string) Content {
	c := Content{
		Subject:    "[Payments] " + heading,
		Heading:    heading,
		Paragraphs: lines,
		Note:       "Handle it from the payments pages of the console. Do not forward this message outside the team.",
	}
	if link != "" {
		c.Action = &Action{Label: "Open in the console", URL: link}
	}
	return c
}
