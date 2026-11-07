package mail

import (
	"fmt"
	"strings"
)

// Contact-form e-mails. The acknowledgement goes to whatever address was
// typed into the form, so it never repeats the submitted subject or message
// (the form must not relay text to arbitrary inboxes).

// SupportAcknowledgement confirms receipt of a support request. link is the
// private link to the request (it carries the access token in its fragment).
func SupportAcknowledgement(locale, reference, link string) Content {
	t := tr(locale)
	c := Content{
		Subject: t(fmt.Sprintf("We received your request %s", reference)),
		Heading: t("We received your request"),
		Paragraphs: []string{
			t(fmt.Sprintf("Thank you for contacting Staryield. Your request reference is %s.", reference)),
			t("Our team usually replies within two business days. Please keep the reference if you write to us again."),
		},
		Note: t("If you did not contact us, you can ignore this email."),
	}
	if link != "" {
		c.Paragraphs = append(c.Paragraphs, t("Use the button below to follow your request and answer our team. Keep this link private: anyone who has it can read your request."))
		c.Action = &Action{Label: t("View your request"), URL: link}
	}
	return c
}

// SupportReply tells the requester that our team answered. It carries the
// reply text written by staff and the private link, and never repeats what the
// requester submitted (the form accepts any address).
func SupportReply(locale, reference, body, link string) Content {
	t := tr(locale)
	paras := []string{t(fmt.Sprintf("Our support team replied to your request %s:", reference))}
	paras = append(paras, splitParagraphs(body)...)
	paras = append(paras, t("Use the button below to read the whole conversation and answer."))
	return Content{
		Subject:    t(fmt.Sprintf("New reply to your request %s", reference)),
		Preheader:  t(fmt.Sprintf("Our support team replied to your request %s:", reference)),
		Heading:    t("Our team replied to your request"),
		Paragraphs: paras,
		Action:     &Action{Label: t("View your request"), URL: link},
		Note:       t("Replies to this email are not read. Keep the link private: anyone who has it can read your request."),
	}
}

// SupportCustomerReply is the copy sent to the team (English, for staff) when a
// requester answers. name and body are the requester's input.
func SupportCustomerReply(reference, name, body, consoleLink string, reopened bool) Content {
	if name == "" {
		name = "(no name)"
	}
	status := "The request stays open."
	if reopened {
		status = "The request was re-opened."
	}
	paras := []string{fmt.Sprintf("Request: %s", reference), fmt.Sprintf("From: %s", name)}
	paras = append(paras, splitParagraphs(body)...)
	paras = append(paras, status)
	c := Content{
		Subject:    fmt.Sprintf("[%s] New reply from the requester", reference),
		Heading:    fmt.Sprintf("New reply on %s", reference),
		Paragraphs: paras,
		Note:       "Reply from the support console. Do not forward this message outside the team.",
	}
	if consoleLink != "" {
		c.Action = &Action{Label: "Open in the console", URL: consoleLink}
	}
	return c
}

// splitParagraphs breaks a message at blank lines (single line breaks are kept
// inside a paragraph) and drops empty parts.
func splitParagraphs(s string) []string {
	var out []string
	for _, p := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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

// SupportChatReportForTeam is the team's copy of a conversation report
// (revisions 3.1). The subject starts with "[Chat report]" ("[Urgent][Chat
// report]" when urgent); the advisor is never e-mailed. The transcript is read
// in the console (audited), never sent by e-mail.
func SupportChatReportForTeam(reference, urgent, reason, name, email, subject, details, conversationID, sessionID string, seq int64) Content {
	prefix := "[Chat report]"
	if urgent != "" {
		prefix = "[Urgent][Chat report]"
	}
	if details == "" {
		details = "(no details given)"
	}
	return Content{
		Subject: fmt.Sprintf("%s [%s] %s", prefix, reference, subject),
		Heading: fmt.Sprintf("Chat report %s", reference),
		Paragraphs: []string{
			fmt.Sprintf("Reason: %s", reason),
			fmt.Sprintf("From: %s <%s> (signed-in account: yes)", name, email),
			fmt.Sprintf("Conversation: %s, session: %s, newest message seen: %d", conversationID, sessionID, seq),
			details,
		},
		Note: "Read the transcript in the console (step-up, audited). Do not forward this message outside the team.",
	}
}
