package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func TestAdminAuthRequiresAdminOriginPasswordAndTOTP(t *testing.T) {
	e := newEnv(t)
	id, email := e.insertAdmin()

	// The regular web login gives no access to admin routes.
	web := e.login(email, strongPassword)
	r := web.call(http.MethodGet, "/api/v1/admin/users", nil)
	if r.status != http.StatusForbidden || r.errCode(t) != "csrf_failed" {
		t.Fatalf("main-site origin must be refused: %d %s", r.status, r.body)
	}
	adminFromWeb := e.adminClient()
	adminFromWeb.http.Jar = web.http.Jar // the web session cookies are not an admin session
	if r := adminFromWeb.call(http.MethodGet, "/api/v1/admin/users", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("web session on admin origin: %d %s", r.status, r.body)
	}
	// Requests without Origin (scripts, image tags) are refused too.
	noOrigin := e.adminClient()
	noOrigin.origin = ""
	if r := noOrigin.raw(http.MethodGet, "/api/v1/admin/auth/session", nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("missing origin: %d", r.status)
	}

	// A client account with the right password gets the generic error.
	cl := e.registerClient()
	c := e.adminClient()
	r = c.call(http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": cl.user.Email, "password": strongPassword})
	if r.status != http.StatusUnauthorized || r.errCode(t) != "invalid_credentials" {
		t.Fatalf("client on admin login: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": "wrong-Passw0rd!"}); r.errCode(t) != "invalid_credentials" {
		t.Fatalf("wrong password: %s", r.body)
	}

	// Enrolment: the secret is stable for one challenge and only a valid
	// code completes it.
	var ch service.LoginChallenge
	c.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": strongPassword}, &ch)
	if ch.Status != "enrollment_required" || ch.Challenge == "" {
		t.Fatalf("challenge: %+v", ch)
	}
	var enr1, enr2 service.Enrollment
	c.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/enroll", map[string]any{"challenge": ch.Challenge}, &enr1)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/enroll", map[string]any{"challenge": ch.Challenge}, &enr2)
	if enr1.Secret == "" || enr1.Secret != enr2.Secret || !strings.HasPrefix(enr1.OTPAuthURL, "otpauth://totp/") {
		t.Fatalf("enrolment: %+v %+v", enr1, enr2)
	}
	r = c.call(http.MethodPost, "/api/v1/admin/auth/enroll/confirm", map[string]any{"challenge": ch.Challenge, "code": "000000"})
	if r.status != http.StatusUnauthorized || r.errCode(t) != "invalid_code" {
		t.Fatalf("bad code: %d %s", r.status, r.body)
	}
	var sess service.AdminSessionView
	r = c.call(http.MethodPost, "/api/v1/admin/auth/enroll/confirm", map[string]any{"challenge": ch.Challenge, "code": totpNow(e, enr1.Secret)})
	if r.status != http.StatusOK {
		t.Fatalf("confirm: %d %s", r.status, r.body)
	}
	r.decode(t, &sess)
	if sess.Admin.ID != id || len(sess.RecoveryCodes) != 10 || sess.CSRFToken == "" || sess.IdleTimeoutSeconds != 1800 {
		t.Fatalf("session: %+v", sess)
	}
	recoveryCodes := sess.RecoveryCodes
	// The admin cookie is HttpOnly and SameSite=Strict.
	cookie := ""
	for _, v := range r.header.Values("Set-Cookie") {
		if strings.HasPrefix(v, "sy_adm=") {
			cookie = v
		}
	}
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Strict") {
		t.Fatalf("admin cookie flags: %q", cookie)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users", nil, nil)
	// The challenge is single-use.
	if r := c.call(http.MethodPost, "/api/v1/admin/auth/enroll/confirm", map[string]any{"challenge": ch.Challenge, "code": totpNow(e, enr1.Secret)}); r.status != http.StatusBadRequest {
		t.Fatalf("reused challenge: %d %s", r.status, r.body)
	}
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/admin/auth/logout", nil, nil)
	if r := c.call(http.MethodGet, "/api/v1/admin/users", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", r.status)
	}

	// Second sign-in: TOTP verification; the same code cannot be replayed.
	c2 := e.adminClient()
	c2.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": strongPassword}, &ch)
	if ch.Status != "mfa_required" {
		t.Fatalf("expected mfa: %+v", ch)
	}
	r = c2.call(http.MethodPost, "/api/v1/admin/auth/verify", map[string]any{"challenge": ch.Challenge, "code": totpNow(e, enr1.Secret)})
	if r.errCode(t) != "invalid_code" {
		t.Fatalf("replayed code must fail: %s", r.body)
	}
	var details struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &details)
	if details.Error.Details["attempts_left"] != float64(4) {
		t.Fatalf("attempts left: %v", details.Error.Details)
	}
	e.clock.Advance(security.TOTPPeriod)
	c2.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/verify", map[string]any{"challenge": ch.Challenge, "code": totpNow(e, enr1.Secret)}, &sess)

	// Recovery codes work once.
	c3 := e.adminClient()
	c3.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": strongPassword}, &ch)
	c3.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/verify", map[string]any{"challenge": ch.Challenge,
		"recovery_code": strings.ToUpper(recoveryCodes[0])}, &sess)
	if sess.RecoveryCodesLeft != 9 {
		t.Fatalf("recovery codes left: %d", sess.RecoveryCodesLeft)
	}
	c4 := e.adminClient()
	c4.must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": strongPassword}, &ch)
	if r := c4.call(http.MethodPost, "/api/v1/admin/auth/verify", map[string]any{"challenge": ch.Challenge, "recovery_code": recoveryCodes[0]}); r.errCode(t) != "invalid_code" {
		t.Fatalf("recovery code reuse: %s", r.body)
	}

	// Five wrong codes kill the challenge.
	for i := 0; i < 4; i++ {
		c4.call(http.MethodPost, "/api/v1/admin/auth/verify", map[string]any{"challenge": ch.Challenge, "code": "123456"})
	}
	if r := c4.call(http.MethodPost, "/api/v1/admin/auth/verify", map[string]any{"challenge": ch.Challenge, "code": "123456"}); r.status != http.StatusBadRequest || r.errCode(t) != "invalid_token" {
		t.Fatalf("challenge must die after too many attempts: %d %s", r.status, r.body)
	}

	// Sessions list shows the active sessions; one can be revoked.
	var rows struct {
		Items []service.AdminSessionRow `json:"items"`
	}
	c2.must(http.StatusOK, http.MethodGet, "/api/v1/admin/auth/sessions", nil, &rows)
	if len(rows.Items) < 2 {
		t.Fatalf("sessions: %+v", rows.Items)
	}
	for _, row := range rows.Items {
		if !row.Current {
			c2.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/auth/sessions/"+row.ID.String(), nil, nil)
		}
	}
	if r := c3.call(http.MethodGet, "/api/v1/admin/auth/session", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("revoked session still valid: %d", r.status)
	}
}

