package mail

import "fmt"

// Growth: the referral loop. Copy never names the friend (inviters learn
// counts, never who joined or who read) and never promises cash: the reward is
// free chat minutes, added when the rewards launch.

// referralNotification builds the e-mail (and Web Push text) of the referral
// notification kinds; ok is false for any other kind.
func referralNotification(t func(string) string, kind string, data map[string]any, link string) (Content, bool) {
	minutes := int(intOf(data["minutes"]))
	switch kind {
	case "referral.joined":
		return Content{Subject: t("A friend you invited joined Staryield"),
			Paragraphs: []string{
				t("A friend signed up with your invite link."),
				t("Your free minutes are earned once they verify their e-mail address and complete a reading."),
			},
			Action: &Action{Label: t("See your invites"), URL: link}}, true
	case "referral.qualified":
		body := t("A friend you invited completed a reading.")
		var extra string
		switch {
		case minutes == 1 && data["launched"] == true:
			extra = t("Your free minute is being added to your wallet.")
		case data["launched"] == true:
			extra = t(fmt.Sprintf("Your %d free minutes are being added to your wallet.", minutes))
		case minutes == 1:
			extra = t("You earned 1 free minute, added to your wallet when the rewards launch.")
		default:
			extra = t(fmt.Sprintf("You earned %d free minutes, added to your wallet when the rewards launch.", minutes))
		}
		return Content{Subject: t("A friend you invited completed a reading"), Paragraphs: []string{body, extra},
			Action: &Action{Label: t("See your invites"), URL: link}}, true
	case "referral.reward_granted":
		p := t(fmt.Sprintf("%d free minutes from the friends you invited were added to your wallet.", minutes))
		if minutes == 1 {
			p = t("1 free minute from the friends you invited was added to your wallet.")
		}
		return Content{Subject: t("Your free minutes were added"),
			Paragraphs: []string{p, t("Thank you for inviting your friends to Staryield.")},
			Action:     &Action{Label: t("See your invites"), URL: link}}, true
	}
	return Content{}, false
}
