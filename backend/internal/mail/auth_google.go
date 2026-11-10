package mail

import "fmt"

// Account security e-mails of Sign in with Google (revisions 4.0, §23.1).
// They go out in the security category and only ever name the masked
// Google address.

// GoogleLinked tells the account owner that Google sign-in was added.
func GoogleLinked(locale, maskedGoogleEmail, settingsLink string) Content {
	t := tr(locale)
	return Content{
		Subject:   t("Google sign-in was added to your account"),
		Preheader: t("You can now sign in to Staryield with Google."),
		Heading:   t("Google sign-in was added to your account"),
		Paragraphs: []string{
			t(fmt.Sprintf("You can now sign in to your Staryield account with the Google account %s.", maskedGoogleEmail)),
			t("Your password, if you have one, still works."),
		},
		Action: &Action{Label: t("Review your sign-in methods"), URL: settingsLink},
		Note:   t("If you did not make this change, contact our support team immediately and choose a new password."),
	}
}

// GoogleUnlinked tells the account owner that Google sign-in was removed.
func GoogleUnlinked(locale, settingsLink string) Content {
	t := tr(locale)
	return Content{
		Subject:   t("Google sign-in was removed from your account"),
		Preheader: t("Google can no longer be used to sign in to Staryield."),
		Heading:   t("Google sign-in was removed from your account"),
		Paragraphs: []string{
			t("Google can no longer be used to sign in to your Staryield account."),
			t("You can still sign in with your password."),
		},
		Action: &Action{Label: t("Review your sign-in methods"), URL: settingsLink},
		Note:   t("If you did not make this change, contact our support team immediately and choose a new password."),
	}
}