func TestAdminIdleTimeoutAndStepUp(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	target := e.registerClient()

	var sess service.AdminSessionView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/auth/session", nil, &sess)
	if sess.StepUpUntil.Sub(e.clock.Now()) > 10*time.Minute || sess.IdleExpiresAt.IsZero() {
		t.Fatalf("session view: %+v", sess)
	}

	// Sensitive actions need a TOTP confirmation once the window has passed.
	e.clock.Advance(11 * time.Minute)
	r := admin.call(http.MethodPatch, "/api/v1/admin/users/"+target.user.ID.String(), map[string]any{"status": "banned", "status_reason": "Abuse"})
	if r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("ban without step-up: %d %s", r.status, r.body)
	}
	for _, path := range []string{"/api/v1/admin/users/" + target.user.ID.String() + "/credits"} {
		if r := admin.call(http.MethodPost, path, map[string]any{"amount": 10, "reason": "Goodwill", "idempotency_key": uniq("adj-key-")}); r.errCode(t) != "step_up_required" {
			t.Fatalf("%s without step-up: %s", path, r.body)
		}
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/auth/step-up", map[string]any{"code": "000000"}); r.errCode(t) != "invalid_code" {
		t.Fatalf("bad step-up: %s", r.body)
	}
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+target.user.ID.String(), map[string]any{"status": "banned", "status_reason": "Abuse"}, nil)

	// Idle sessions expire (30 min without activity).
	e.clock.Advance(31 * time.Minute)
	if r := admin.call(http.MethodGet, "/api/v1/admin/users", nil); r.status != http.StatusUnauthorized || r.errCode(t) != "session_expired" {
		t.Fatalf("idle session: %d %s", r.status, r.body)
	}
}

