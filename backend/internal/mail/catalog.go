package mail

import (
	"fmt"
	"math"
	"strings"

	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
)

// The catalogue builds every e-mail (and Web Push text) from English source
// strings translated through internal/i18n. Copy never includes chat
// message content: only names and amounts the in-app notification already
// shows.

func tr(locale string) func(string) string {
	locale = i18n.Normalize(locale)
	return func(s string) string { return i18n.T(locale, s) }
}

// PasswordReset is the reset-link e-mail.
func PasswordReset(locale, link string, minutes int) Content {
	t := tr(locale)
	return Content{
		Subject:   t("Reset your Staryield password"),
		Preheader: t("Use this link to choose a new password."),
		Heading:   t("Reset your password"),
		Paragraphs: []string{
			t("We received a request to reset your password."),
			t(fmt.Sprintf("Use this link within %d minutes:", minutes)),
		},
		Action: &Action{Label: t("Choose a new password"), URL: link},
		Note:   t("If you did not ask for this, you can ignore this email."),
	}
}

// EmailChangeRequested asks the owner of a new address to confirm it.
func EmailChangeRequested(locale, link string, minutes int) Content {
	t := tr(locale)
	return Content{
		Subject:   t("Confirm your new email address"),
		Preheader: t("Confirm the new address for your Staryield account."),
		Heading:   t("Confirm your new email address"),
		Paragraphs: []string{
			t("Confirm that you want to use this address for your Staryield account."),
			t(fmt.Sprintf("This link expires in %d minutes.", minutes)),
		},
		Action: &Action{Label: t("Confirm email address"), URL: link},
		Note:   t("If you did not ask for this, you can ignore this email."),
	}
}

// EmailChanged tells the previous address that the account e-mail changed.
func EmailChanged(locale, maskedNewEmail string) Content {
	t := tr(locale)
	return Content{
		Subject: t("Your email address was changed"),
		Heading: t("Your email address was changed"),
		Paragraphs: []string{
			t(fmt.Sprintf("The email address of your Staryield account was changed to %s.", maskedNewEmail)),
		},
		Note: t("If you did not make this change, contact our support team immediately."),
	}
}

// ClosureScheduled confirms a self-service account closure request.
func ClosureScheduled(locale, date, link string) Content {
	t := tr(locale)
	return Content{
		Subject: t("Your account is scheduled for closure"),
		Heading: t("Your account is scheduled for closure"),
		Paragraphs: []string{
			t(fmt.Sprintf("Your Staryield account will be closed permanently on %s.", date)),
			t("Changed your mind? Sign in before that date to keep your account."),
		},
		Action: &Action{Label: t("Keep my account"), URL: link},
		Note:   t("If you did not make this change, contact our support team immediately."),
	}
}

// Reactivated confirms an account was reactivated.
func Reactivated(locale, link string) Content {
	t := tr(locale)
	return Content{
		Subject:    t("Your account is active again"),
		Heading:    t("Your account is active again"),
		Paragraphs: []string{t("Welcome back. Your Staryield account has been reactivated.")},
		Action:     &Action{Label: t("Open Staryield"), URL: link},
		Note:       t("If this was not you, change your password immediately."),
	}
}

