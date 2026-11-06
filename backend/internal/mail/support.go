package mail

import "fmt"

// Contact-form e-mails. The acknowledgement goes to whatever address was
// typed into the form, so it never repeats the submitted subject or message
// (the form must not relay text to arbitrary inboxes).

// SupportAcknowledgement confirms receipt of a support request.
func SupportAcknowledgement(locale, reference string) Content {
	t := tr(locale)
	return Content{
		Subject: t(fmt.Sprintf("We received your request %s", reference)),
		Heading: t("We received your request"),
		Paragraphs: []string{
			t(fmt.Sprintf("Thank you for contacting Staryield. Your request reference is %s.", reference)),
			t("Our team usually replies within two business days. Please keep the reference if you write to us again."),
		},
		Note: t("If you did not contact us, you can ignore this email."),
	}
}

// SupportTicketForTeam is the copy sent to the support inbox (English, for
// staff). name, email, subject and message are the requester's input.
func SupportTicketForTeam(reference, topic, name, email, subject, message string, signedIn bool) Content {
	account := "no (anonymous)"
	if signedIn {
		account = "yes"
	}
	return Content{
		Subject: fmt.Sprintf("[%s] %s: %s", reference, topic, subject),
		Heading: fmt.Sprintf("Support request %s", reference),
		Paragraphs: []string{
			fmt.Sprintf("Topic: %s", topic),
			fmt.Sprintf("From: %s <%s> (signed-in account: %s)", name, email, account),
			fmt.Sprintf("Subject: %s", subject),
			message,
		},
		Note: "Reply to the requester from the support mailbox. Do not forward this message outside the team.",
	}
}
