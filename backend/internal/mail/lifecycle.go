package mail

import "fmt"

// Lifecycle messages: the e-mail and Web Push form of the notification kinds
// the lifecycle job sends. The nudges are promotional: their channel policy is
// the marketing category, so they only reach people who opted in. The copy
// states only what is true when it is sent (the wallet's free minutes, the
// advisors that are online at that moment). The verification reminder
// (lifecycle.verify_reminder) is in-app only: its e-mail is the fresh
// verification link the e-mail verification feature sends.

// lifecycleNotification renders a lifecycle kind; ok is false for another kind.
func lifecycleNotification(t func(string) string, kind string, data map[string]any, link string) (Content, bool) {
	action := func(label string) *Action { return &Action{Label: t(label), URL: link} }
	free, online := intOf(data["free_minutes"]), intOf(data["online_advisors"])
	switch kind {
	case "lifecycle.first_reading":
		switch {
		case free == 1:
			return Content{Subject: t("Your free minute is waiting"),
				Paragraphs: []string{t("You have 1 free minute to use on your first reading.")},
				Action:     action("Choose an advisor")}, true
		case free > 1:
			return Content{Subject: t("Your free minutes are waiting"),
				Paragraphs: []string{t(fmt.Sprintf("You have %d free minutes to use on your first reading.", free))},
				Action:     action("Choose an advisor")}, true
		case online > 0:
			return Content{Subject: t("Advisors are online now"),
				Paragraphs: []string{t("Choose an advisor and start your first reading.")},
				Action:     action("Choose an advisor")}, true
		}
		return Content{Subject: t("Your first reading is waiting"),
			Paragraphs: []string{t("Choose an advisor and start your first reading whenever you are ready.")},
			Action:     action("Choose an advisor")}, true
	case "lifecycle.invite_friends":
		return Content{Subject: t("Enjoyed your reading?"),
			Paragraphs: []string{t("Invite a friend to Staryield and share the experience.")},
			Action:     action("Invite a friend")}, true
	case "lifecycle.win_back":
		switch {
		case online == 1:
			return Content{Subject: t("An advisor is online now"),
				Paragraphs: []string{t("1 advisor is online now. Pick up where you left off.")},
				Action:     action("Choose an advisor")}, true
		case online > 1:
			return Content{Subject: t("Advisors are online now"),
				Paragraphs: []string{t(fmt.Sprintf("%d advisors are online now. Pick up where you left off.", online))},
				Action:     action("Choose an advisor")}, true
		}
		return Content{Subject: t("Your daily horoscope is ready"),
			Paragraphs: []string{t("Read today's horoscope for your sign.")},
			Action:     action("Read your horoscope")}, true
	}
	return Content{}, false
}