// Notification renders the e-mail for an in-app notification kind (Kind and
// Data as stored on the notification; link is the absolute URL to open).
// ok is false for kinds that have no e-mail form.
func Notification(locale, kind string, data map[string]any, link string) (Content, bool) {
	t := tr(locale)
	var c Content
	switch kind {
	case "chat.requested":
		c = Content{Subject: t("New chat request"),
			Paragraphs: []string{named(t, data, "client_name", "%s wants to start a live chat.", "A client wants to start a live chat.")},
			Action:     &Action{Label: t("Open the chatroom"), URL: link}}
	case "chat.missed":
		c = Content{Subject: t("You missed a chat request"),
			Paragraphs: []string{
				named(t, data, "client_name", "%s tried to start a live chat with you while you were away.",
					"A client tried to start a live chat with you while you were away."),
				t("Go online to let clients reach you again."),
			},
			Action: &Action{Label: t("Open the chatroom"), URL: link}}
	case "chat.review_prompt":
		c = Content{Subject: t("How was your reading?"),
			Paragraphs: []string{
				named(t, data, "psychic_name", "Rate your session with %s.", "Rate your last session."),
				t("Your feedback helps other clients choose their advisor."),
			},
			Action: &Action{Label: t("Leave a review"), URL: link}}
	case "review.received":
		c = Content{Subject: t("You received a new review"),
			Paragraphs: []string{t(fmt.Sprintf("A client rated your reading %d out of 5.", intOf(data["rating"])))},
			Action:     &Action{Label: t("See your reviews"), URL: link}}
	case "request.created":
		c = Content{Subject: t("New custom request"),
			Paragraphs: []string{
				named(t, data, "service_name", "You have a new order for %s.", "You have a new custom request."),
				t("Accept or decline it from your dashboard before it expires."),
			},
			Action: &Action{Label: t("Review the request"), URL: link}}
	case "request.updated":
		c = Content{Subject: t("Update on your custom request"),
			Paragraphs: []string{requestStatus(t, str(data["service_name"]), str(data["status"]))},
			Action:     &Action{Label: t("View the request"), URL: link}}
	case "purchase.succeeded":
		c = Content{Subject: t("Credits added"),
			Paragraphs: []string{t(fmt.Sprintf("%d credits were added to your balance.", intOf(data["credits"])))},
			Action:     &Action{Label: t("View your balance"), URL: link}}
	case "credits.adjusted":
		amount := intOf(data["amount"])
		p := t(fmt.Sprintf("%d credits were added to your balance by our support team.", amount))
		if amount < 0 {
			p = t(fmt.Sprintf("%d credits were removed from your balance by our support team.", -amount))
		}
		c = Content{Subject: t("Your credit balance changed"), Paragraphs: []string{p},
			Action: &Action{Label: t("View your balance"), URL: link}}
	case "withdrawal.updated":
		credits := intOf(data["amount_credits"])
		var p string
		switch str(data["status"]) {
		case "approved":
			p = t(fmt.Sprintf("Your withdrawal of %d credits was approved.", credits))
		case "paid":
			p = t(fmt.Sprintf("Your withdrawal of %d credits was paid.", credits))
		case "rejected":
			p = t(fmt.Sprintf("Your withdrawal of %d credits was rejected. The credits are available again.", credits))
		default:
			p = t(fmt.Sprintf("Your withdrawal of %d credits was updated.", credits))
		}
		c = Content{Subject: t("Update on your withdrawal"), Paragraphs: []string{p},
			Action: &Action{Label: t("View your withdrawals"), URL: link}}
	case "waitlist.available":
		c = Content{Subject: t("Your advisor is online"),
			Paragraphs: []string{named(t, data, "psychic_name", "%s is online now. Start your reading while they are available.",
				"An advisor you waited for is online now.")},
			Action: &Action{Label: t("Start a chat"), URL: link}}
	case "export.ready":
		c = Content{Subject: t("Your data export is ready"),
			Paragraphs: []string{
				t("The copy of your Staryield data you requested is ready."),
				t("Download it from your account settings before the link expires."),
			},
			Action: &Action{Label: t("Download your data"), URL: link}}
	case "subscription.renewed":
		c = Content{Subject: t("Your membership was renewed"),
			Paragraphs: []string{named(t, data, "plan_name", "Your %s membership was renewed. Thank you for staying with us.",
				"Your membership was renewed. Thank you for staying with us.")},
			Action: &Action{Label: t("Manage membership"), URL: link}}
	case "subscription.payment_failed":
		c = Content{Subject: t("We could not renew your membership"),
			Paragraphs: []string{
				t("We could not charge your payment method for your membership."),
				t("Update your payment method to keep your benefits."),
			},
			Action: &Action{Label: t("Update payment method"), URL: link}}
	case "subscription.canceled":
		c = Content{Subject: t("Your membership has ended"),
			Paragraphs: []string{t("Your membership has ended. You can subscribe again at any time.")},
			Action:     &Action{Label: t("See plans"), URL: link}}
	case "broadcast":
		title, body := localizedPair(locale, data["title"]), localizedPair(locale, data["body"])
		if title == "" {
			return Content{}, false
		}
		c = Content{Subject: title, Paragraphs: nonEmpty(body), Action: &Action{Label: t("Open Staryield"), URL: link}}
	default:
		return Content{}, false
	}
	if c.Heading == "" {
		c.Heading = c.Subject
	}
	if c.Preheader == "" && len(c.Paragraphs) > 0 {
		c.Preheader = c.Paragraphs[0]
	}
	return c, true
}

// PushText returns the Web Push title and body for a notification kind.
func PushText(locale, kind string, data map[string]any) (title, body string, ok bool) {
	c, ok := Notification(locale, kind, data, "")
	if !ok {
		return "", "", false
	}
	if len(c.Paragraphs) > 0 {
		body = c.Paragraphs[0]
	}
	return c.Subject, truncateRunes(body, 180), true
}

// named formats withName with the data value (a person or service name), or
// returns fallback when the value is missing. Values are cleaned to a single
// line of at most 80 characters.
func named(t func(string) string, data map[string]any, key, withName, fallback string) string {
	v := truncateRunes(oneLine(str(data[key])), 80)
	if v == "" {
		return t(fallback)
	}
	return t(fmt.Sprintf(withName, v))
}

func requestStatus(t func(string) string, service, status string) string {
	service = truncateRunes(oneLine(service), 80)
	if service == "" {
		return t("Your custom request was updated.")
	}
	switch status {
	case "accepted":
		return t(fmt.Sprintf("Your request for %s was accepted.", service))
	case "delivered":
		return t(fmt.Sprintf("Your request for %s was delivered.", service))
	case "declined":
		return t(fmt.Sprintf("Your request for %s was declined. The held credits are back in your balance.", service))
	case "cancelled":
		return t(fmt.Sprintf("The request for %s was cancelled.", service))
	case "expired":
		return t(fmt.Sprintf("The request for %s expired.", service))
	case "disputed":
		return t(fmt.Sprintf("The request for %s is under review by our support team.", service))
	case "refunded":
		return t(fmt.Sprintf("The request for %s was refunded.", service))
	}
	return t(fmt.Sprintf("The request for %s was updated.", service))
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func intOf(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0
		}
		return int64(n)
	case float32:
		return int64(n)
	}
	return 0
}

// localizedPair picks the locale's entry of a {"en": "...", "fr": "..."}
// map, falling back to English.
func localizedPair(locale string, v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		if s, ok := v.(map[string]string); ok {
			m = map[string]any{}
			for k, x := range s {
				m[k] = x
			}
		} else {
			return ""
		}
	}
	if s := str(m[i18n.Normalize(locale)]); s != "" {
		return s
	}
	return str(m["en"])
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
