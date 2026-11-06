package mail

import "fmt"

// Account security e-mails of the identity module (e-mail change, account
// deactivation and closure). They are sent in the security category and
// never include anything but the masked addresses involved.

// EmailChangeNotice tells the current address that a change to a new one
// was requested, with a link to cancel it.
func EmailChangeNotice(locale, maskedNewEmail, cancelLink string) Content {
	t := tr(locale)
	return Content{
		Subject:   t("Email change requested"),
		Preheader: t("Someone asked to change the email address of your account."),
		Heading:   t("Email change requested"),
		Paragraphs: []string{
			t(fmt.Sprintf("Someone asked to change the email address of your Staryield account to %s.", maskedNewEmail)),
			t("If it was you, confirm the change from the email we sent to the new address."),
		},
		Action: &Action{Label: t("This was not me"), URL: cancelLink},
		Note:   t("If you did not ask for this, cancel the change and choose a new password."),
	}
}

// EmailChangeAttempt tells the owner of an address already in use that
// someone tried to move another account to it (nothing changed).
func EmailChangeAttempt(locale, signInLink string) Content {
	t := tr(locale)
	return Content{
		Subject: t("Someone tried to use your email address"),
		Heading: t("Someone tried to use your email address"),
		Paragraphs: []string{
			t("Someone asked to use this email address for another Staryield account."),
			t("It already belongs to your account, so nothing was changed."),
		},
		Action: &Action{Label: t("Sign in"), URL: signInLink},
		Note:   t("If you did not ask for this, you can ignore this email."),
	}
}

// EmailChangedRevertible tells the previous address that the change was
// confirmed, with a link to restore it.
func EmailChangedRevertible(locale, maskedNewEmail, restoreLink string) Content {
	t := tr(locale)
	return Content{
		Subject: t("Your email address was changed"),
		Heading: t("Your email address was changed"),
		Paragraphs: []string{
			t(fmt.Sprintf("The email address of your Staryield account was changed to %s.", maskedNewEmail)),
			t("If you did not make this change, restore your previous address with this link."),
		},
		Action: &Action{Label: t("Restore my email address"), URL: restoreLink},
		Note:   t("If you did not make this change, contact our support team immediately."),
	}
}

// EmailRestored confirms that the previous address was restored and asks
// for a new password.
func EmailRestored(locale, resetLink string, minutes int) Content {
	t := tr(locale)
	return Content{
		Subject: t("Your email address was restored"),
		Heading: t("Your email address was restored"),
		Paragraphs: []string{
			t("Your previous email address is back on your account and every device was signed out."),
			t(fmt.Sprintf("Choose a new password within %d minutes to secure your account:", minutes)),
		},
		Action: &Action{Label: t("Choose a new password"), URL: resetLink},
		Note:   t("If you did not make this change, contact our support team immediately."),
	}
}

// AccountDeactivated confirms a self-service deactivation.
func AccountDeactivated(locale, signInLink string) Content {
	t := tr(locale)
	return Content{
		Subject: t("Your account is deactivated"),
		Heading: t("Your account is deactivated"),
		Paragraphs: []string{
			t("Your Staryield account is paused: your profile is hidden and you were signed out everywhere."),
			t("Sign in at any time to reactivate it."),
		},
		Action: &Action{Label: t("Sign in"), URL: signInLink},
		Note:   t("If you did not make this change, contact our support team immediately."),
	}
}

// AccountClosed is the last e-mail sent to an account, when it is erased.
func AccountClosed(locale string) Content {
	t := tr(locale)
	return Content{
		Subject: t("Your account was closed"),
		Heading: t("Your account was closed"),
		Paragraphs: []string{
			t("As you asked, your Staryield account was closed and your personal data erased."),
			t("Records we must keep by law, such as payments, are kept without your name."),
		},
		Note: t("Thank you for the time you spent with us."),
	}
}
