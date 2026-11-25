package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Growth: the seams app.go wires between the e-mail verification, referral
// and lifecycle features, exercised through the public API only (no test
// calls a hook directly).

// A friend who completed a reading qualifies their inviter's reward the
// moment they open the verification link from the welcome e-mail.
func TestVerificationLinkQualifiesTheReferral(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	e.ensurePacks()
	psychic := e.createPsychic(40)
	inviter := e.registerClient()
	e.elsewhere(inviter.user.ID)
	e.skipWelcomeMail()

	friend := e.registerReferred(inviter.referrals().Code, "")
	_, token := verificationLink(t, e, friend.user.Email)
	e.readingOf(psychic, friend)
	time.Sleep(100 * time.Millisecond)
	if r := e.referralOf(friend.user.ID); r.Status != domain.ReferralJoined {
		t.Fatalf("qualified before the address was verified: %+v", r)
	}

	// Opened on another device (no session), as from a phone's mail app.
	var res service.VerificationResult
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}, &res)
	if res.Status != "verified" {
		t.Fatalf("verify: %+v", res)
	}
	q := e.waitReferral(friend.user.ID, domain.ReferralQualified)
	if q.RewardMinutes != 5 || q.FirstSessionID == nil {
		t.Fatalf("qualified: %+v", q)
	}
	waitReferralNotice(t, inviter, service.NotifyReferralQualified)
}

// With the app's wiring the admin console lists every journey as available
// (no "no verification" or "referrals off" note), and the reminder sends a real, fresh
// verification link (transactional, in the security category) through the
// e-mail verification feature.
func TestLifecycleVerifyReminderSendsARealLink(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	e.lcStart()
	// lcStart isolates the journeys; put the app's wiring back.
	e.app.Services.Lifecycle.SetVerificationReminder(e.app.Services.EmailVerification.SendReminder)
	e.app.Services.Lifecycle.SetReferralsEnabled(e.app.Services.Referrals.Enabled)
	e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = jsonb_set(value, '{journeys,first_reading,enabled}', 'false') WHERE key = 'lifecycle'`)
	admin := e.createAdmin()
	var view service.LifecycleView
	admin.must(http.StatusOK, http.MethodGet, lifecyclePath, nil, &view)
	for _, j := range view.Journeys {
		if !j.Available || j.Note != "" {
			t.Fatalf("journey %s is held back with the app's wiring: %+v", j.Key, j)
		}
	}
	c := e.registerClient()
	e.app.DB.Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, c.user.ID)
	e.lcAge(c.user.ID, 25*time.Hour)
	e.skipWelcomeMail()
	// The welcome link went out moments ago (the test's sign-up); the reminder
	// respects the resend cooldown, so let it pass.
	e.clock.Advance(2 * time.Minute)

	e.lcRun()
	rows := e.lcMessages(c.user.ID, domain.JourneyVerifyReminder)
	if len(rows) != 1 || rows[0].Status != domain.LifecycleSent {
		t.Fatalf("reminder: %+v", rows)
	}
	_, token := verificationLink(t, e, c.user.Email)
	var res service.VerificationResult
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}, &res)
	if res.Status != "verified" || !res.EmailVerified {
		t.Fatalf("the reminder's link does not verify: %+v", res)
	}
}
