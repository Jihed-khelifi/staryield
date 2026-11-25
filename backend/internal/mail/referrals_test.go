package mail

import (
	"strings"
	"testing"
)

// Growth: every variant of the referral notifications renders in the three
// languages, says how many minutes, and never names anyone.
func TestReferralNotificationsAreTranslatedAndAnonymous(t *testing.T) {
	variants := []struct {
		kind string
		data map[string]any
		want string // a fragment of the English body
	}{
		{"referral.joined", map[string]any{}, "signed up with your invite link"},
		{"referral.qualified", map[string]any{"minutes": float64(5), "launched": false}, "You earned 5 free minutes, added to your wallet when the rewards launch."},
		{"referral.qualified", map[string]any{"minutes": float64(1), "launched": false}, "You earned 1 free minute,"},
		{"referral.qualified", map[string]any{"minutes": float64(5), "launched": true}, "Your 5 free minutes are being added"},
		{"referral.qualified", map[string]any{"minutes": float64(1), "launched": true}, "Your free minute is being added"},
		{"referral.reward_granted", map[string]any{"minutes": float64(15), "friends": float64(3)}, "15 free minutes from the friends you invited"},
		{"referral.reward_granted", map[string]any{"minutes": float64(1), "friends": float64(1)}, "1 free minute from the friends you invited was added"},
	}
	for _, v := range variants {
		en, ok := Notification("en", v.kind, v.data, "https://staryield.net/en/profile/invite")
		if !ok {
			t.Fatalf("%s has no content", v.kind)
		}
		if body := strings.Join(en.Paragraphs, " "); !strings.Contains(body, v.want) {
			t.Errorf("%s %v: %q lacks %q", v.kind, v.data, body, v.want)
		}
		if en.Action == nil || en.Action.URL != "https://staryield.net/en/profile/invite" {
			t.Errorf("%s: action %+v", v.kind, en.Action)
		}
		ens := contentStrings(en)
		for _, loc := range []string{"fr", "es"} {
			got := contentStrings(mustReferral(t, loc, v.kind, v.data))
			for i := range ens {
				if ens[i] != "" && got[i] == ens[i] {
					t.Errorf("%s/%s: untranslated %q", v.kind, loc, ens[i])
				}
				if strings.Contains(got[i], "%d") || strings.Contains(got[i], "%!") {
					t.Errorf("%s/%s: unfilled template %q", v.kind, loc, got[i])
				}
			}
		}
	}
	// The French and Spanish bodies keep the number of minutes.
	for _, loc := range []string{"fr", "es"} {
		c := mustReferral(t, loc, "referral.reward_granted", map[string]any{"minutes": float64(15)})
		if !strings.Contains(c.Paragraphs[0], "15") {
			t.Errorf("%s: %q", loc, c.Paragraphs[0])
		}
	}
	// Push text exists for the three kinds.
	for _, kind := range []string{"referral.joined", "referral.qualified", "referral.reward_granted"} {
		if title, body, ok := PushText("es", kind, map[string]any{"minutes": float64(5)}); !ok || title == "" || body == "" {
			t.Errorf("push %s: %q %q %v", kind, title, body, ok)
		}
	}
}

func mustReferral(t *testing.T, locale, kind string, data map[string]any) Content {
	t.Helper()
	c, ok := Notification(locale, kind, data, "https://staryield.net/"+locale+"/profile/invite")
	if !ok {
		t.Fatalf("%s/%s has no content", kind, locale)
	}
	return c
}