func TestAdminUserManagement(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()

	// Create a client account.
	email := uniq("created") + "@example.com"
	var u service.AdminUserView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/users", map[string]any{"email": email, "password": strongPassword,
		"role": "client", "display_name": "Created User", "locale": "fr"}, &u)
	if u.Role != "client" || u.Locale != "fr" || u.Wallet == nil || u.Stats == nil {
		t.Fatalf("created: %+v", u)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/users", map[string]any{"email": email, "password": strongPassword, "role": "client"}); r.errCode(t) != "email_taken" {
		t.Fatalf("duplicate: %s", r.body)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/users", map[string]any{"email": uniq("p") + "@example.com", "password": strongPassword, "role": "psychic"}); r.errCode(t) != "validation_failed" {
		t.Fatalf("psychic via users: %s", r.body)
	}
	created := e.login(email, strongPassword)
	ws := created.connect("")

	// Edit profile fields.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+u.ID.String(), map[string]any{"display_name": "Renamed", "locale": "es"}, &u)
	if u.DisplayName != "Renamed" || u.Locale != "es" {
		t.Fatalf("edit: %+v", u)
	}
	// Admins cannot target themselves.
	if r := admin.call(http.MethodPatch, "/api/v1/admin/users/"+admin.adminID.String(), map[string]any{"status": "deactivated"}); r.status != http.StatusForbidden {
		t.Fatalf("self edit: %d", r.status)
	}

	// Ban: reason required, sessions revoked immediately, login refused.
	if r := admin.call(http.MethodPatch, "/api/v1/admin/users/"+u.ID.String(), map[string]any{"status": "banned"}); r.errCode(t) != "validation_failed" {
		t.Fatalf("ban without reason: %s", r.body)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+u.ID.String(), map[string]any{"status": "banned", "status_reason": "Fraud"}, &u)
	if u.Status != "banned" || u.StatusReason != "Fraud" {
		t.Fatalf("banned: %+v", u)
	}
	ws.waitFor(realtime.EventAuthRevoked, 5*time.Second)
	if r := created.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized && r.status != http.StatusForbidden {
		t.Fatalf("banned session still works: %d", r.status)
	}
	if r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": strongPassword}); r.errCode(t) != "account_disabled" {
		t.Fatalf("banned login: %s", r.body)
	}
	// Unban restores access and clears the reason.
	var unbanned service.AdminUserView
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+u.ID.String(), map[string]any{"status": "active"}, &unbanned)
	if unbanned.Status != "active" || unbanned.StatusReason != "" {
		t.Fatalf("unbanned: %+v", unbanned)
	}
	e.login(email, strongPassword)

	// Filters.
	var page service.Page[service.AdminUserRow]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users?status=active&role=client&q="+u.Email[:10], nil, &page)
	if len(page.Items) != 1 || page.Items[0].ID != u.ID {
		t.Fatalf("filter: %+v", page.Items)
	}

	// Delete: anonymised, cannot sign in, history kept.
	if r := admin.call(http.MethodDelete, "/api/v1/admin/users/"+u.ID.String(), map[string]any{"reason": ""}); r.errCode(t) != "validation_failed" {
		t.Fatalf("delete without reason: %s", r.body)
	}
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+u.ID.String(), map[string]any{"reason": "GDPR request"}, nil)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+u.ID.String(), nil, &u)
	if u.Status != "deleted" || !strings.HasPrefix(u.Email, "deleted+") || u.DisplayName != "Deleted user" || u.DeletedAt == nil {
		t.Fatalf("deleted: %+v", u)
	}
	if u.Profile != nil && (u.Profile.BirthDate != "" || u.Profile.Nickname != "") {
		t.Fatalf("profile not erased: %+v", u.Profile)
	}
	if r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": strongPassword}); r.errCode(t) != "invalid_credentials" {
		t.Fatalf("deleted login: %s", r.body)
	}

	// Creating another administrator requires step-up after the window.
	e.clock.Advance(11 * time.Minute)
	r := admin.call(http.MethodPost, "/api/v1/admin/users", map[string]any{"email": uniq("adm") + "@example.com", "password": strongPassword, "role": "admin"})
	if r.errCode(t) != "step_up_required" {
		t.Fatalf("admin creation without step-up: %s", r.body)
	}
	admin.stepUp()
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/users", map[string]any{"email": uniq("adm") + "@example.com", "password": strongPassword, "role": "admin"}, nil)

	// Audit trail records actions with the actor's email.
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?target_id="+u.ID.String(), nil, &audit)
	actions := map[string]bool{}
	for _, a := range audit.Items {
		actions[a.Action] = true
		if a.ActorEmail == "" {
			t.Fatalf("actor email missing: %+v", a)
		}
	}
	for _, want := range []string{"user.created", "user.updated", "user.banned", "user.reactivated", "user.deleted"} {
		if !actions[want] {
			t.Errorf("audit missing %s (have %v)", want, actions)
		}
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=admin.logged_in", nil, &audit)
	if len(audit.Items) == 0 {
		t.Fatal("login not audited")
	}
}

