package mail

import "fmt"

// EmailVerification is the welcome e-mail with the link that confirms the
// address of a new account (security category; the link is single use).
func EmailVerification(locale, link string, hours int) Content {
	t := tr(locale)
	return Content{
		Subject:   t("Welcome to Staryield — confirm your email"),
		Preheader: t("One click confirms your email address."),
		Heading:   t("Welcome to Staryield"),
		Paragraphs: []string{
			t("Your account is ready. Confirm your email address to keep it secure and to receive receipts and account notices."),
			t(fmt.Sprintf("The link works once and expires in %d hours.", hours)),
		},
		Action: &Action{Label: t("Confirm my email"), URL: link},
		Note:   t("If you did not create an account, ignore this email."),
	}
}
