package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Growth: the referral loop, against the real API, PostgreSQL and Redis.

// resetReferrals puts the program in a known state before the environment's
// first read of the site configuration (the tests share one database).
func (e *env) resetReferrals(enabled bool, minutes, max int) {
	e.t.Helper()
	db := e.app.DB
	if err := db.Exec("DELETE FROM referrals").Error; err != nil {
		e.t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"enabled": %t, "reward_minutes": %d, "max_rewards_per_referrer": %d, "launched_at": null}`, enabled, minutes, max)
	if err := db.Exec(`UPDATE site_settings SET value = ?::jsonb, version = version + 1 WHERE key = 'referral_program'`, doc).Error; err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() {
		_ = db.Exec(`UPDATE site_settings SET value = '{"enabled": true, "reward_minutes": 5, "max_rewards_per_referrer": 20, "launched_at": null}'::jsonb,
			version = version + 1 WHERE key = 'referral_program'`).Error
		_ = db.Exec("DELETE FROM referrals").Error
	})
}

// registerReferred signs a friend up with an invite code (and, optionally, the
// browser's anonymous id) the way the web app sends it.
func (e *env) registerReferred(code, anonymousID string) *client {
	e.t.Helper()
	c := e.client()
	attr := map[string]any{"referral_code": code, "landing_path": "/en"}
	if anonymousID != "" {
		attr["anonymous_id"] = anonymousID
	}
	var out struct {
		User *service.UserView `json:"user"`
	}
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("friend") + "@example.com", "password": strongPassword, "accept_terms": true, "birth_date": "1991-03-02",
		"locale": "en", "attribution": attr,
	}, &out)
	c.user, c.pw = out.User, strongPassword
	return c
}

// elsewhere makes the inviter's signed-in device come from another network
// than the test client (every test request comes from 127.0.0.1, which the
// anti-abuse rule would otherwise flag).
func (e *env) elsewhere(userID uuid.UUID) {
	e.t.Helper()
	if err := e.app.DB.Exec("UPDATE user_sessions SET ip_hash = 'another-network' WHERE user_id = ?", userID).Error; err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) verifyEmail(userID uuid.UUID) {
	e.t.Helper()
	if err := e.app.DB.Exec("UPDATE users SET email_verified_at = now() WHERE id = ?", userID).Error; err != nil {
		e.t.Fatal(err)
	}
	e.app.Services.Referrals.OnEmailVerified(context.Background(), userID)
}

func (e *env) referralOf(friend uuid.UUID) domain.Referral {
	e.t.Helper()
	var r domain.Referral
	if err := e.app.DB.First(&r, "referee_id = ?", friend).Error; err != nil {
		e.t.Fatalf("referral of %s: %v", friend, err)
	}
	return r
}

func (e *env) waitReferral(friend uuid.UUID, status domain.ReferralStatus) domain.Referral {
	e.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r := e.referralOf(friend)
		if r.Status == status || time.Now().After(deadline) {
			if r.Status != status {
				e.t.Fatalf("referral is %s, want %s: %+v", r.Status, status, r)
			}
			return r
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readingOf runs one live reading to its settlement.
func (e *env) readingOf(psychic, friend *client) {
	e.t.Helper()
	_, _, _, _, s := startedSessionOn(e.t, e, psychic, friend)
	e.tick(time.Minute + time.Second)
	endSession(e.t, friend, s.ID)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var status string
		if err := e.app.DB.Raw("SELECT status FROM chat_sessions WHERE id = ?", s.ID).Scan(&status).Error; err != nil {
			e.t.Fatal(err)
		}
		if status == "settled" {
			return
		}
		if time.Now().After(deadline) {
			e.maintenance()
			if time.Now().After(deadline.Add(5 * time.Second)) {
				e.t.Fatalf("session stays %s", status)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func referralNoticeList(t *testing.T, c *client) []service.NotificationView {
	t.Helper()
	var page struct {
		Items []service.NotificationView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications?limit=100", nil, &page)
	return page.Items
}

func waitReferralNotice(t *testing.T, c *client, kind string) service.NotificationView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		for _, n := range referralNoticeList(t, c) {
			if n.Kind == kind {
				return n
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s notification: %+v", kind, referralNoticeList(t, c))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func countReferralNotices(t *testing.T, c *client, kind string) int {
	t.Helper()
	n := 0
	for _, x := range referralNoticeList(t, c) {
		if x.Kind == kind {
			n++
		}
	}
	return n
}

func (c *client) referrals() service.MyReferrals {
	c.e.t.Helper()
	var out service.MyReferrals
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/referrals", nil, &out)
	return out
}

// The whole loop: link, sign-up, verification, a settled reading, pending
// before launch, the admin's launch, the grant and everything that announces it.
func TestReferralLoopEndToEnd(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	e.ensurePacks()
	ctx := context.Background()
	admin := e.createAdmin()
	psychic := e.createPsychic(40)

	inviter := e.registerClient()
	e.elsewhere(inviter.user.ID)
	mine := inviter.referrals()
	if len(mine.Code) != 8 || !strings.HasSuffix(mine.Link, "/en?ref="+mine.Code) ||
		mine.Program != (service.ReferralProgramView{Enabled: true, RewardMinutes: 5, MaxRewards: 20}) ||
		mine.Stats != (service.ReferralStatsView{RemainingRewards: 20}) {
		t.Fatalf("first call: %+v", mine)
	}
	if again := inviter.referrals(); again.Code != mine.Code {
		t.Fatal("the code changed between calls")
	}
	// The public site configuration advertises the program.
	var cfg struct {
		Referrals struct {
			Enabled       bool `json:"enabled"`
			RewardMinutes int  `json:"reward_minutes"`
			Launched      bool `json:"launched"`
		} `json:"referrals"`
	}
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/site/config", nil, &cfg)
	if !cfg.Referrals.Enabled || cfg.Referrals.RewardMinutes != 5 || cfg.Referrals.Launched {
		t.Fatalf("site config: %+v", cfg.Referrals)
	}

	// A friend signs up with the link (the code is case-insensitive).
	friend := e.registerReferred(strings.ToLower(mine.Code), "")
	if r := e.referralOf(friend.user.ID); r.Status != domain.ReferralJoined || r.ReferrerID != inviter.user.ID || r.Flagged || r.SignupIPHash == "" {
		t.Fatalf("referral: %+v", r)
	}
	joined := waitReferralNotice(t, inviter, service.NotifyReferralJoined)
	raw, _ := json.Marshal(joined)
	for _, leak := range []string{friend.user.ID.String(), friend.user.Email, friend.user.DisplayName} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(leak)) {
			t.Fatalf("the notification leaks %q: %s", leak, raw)
		}
	}
	if joined.Link != "/profile/invite" || len(joined.Data) != 0 {
		t.Fatalf("joined notification: %+v", joined)
	}
	if s := inviter.referrals().Stats; s.Joined != 1 || s.Qualified != 0 || s.RemainingRewards != 20 {
		t.Fatalf("after sign-up: %+v", s)
	}

	// A reading alone is not enough: the address must be verified.
	e.readingOf(psychic, friend)
	time.Sleep(100 * time.Millisecond)
	if r := e.referralOf(friend.user.ID); r.Status != domain.ReferralJoined {
		t.Fatalf("qualified without a verified address: %+v", r)
	}
	e.verifyEmail(friend.user.ID)
	q := e.waitReferral(friend.user.ID, domain.ReferralQualified)
	if q.RewardMinutes != 5 || q.FirstSessionID == nil || q.QualifiedAt == nil {
		t.Fatalf("qualified: %+v", q)
	}
	pending := waitReferralNotice(t, inviter, service.NotifyReferralQualified)
	if pending.Data["minutes"] != float64(5) || pending.Data["launched"] != false {
		t.Fatalf("pending notice: %+v", pending)
	}
	before := inviter.wallet()
	if s := inviter.referrals().Stats; s.Joined != 1 || s.Qualified != 1 || s.PendingMinutes != 5 || s.GrantedMinutes != 0 {
		t.Fatalf("pending stats: %+v", s)
	}
	if err := e.app.Scheduler.RunJob(ctx, service.ReferralGrantJob); err != nil {
		t.Fatal(err)
	}
	if w := inviter.wallet(); w.FreeMinutes != before.FreeMinutes {
		t.Fatalf("minutes granted before launch: %+v", w)
	}

	// The admin sees it, and can only launch with step-up and the typed phrase.
	var sum service.ReferralSummary
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/referrals/summary", nil, &sum)
	if sum.Qualified != 1 || sum.ReadyRewards != 1 || sum.ReadyMinutes != 5 || sum.Program.Launched() || len(sum.TopInviters) != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	e.clock.Advance(11 * time.Minute) // the step-up of the sign-in has lapsed
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/launch", map[string]any{"confirmation": "LAUNCH"}); r.errCode(t) != "step_up_required" {
		t.Fatalf("launch without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	for _, bad := range []string{"", "launch", "go"} {
		if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/launch", map[string]any{"confirmation": bad}); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("launch with %q: %d %s", bad, r.status, r.body)
		}
	}
	if inviter.referrals().Program.Launched {
		t.Fatal("launched without the confirmation")
	}
	aws := inviter.connect("")
	var launched service.ReferralLaunchResult
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/referrals/launch", map[string]any{"confirmation": "LAUNCH"}, &launched)
	if launched.Rewards != 1 || launched.Minutes != 5 || launched.LaunchedAt.IsZero() {
		t.Fatalf("launch: %+v", launched)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/launch", map[string]any{"confirmation": "LAUNCH"}); r.status != http.StatusConflict ||
		r.errCode(t) != string(service.CodeReferralLaunched) {
		t.Fatalf("second launch: %d %s", r.status, r.body)
	}

	// The grant job adds the minutes, once.
	for i := 0; i < 3; i++ {
		if err := e.app.Scheduler.RunJob(ctx, service.ReferralGrantJob); err != nil {
			t.Fatal(err)
		}
	}
	if w := inviter.wallet(); w.FreeMinutes != before.FreeMinutes+5 {
		t.Fatalf("wallet after the grant: %+v (before %+v)", w, before)
	}
	ev := aws.waitMatch(realtime.EventWalletUpdated, fmt.Sprintf(`"free_minutes":%d`, before.FreeMinutes+5), 5*time.Second)
	if ev.Type != realtime.EventWalletUpdated {
		t.Fatalf("event: %+v", ev)
	}
	granted := waitReferralNotice(t, inviter, service.NotifyReferralRewardGranted)
	if granted.Data["minutes"] != float64(5) || granted.Data["friends"] != float64(1) {
		t.Fatalf("granted notice: %+v", granted)
	}
	if n := countReferralNotices(t, inviter, service.NotifyReferralRewardGranted); n != 1 {
		t.Fatalf("%d grant notices", n)
	}
	// The e-mail goes through the outbox (enqueued by the dispatcher's workers).
	var grantMail *mail.Message
	for deadline := time.Now().Add(10 * time.Second); grantMail == nil; time.Sleep(50 * time.Millisecond) {
		for _, m := range e.flushMail() {
			if strings.EqualFold(m.To, inviter.user.Email) && strings.Contains(m.Subject, "free minutes were added") {
				grantMail = &m
			}
		}
		if grantMail == nil && time.Now().After(deadline) {
			t.Fatal("no e-mail about the added minutes")
		}
	}
	if grantMail.Category != "account" || !strings.Contains(grantMail.Text, "5 free minutes") || strings.Contains(grantMail.Text, friend.user.Email) {
		t.Fatalf("grant e-mail: %+v", *grantMail)
	}
	r := e.referralOf(friend.user.ID)
	if r.Status != domain.ReferralGranted || r.GrantedAt == nil {
		t.Fatalf("granted: %+v", r)
	}
	final := inviter.referrals()
	if !final.Program.Launched || final.Stats.GrantedMinutes != 5 || final.Stats.PendingMinutes != 0 || final.Stats.Qualified != 1 {
		t.Fatalf("final stats: %+v", final)
	}
	// What the inviter gets back never identifies the friend.
	body := inviter.call(http.MethodGet, "/api/v1/me/referrals", nil).body
	for _, leak := range []string{friend.user.ID.String(), friend.user.Email, "referee"} {
		if strings.Contains(string(body), leak) {
			t.Fatalf("GET /me/referrals leaks %q: %s", leak, body)
		}
	}

	// A friend who qualifies after the launch is granted without a pending notice.
	second := e.registerReferred(mine.Code, "")
	e.verifyEmail(second.user.ID)
	e.readingOf(psychic, second)
	e.waitReferral(second.user.ID, domain.ReferralQualified)
	if err := e.app.Scheduler.RunJob(ctx, service.ReferralGrantJob); err != nil {
		t.Fatal(err)
	}
	e.waitReferral(second.user.ID, domain.ReferralGranted)
	if w := inviter.wallet(); w.FreeMinutes != before.FreeMinutes+10 {
		t.Fatalf("wallet after the second grant: %+v", w)
	}
	if n := countReferralNotices(t, inviter, service.NotifyReferralQualified); n != 1 {
		t.Fatalf("a qualification after launch must not say 'pending': %d notices", n)
	}

	// The personal-data exports: the inviter's code and counters, the friend's
	// "invited by a friend" — never who invited whom.
	var mineExport struct {
		Code      string `json:"code"`
		Joined    int64  `json:"friends_joined"`
		Completed int64  `json:"friends_completed_a_reading"`
		Granted   int64  `json:"free_minutes_granted"`
		Invited   bool   `json:"invited_by_a_friend"`
	}
	files := exportFiles(t, e, inviter)
	if err := json.Unmarshal(files["referrals.json"], &mineExport); err != nil || mineExport.Code != mine.Code || mineExport.Joined != 2 ||
		mineExport.Completed != 2 || mineExport.Granted != 10 || mineExport.Invited {
		t.Fatalf("inviter's export: %+v %v %s", mineExport, err, files["referrals.json"])
	}
	friendFiles := exportFiles(t, e, friend)
	for _, leak := range []string{inviter.user.ID.String(), inviter.user.Email, mine.Code} {
		if strings.Contains(string(friendFiles["referrals.json"]), leak) {
			t.Fatalf("the friend's export names the inviter (%q): %s", leak, friendFiles["referrals.json"])
		}
	}
	var friendExport struct {
		Invited bool `json:"invited_by_a_friend"`
	}
	if err := json.Unmarshal(friendFiles["referrals.json"], &friendExport); err != nil || !friendExport.Invited {
		t.Fatalf("friend's export: %v %s", err, friendFiles["referrals.json"])
	}

	// Erasing the friend keeps the decision, without the friend; the inviter's
	// numbers do not move. Erasing the inviter removes their code and invites.
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+friend.user.ID.String(), map[string]any{"reason": "GDPR erasure"}, nil)
	var anon domain.Referral
	if err := e.app.DB.First(&anon, "id = ?", r.ID).Error; err != nil || anon.RefereeID != nil || anon.SignupIPHash != "" || anon.Status != domain.ReferralGranted {
		t.Fatalf("after the friend's erasure: %+v %v", anon, err)
	}
	if s := inviter.referrals().Stats; s.Qualified != 2 || s.GrantedMinutes != 10 {
		t.Fatalf("stats after the friend's erasure: %+v", s)
	}
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+inviter.user.ID.String(), map[string]any{"reason": "GDPR erasure"}, nil)
	var n int64
	e.app.DB.Raw("SELECT (SELECT count(*) FROM referral_codes WHERE user_id = ?) + (SELECT count(*) FROM referrals WHERE referrer_id = ?)",
		inviter.user.ID, inviter.user.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d referral rows survived the inviter's erasure", n)
	}
}

// The server events about a friend's sign-up and qualifying are the friend's
// own: they never carry the inviter's id (Privacy Policy 2.21: no analytics
// event identifies a friend you invited).
func TestReferralEventsDoNotCarryTheInviterId(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	psychic := e.createPsychic(40)
	inviter := e.registerClient()
	code := inviter.referrals().Code
	friend := e.registerReferred(code, "")
	e.readingOf(psychic, friend)
	e.verifyEmail(friend.user.ID)
	e.waitReferral(friend.user.ID, domain.ReferralQualified)
	time.Sleep(200 * time.Millisecond)
	e.app.Tracker.Stop(context.Background())
	var joined, qualified int64
	e.app.DB.Raw(`SELECT count(*) FROM analytics_events WHERE name = 'referral.joined' AND user_id = ?`, friend.user.ID).Row().Scan(&joined)
	e.app.DB.Raw(`SELECT count(*) FROM analytics_events WHERE name = 'referral.qualified'`).Row().Scan(&qualified)
	if joined != 1 || qualified < 1 {
		t.Fatalf("events: joined %d, qualified %d", joined, qualified)
	}
	// referral.granted is about the inviter alone and keeps referrer_id; the other two never carry it.
	var withInviter int64
	e.app.DB.Raw(`SELECT count(*) FROM analytics_events WHERE name IN ('referral.joined', 'referral.qualified')
		AND (properties::text LIKE ? OR properties::text LIKE '%referrer_id%')`, "%"+inviter.user.ID.String()+"%").Row().Scan(&withInviter)
	if withInviter != 0 {
		t.Fatalf("%d referral events carry the inviter", withInviter)
	}
}

func TestReferralEndpointAccess(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	if r := e.client().call(http.MethodGet, "/api/v1/me/referrals", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.status)
	}
	if r := e.createPsychic(40).call(http.MethodGet, "/api/v1/me/referrals", nil); r.status != http.StatusForbidden {
		t.Fatalf("advisor: %d", r.status)
	}
	// A client who has not given an adult birth date can still read the card.
	c := e.registerClient()
	if err := e.app.DB.Exec("UPDATE users SET adult_confirmed_at = NULL WHERE id = ?", c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	e.app.Services.Auth.InvalidateStatus(context.Background(), c.user.ID)
	if r := c.call(http.MethodGet, "/api/v1/me/referrals", nil); r.status != http.StatusOK {
		t.Fatalf("client without a birth date: %d %s", r.status, r.body)
	}
	if got := c.call(http.MethodGet, "/api/v1/me/referrals", nil).header.Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("cache-control: %q", got)
	}
	// Admin endpoints are admin-only.
	for _, path := range []string{"/api/v1/admin/referrals", "/api/v1/admin/referrals/summary", "/api/v1/admin/site/referral-program"} {
		if r := c.call(http.MethodGet, path, nil); r.status == http.StatusOK {
			t.Fatalf("%s answered a client", path)
		}
	}
}

// Self-referral by browser, shared networks and bursts are held or rejected
// instead of rewarded; an administrator decides the held ones.
func TestReferralAntiAbuse(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	e.ensurePacks()
	ctx := context.Background()
	admin := e.createAdmin()
	psychic := e.createPsychic(40)
	inviter := e.registerClient()
	code := inviter.referrals().Code

	// The same browser already linked to the inviter: rejected, never rewarded.
	if err := e.app.DB.Exec(`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, user_id, linked_at)
		VALUES ('inviter-browser-1', now(), now(), ?, now())`, inviter.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	self := e.registerReferred(code, "inviter-browser-1")
	if r := e.referralOf(self.user.ID); r.Status != domain.ReferralRejected || r.RejectReason != domain.ReferralRejectSelf {
		t.Fatalf("self referral: %+v", r)
	}
	e.verifyEmail(self.user.ID)
	e.readingOf(psychic, self)
	time.Sleep(100 * time.Millisecond)
	if r := e.referralOf(self.user.ID); r.Status != domain.ReferralRejected {
		t.Fatalf("a rejected referral changed: %+v", r)
	}
	if s := inviter.referrals().Stats; s.Joined != 0 || s.Qualified != 0 {
		t.Fatalf("a rejected referral is counted: %+v", s)
	}

	// A browser that already belongs to another account: held for review (one
	// person opening several accounts with the same link).
	other := e.registerClient()
	if err := e.app.DB.Exec(`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, user_id, linked_at)
		VALUES ('other-browser-1', now(), now(), ?, now())`, other.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	second := e.registerReferred(code, "other-browser-1")
	if r := e.referralOf(second.user.ID); r.Status != domain.ReferralJoined || !r.Flagged || !containsFlag(r.Flags, domain.ReferralFlagSharedBrowser) {
		t.Fatalf("browser of another account: %+v", r)
	}

	// The inviter's device comes from the same network as the sign-up: held for review.
	held := e.registerReferred(code, "")
	r := e.referralOf(held.user.ID)
	if !r.Flagged || len(r.Flags) != 1 || r.Flags[0] != domain.ReferralFlagSameNetwork {
		t.Fatalf("same network: %+v", r)
	}
	if n := countReferralNotices(t, inviter, service.NotifyReferralJoined); n != 0 {
		t.Fatalf("a held sign-up announced itself: %d", n)
	}
	e.verifyEmail(held.user.ID)
	e.readingOf(psychic, held)
	q := e.waitReferral(held.user.ID, domain.ReferralQualified)
	if !q.Flagged {
		t.Fatal("qualification dropped the flag")
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/referrals?flagged=true", nil, nil)
	var page service.Page[service.AdminReferral]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/referrals?flagged=true&status=qualified", nil, &page)
	if len(page.Items) != 1 || page.Items[0].ID != q.ID || page.Items[0].Referee == nil || page.Items[0].Referee.ID != held.user.ID ||
		page.Items[0].Referrer.ID != inviter.user.ID {
		t.Fatalf("flagged list: %+v", page)
	}

	// Launch: the held reward is not granted.
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/referrals/launch", map[string]any{"confirmation": "LAUNCH"}, nil)
	before := inviter.wallet()
	if err := e.app.Scheduler.RunJob(ctx, service.ReferralGrantJob); err != nil {
		t.Fatal(err)
	}
	if w := inviter.wallet(); w.FreeMinutes != before.FreeMinutes {
		t.Fatalf("a held reward was granted: %+v", w)
	}
	if n := countReferralNotices(t, inviter, service.NotifyReferralQualified); n != 0 {
		t.Fatalf("a held reward announced itself: %d", n)
	}

	// Approval needs step-up, releases it, and the next run grants it.
	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/"+q.ID.String()+"/approve", nil); r.errCode(t) != "step_up_required" {
		t.Fatalf("approve without step-up: %d", r.status)
	}
	admin.stepUp()
	var approved service.AdminReferral
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/referrals/"+q.ID.String()+"/approve", nil, &approved)
	if approved.Flagged || approved.DecidedBy == nil || approved.DecidedBy.ID != admin.user.ID {
		t.Fatalf("approved: %+v", approved)
	}
	if e.auditCount("referral.approved", q.ID) != 1 {
		t.Fatal("approval is not audited")
	}
	if err := e.app.Scheduler.RunJob(ctx, service.ReferralGrantJob); err != nil {
		t.Fatal(err)
	}
	if w := inviter.wallet(); w.FreeMinutes != before.FreeMinutes+5 {
		t.Fatalf("approved reward not granted: %+v", w)
	}
	admin.stepUp()
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/"+q.ID.String()+"/approve", nil); r.status != http.StatusConflict {
		t.Fatalf("approving twice: %d", r.status)
	}

	// More than ten sign-ups in a day are held. The inviter already has 3 rows
	// above (rejected and two held), so the 8th new sign-up is the 11th in the window.
	for i := 1; i <= 12; i++ {
		u := e.registerClient()
		e.app.Services.Referrals.Attribute(ctx, u.user.ID, &service.AttributionInput{ReferralCode: code})
		got := e.referralOf(u.user.ID)
		held := len(got.Flags) == 1 && got.Flags[0] == domain.ReferralFlagSignupBurst
		if held != (i >= 8) || got.Flagged != held {
			t.Fatalf("sign-up %d: %+v", i, got)
		}
	}
}

// The per-inviter cap holds under concurrency, and the grant job never
// double-grants when several instances run it at once.
func TestReferralCapAndGrantsUnderConcurrency(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 3)
	ctx := context.Background()
	inviter := e.registerClient()
	e.elsewhere(inviter.user.ID)
	code := inviter.referrals().Code

	// Ten friends are verified and have a reading, but the program was paused
	// while it happened, so none was evaluated yet.
	friends := make([]uuid.UUID, 0, 10)
	for i := 0; i < 10; i++ {
		f := e.registerClient()
		e.app.Services.Referrals.Attribute(ctx, f.user.ID, &service.AttributionInput{ReferralCode: code})
		friends = append(friends, f.user.ID)
		if err := e.app.DB.Exec("UPDATE users SET email_verified_at = now() WHERE id = ?", f.user.ID).Error; err != nil {
			t.Fatal(err)
		}
		e.settledReading(f.user.ID)
	}
	// Several instances evaluate at the same moment (hooks, sweeps).
	var wg sync.WaitGroup
	for round := 0; round < 3; round++ {
		for _, id := range friends {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.app.Services.Referrals.Evaluate(ctx, id)
			}()
		}
	}
	wg.Wait()
	var qualified, rejected int64
	e.app.DB.Raw("SELECT count(*) FROM referrals WHERE referrer_id = ? AND status = 'qualified'", inviter.user.ID).Scan(&qualified)
	e.app.DB.Raw("SELECT count(*) FROM referrals WHERE referrer_id = ? AND status = 'rejected' AND reject_reason = 'cap_reached'", inviter.user.ID).Scan(&rejected)
	if qualified != 3 || rejected != 7 {
		t.Fatalf("cap of 3: %d qualified, %d rejected", qualified, rejected)
	}
	if s := inviter.referrals().Stats; s.Joined != 3 || s.Qualified != 3 || s.RemainingRewards != 0 {
		t.Fatalf("stats: %+v", s)
	}

	// Launch directly, then run the grant job on several "instances" at once.
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/referrals/launch", map[string]any{"confirmation": "LAUNCH"}, nil)
	before := inviter.wallet()
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.app.Services.Referrals.RunGrants(ctx); err != nil {
				t.Errorf("grant run: %v", err)
			}
		}()
	}
	wg.Wait()
	if w := inviter.wallet(); w.FreeMinutes != before.FreeMinutes+15 {
		t.Fatalf("3 rewards of 5 minutes expected, wallet: %+v (before %+v)", w, before)
	}
	var granted int64
	e.app.DB.Raw("SELECT count(*) FROM referrals WHERE referrer_id = ? AND status = 'granted' AND granted_at IS NOT NULL", inviter.user.ID).Scan(&granted)
	if granted != 3 {
		t.Fatalf("%d granted", granted)
	}
	waitReferralNotice(t, inviter, service.NotifyReferralRewardGranted)
	if n := countReferralNotices(t, inviter, service.NotifyReferralRewardGranted); n < 1 || n > 3 {
		t.Fatalf("%d grant notices", n)
	}
}

func containsFlag(flags domain.StringList, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

// settledReading records a finished reading for a client without running a
// live chat (the grant and cap tests need many of them).
func (e *env) settledReading(clientID uuid.UUID) {
	e.t.Helper()
	var advisors []uuid.UUID
	if err := e.app.DB.Model(&domain.Psychic{}).Limit(1).Pluck("user_id", &advisors).Error; err != nil || len(advisors) == 0 {
		e.t.Fatalf("no advisor: %v", err)
	}
	psychic := advisors[0]
	now := e.clock.Now()
	conv := domain.Conversation{ID: domain.NewID(), ClientID: clientID, PsychicID: psychic, LastActivityAt: now, CreatedAt: now, UpdatedAt: now}
	if err := e.app.DB.Create(&conv).Error; err != nil {
		e.t.Fatalf("conversation: %v", err)
	}
	s := domain.Session{ID: domain.NewID(), ConversationID: conv.ID, ClientID: clientID, PsychicID: psychic, Status: domain.SessionSettled,
		EndReason: domain.EndClientEnded, RatePerMinute: 30, ListRatePerMinute: 30, RequestKey: uniq("seed-req-"), RequestedAt: now,
		ExpiresAt: now, EndedAt: &now, SettledAt: &now, BilledMinutes: 2, FreeMinutesUsed: 2, CreatedAt: now, UpdatedAt: now}
	if err := e.app.DB.Create(&s).Error; err != nil {
		e.t.Fatalf("session: %v", err)
	}
}

func TestReferralAdminSettingsAndDecisions(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	ctx := context.Background()
	admin := e.createAdmin()
	inviter := e.registerClient()
	e.elsewhere(inviter.user.ID)
	code := inviter.referrals().Code
	friend := e.registerReferred(code, "")

	// Settings: readable by every admin, written with step-up, versioned, audited.
	var set service.ReferralProgramSetting
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/referral-program", nil, &set)
	if !set.Enabled || set.RewardMinutes != 5 || set.MaxRewardsPerReferrer != 20 || set.Launched() || set.Version < 1 {
		t.Fatalf("setting: %+v", set)
	}
	put := func(body map[string]any) response {
		return admin.call(http.MethodPut, "/api/v1/admin/site/referral-program", body)
	}
	body := map[string]any{"enabled": true, "reward_minutes": 8, "max_rewards_per_referrer": 10, "version": set.Version}
	e.clock.Advance(11 * time.Minute)
	if r := put(body); r.errCode(t) != "step_up_required" {
		t.Fatalf("PUT without step-up: %d", r.status)
	}
	admin.stepUp()
	for _, bad := range []map[string]any{{"enabled": true, "reward_minutes": 0, "max_rewards_per_referrer": 10},
		{"enabled": true, "reward_minutes": 61, "max_rewards_per_referrer": 10}, {"enabled": true, "reward_minutes": 5, "max_rewards_per_referrer": 501},
		{"reward_minutes": 5, "max_rewards_per_referrer": 10}} {
		if r := put(bad); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("PUT %v: %d %s", bad, r.status, r.body)
		}
	}
	var saved service.ReferralProgramSetting
	r := put(body)
	if r.status != http.StatusOK {
		t.Fatalf("PUT: %d %s", r.status, r.body)
	}
	r.decode(t, &saved)
	if saved.RewardMinutes != 8 || saved.MaxRewardsPerReferrer != 10 || saved.Version != set.Version+1 || saved.UpdatedBy == nil {
		t.Fatalf("saved: %+v", saved)
	}
	if r := put(body); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	// The new value reaches the public configuration and the inviter at once.
	if got := inviter.referrals().Program; got.RewardMinutes != 8 || got.MaxRewards != 10 {
		t.Fatalf("inviter sees %+v", got)
	}
	var audits int64
	e.app.DB.Raw("SELECT count(*) FROM audit_logs WHERE action = 'site.referral_program.updated'").Scan(&audits)
	if audits < 1 {
		t.Fatal("the setting change is not audited")
	}

	// A disabled program accepts no new referral and qualifies nothing.
	admin.stepUp()
	saved.Enabled = false
	if r := put(map[string]any{"enabled": false, "reward_minutes": 8, "max_rewards_per_referrer": 10, "version": saved.Version}); r.status != http.StatusOK {
		t.Fatalf("disable: %d %s", r.status, r.body)
	}
	e.registerReferred(code, "")
	var count int64
	e.app.DB.Raw("SELECT count(*) FROM referrals WHERE referrer_id = ?", inviter.user.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("a sign-up was credited while the program is disabled: %d", count)
	}
	if got := inviter.referrals().Program; got.Enabled {
		t.Fatalf("inviter sees an enabled program: %+v", got)
	}

	// Decisions: reject needs step-up and a reason.
	var list service.Page[service.AdminReferral]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/referrals?limit=1", nil, &list)
	if len(list.Items) != 1 || list.Items[0].Referee == nil || list.Items[0].Referee.ID != friend.user.ID {
		t.Fatalf("list: %+v", list)
	}
	id := list.Items[0].ID.String()
	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/"+id+"/reject", map[string]any{"reason": "Duplicate account"}); r.errCode(t) != "step_up_required" {
		t.Fatalf("reject without step-up: %d", r.status)
	}
	admin.stepUp()
	for _, bad := range []string{"", "x", strings.Repeat("r", 201)} {
		if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/"+id+"/reject", map[string]any{"reason": bad}); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("reason %q: %d", bad, r.status)
		}
	}
	var rejected service.AdminReferral
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/referrals/"+id+"/reject", map[string]any{"reason": "Duplicate account"}, &rejected)
	if rejected.Status != "rejected" || rejected.RejectReason != "Duplicate account" || rejected.DecidedBy == nil {
		t.Fatalf("rejected: %+v", rejected)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/"+id+"/reject", map[string]any{"reason": "Again"}); r.status != http.StatusConflict {
		t.Fatalf("rejecting twice: %d", r.status)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/referrals/"+uuid.NewString()+"/reject", map[string]any{"reason": "Nobody"}); r.status != http.StatusNotFound {
		t.Fatalf("unknown id: %d", r.status)
	}
	var byStatus service.Page[service.AdminReferral]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/referrals?status=rejected", nil, &byStatus)
	if len(byStatus.Items) != 1 {
		t.Fatalf("by status: %+v", byStatus)
	}
	for _, bad := range []string{"status=nope", "flagged=maybe", "cursor=xx", "limit=0"} {
		if r := admin.call(http.MethodGet, "/api/v1/admin/referrals?"+bad, nil); r.status < 400 {
			t.Fatalf("%s accepted", bad)
		}
	}
	_ = ctx
}

// Rejected/held friends never reveal themselves to the inviter, and the
// sweep job qualifies a friend whose hooks never ran.
func TestReferralSweepCatchesMissedHooks(t *testing.T) {
	e := newEnv(t)
	e.resetReferrals(true, 5, 20)
	ctx := context.Background()
	inviter := e.registerClient()
	e.elsewhere(inviter.user.ID)
	friend := e.registerReferred(inviter.referrals().Code, "")
	e.app.DB.Exec("UPDATE users SET email_verified_at = now() WHERE id = ?", friend.user.ID)
	e.settledReading(friend.user.ID)
	if r := e.referralOf(friend.user.ID); r.Status != domain.ReferralJoined {
		t.Fatalf("hooks did not run, yet: %+v", r)
	}
	if err := e.app.Scheduler.RunJob(ctx, service.ReferralSweepJob); err != nil {
		t.Fatal(err)
	}
	if r := e.referralOf(friend.user.ID); r.Status != domain.ReferralQualified {
		t.Fatalf("the sweep did not qualify: %+v", r)
	}
}

// Google sign-ups carry the invite code like password sign-ups, both when the
// terms were accepted before leaving for Google and through the consent step
// (the code then comes from the attempt started on the login page).
func TestReferralGoogleSignUpsCreditTheInviter(t *testing.T) {
	e, f := newGoogleEnv(t)
	e.resetReferrals(true, 5, 20)
	ctx := context.Background()
	inviter := e.registerClient()
	e.elsewhere(inviter.user.ID)
	code := inviter.referrals().Code

	// Sign-up intent with the terms: the callback creates the account.
	direct := e.gbrowser()
	direct.mustResult(f, gid(uniq("gref")+"@example.com"), map[string]any{"intent": "signup", "accept_terms": true,
		"attribution": map[string]any{"referral_code": " " + strings.ToLower(code) + " ", "landing_path": "/en"}}, "signed_up")
	var me service.UserView
	direct.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if r := e.referralOf(me.ID); r.Status != domain.ReferralJoined || r.ReferrerID != inviter.user.ID || r.Flagged {
		t.Fatalf("direct Google sign-up: %+v", r)
	}

	// Sign-in intent on an unknown account: the consent step creates it.
	consent := e.gbrowser()
	consent.mustResult(f, gid(uniq("grefc")+"@example.com"), map[string]any{"intent": "signin",
		"attribution": map[string]any{"referral_code": code}}, "signup_consent")
	var sess struct {
		User *service.UserView `json:"user"`
	}
	consent.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": true}, &sess)
	if sess.User == nil {
		t.Fatal("consent: no account")
	}
	if r := e.referralOf(sess.User.ID); r.Status != domain.ReferralJoined || r.ReferrerID != inviter.user.ID {
		t.Fatalf("Google sign-up through consent: %+v", r)
	}
	waitReferralNotice(t, inviter, service.NotifyReferralJoined)

	// Google proved the address: a reading is all the friend still needs.
	e.createPsychic(40)
	e.settledReading(me.ID)
	e.app.Services.Referrals.Evaluate(ctx, me.ID)
	if r := e.referralOf(me.ID); r.Status != domain.ReferralQualified {
		t.Fatalf("a Google friend with a reading: %+v", r)
	}
	if s := inviter.referrals().Stats; s.Joined != 2 || s.Qualified != 1 || s.PendingMinutes != 5 {
		t.Fatalf("inviter stats: %+v", s)
	}
}