func TestAdminPsychicManagement(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()

	slugA, slugB := uniq("adva"), uniq("advb")
	var a, b service.AdminPsychicView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics", map[string]any{"email": slugA + "@example.com", "password": strongPassword,
		"name": "Zelda " + slugA, "slug": slugA, "rate_per_minute": 30, "sort_order": 1}, &a)
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics", map[string]any{"email": slugB + "@example.com", "password": strongPassword,
		"name": "Yara " + slugB, "slug": slugB, "rate_per_minute": 30, "sort_order": 2, "featured": true, "featured_rank": 1}, &b)
	if !b.Featured || b.FeaturedRank != 1 || b.Email == "" || b.UserStatus != "active" {
		t.Fatalf("created: %+v", b)
	}

	// Featured advisors come first in the public directory.
	order := func() []string {
		var list struct {
			Items []service.PsychicView `json:"items"`
		}
		e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics", nil, &list)
		var slugs []string
		for _, p := range list.Items {
			if p.Slug == slugA || p.Slug == slugB {
				slugs = append(slugs, p.Slug)
			}
		}
		return slugs
	}
	if got := order(); len(got) != 2 || got[0] != slugB {
		t.Fatalf("featured first: %v", got)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+b.ID.String(), map[string]any{"featured": false}, &b)
	if got := order(); got[0] != slugA {
		t.Fatalf("unfeatured order: %v", got)
	}

	// Hide: gone from the directory and profile.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+a.ID.String(), map[string]any{"listed": false}, &a)
	if got := order(); len(got) != 1 {
		t.Fatalf("hidden advisor listed: %v", got)
	}
	if r := e.client().call(http.MethodGet, "/api/v1/psychics/"+slugA, nil); r.status != http.StatusNotFound {
		t.Fatalf("hidden profile: %d", r.status)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+a.ID.String(), map[string]any{"listed": true}, &a)

	// Ban: also disappears publicly while staying listed.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+a.ID.String(), map[string]any{"status": "banned", "status_reason": "Policy"}, nil)
	if r := e.client().call(http.MethodGet, "/api/v1/psychics/"+slugA, nil); r.status != http.StatusNotFound {
		t.Fatalf("banned profile: %d", r.status)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics/"+a.ID.String(), nil, &a)
	if a.UserStatus != "banned" || !a.Listed {
		t.Fatalf("admin view: %+v", a)
	}
	var filtered struct {
		Items []service.AdminPsychicView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics?status=banned&q="+slugA, nil, &filtered)
	if len(filtered.Items) != 1 {
		t.Fatalf("filter banned: %+v", filtered.Items)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+a.ID.String(), map[string]any{"status": "active"}, nil)

	// Profile image: stored in the advisor bucket, served publicly.
	body, ct := multipartFile(t, "image", "portrait.png", pngBytes(t, 1600, 900))
	r := admin.raw(http.MethodPost, "/api/v1/admin/psychics/"+a.ID.String()+"/image", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": csrfOf(admin)})
	if r.status != http.StatusOK {
		t.Fatalf("upload: %d %s", r.status, r.body)
	}
	r.decode(t, &a)
	if !strings.HasPrefix(a.ImageURL, "https://advisors.test/advisors/"+a.ID.String()+"/") {
		t.Fatalf("image url: %s", a.ImageURL)
	}
	key := strings.TrimPrefix(a.ImageURL, "https://advisors.test/")
	if data, ok := e.advisors.Lookup(key); !ok || !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		t.Fatalf("advisor image missing or not JPEG")
	}
	// Non-images are refused by content, not by name.
	body, ct = multipartFile(t, "image", "evil.png", []byte(strings.Repeat("<script>alert(1)</script>", 10)))
	r = admin.raw(http.MethodPost, "/api/v1/admin/psychics/"+a.ID.String()+"/image", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": csrfOf(admin)})
	if r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("non-image: %d %s", r.status, r.body)
	}
	// Replacing deletes the old object; removing clears the URL.
	body, ct = multipartFile(t, "image", "second.png", pngBytes(t, 400, 400))
	r = admin.raw(http.MethodPost, "/api/v1/admin/psychics/"+a.ID.String()+"/image", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": csrfOf(admin)})
	if r.status != http.StatusOK {
		t.Fatalf("second upload: %d %s", r.status, r.body)
	}
	if _, ok := e.advisors.Lookup(key); ok {
		t.Fatal("previous image not deleted")
	}
	admin.must(http.StatusOK, http.MethodDelete, "/api/v1/admin/psychics/"+a.ID.String()+"/image", nil, &a)
	if a.ImageURL != "" {
		t.Fatalf("image not removed: %s", a.ImageURL)
	}

	// Services: delete works when unused.
	var svc service.ServiceView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics/"+a.ID.String()+"/services", map[string]any{"name": "Tarot", "price_credits": 100}, &svc)
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/services/"+svc.ID.String(), nil, nil)
	if r := admin.call(http.MethodDelete, "/api/v1/admin/services/"+svc.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("service deleted twice: %d", r.status)
	}

	// Credit packs: unused packs can be deleted.
	var pack service.PackView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/credit-packs", map[string]any{"name": "Temp", "credits": 77, "price_cents": 777}, &pack)
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/credit-packs/"+pack.ID.String(), nil, nil)

	// Soft delete an advisor.
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/psychics/"+b.ID.String(), map[string]any{"reason": "Left the platform"}, nil)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics/"+b.ID.String(), nil, &b)
	if b.UserStatus != "deleted" || b.Listed {
		t.Fatalf("deleted advisor: %+v", b)
	}
	if r := e.client().call(http.MethodGet, "/api/v1/psychics/"+slugB, nil); r.status != http.StatusNotFound {
		t.Fatalf("deleted advisor public: %d", r.status)
	}
}

func TestAdminPackInUseCannotBeDeleted(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	admin := e.createAdmin()
	c := e.registerClient()
	c.buyCredits(250)
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		if p.Credits == 250 {
			if r := admin.call(http.MethodDelete, "/api/v1/admin/credit-packs/"+p.ID.String(), nil); r.status != http.StatusConflict {
				t.Fatalf("pack in use deleted: %d %s", r.status, r.body)
			}
			return
		}
	}
	t.Fatal("pack not found")
}

func TestAdminPresenceListsOnlineUsers(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	c := e.registerClient()
	w := c.connect("")
	defer w.conn.Close()
	var p service.PresenceView
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/presence?role=client", nil, &p)
		for _, u := range p.Users {
			if u.ID == c.user.ID {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := json.Marshal(p)
	t.Fatalf("online client not listed: %s", b)
}

// csrfOf returns a client's CSRF token, fetching one when needed.
func csrfOf(c *client) string {
	c.ensureCSRF()
	return c.csrf
}
