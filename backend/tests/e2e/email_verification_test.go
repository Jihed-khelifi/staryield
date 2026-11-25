package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Growth: e-mail verification by secure link, announced live to every tab.

type verifyMe struct {
	EmailVerified bool `json:"email_verified"`
}

// skipWelcomeMail delivers the welcome e-mail every password sign-up queues
// (its verification link) and clears the fake provider, for tests that count
// the e-mails of a scenario that starts after the sign-up.
func (e *env) skipWelcomeMail() {
	e.t.Helper()
	e.flushMail()
	e.mail.Reset()
}

func verificationLink(t *testing.T, e *env, to string) (link, token string) {
	t.Helper()
	m := mailTo(t, e.flushMail(), to, "/verify-email#token=")
	if m.Category != "security" {
		t.Fatalf("category %q", m.Category)
	}
	i := strings.Index(m.Text, "/verify-email#token=")
	link = strings.Fields(m.Text[i:])[0]
	return link, strings.TrimPrefix(link, "/verify-email#token=")
}

func TestEmailVerificationFlow(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	live := c.connect("")

	// The welcome e-mail carries the link in the fragment; the account is not verified yet.
	_, token := verificationLink(t, e, c.user.Email)
	if len(token) < 40 || strings.ContainsAny(token, "?&\"<") {
		t.Fatalf("unexpected token %q", token)
	}
	var me verifyMe
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.EmailVerified {
		t.Fatal("a new password account must start unverified")
	}
	var stored int64
	e.app.DB.Raw("SELECT count(*) FROM user_action_tokens WHERE purpose = 'email_verify' AND token_hash = ?", token).Scan(&stored)
	if stored != 0 {
		t.Fatal("the raw token must never be stored")
	}

	// A failed attempt announces nothing.
	anon := e.client()
	if r := anon.call(http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": "not-a-token"}); r.errCode(t) != "invalid_token" {
		t.Fatalf("bad token: %d %s", r.status, r.body)
	}
	live.expectNone(realtime.EventEmailVerified, 400*time.Millisecond)

	// Another device (no session) opens the link: every live tab of the user hears it.
	var res service.VerificationResult
	anon.must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}, &res)
	if res.Status != "verified" || !res.EmailVerified {
		t.Fatalf("verify: %+v", res)
	}
	ev := live.waitFor(realtime.EventEmailVerified, 5*time.Second)
	if !strings.Contains(string(ev.Data), `"email_verified":true`) || !strings.Contains(string(ev.Data), "email_verified_at") {
		t.Fatalf("event: %s", ev.Data)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if !me.EmailVerified {
		t.Fatal("/me must report the verified address")
	}
	if n := e.auditCount("account.email_verified", c.user.ID); n != 1 {
		t.Fatalf("audit rows: %d", n)
	}
	var notes struct {
		Items []struct {
			Kind string `json:"kind"`
			Link string `json:"link"`
		} `json:"items"`
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &notes)
		if len(notes.Items) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	var found bool
	for _, n := range notes.Items {
		found = found || n.Kind == "account.email_verified" && n.Link == "/profile/settings#account"
	}
	if !found {
		t.Fatalf("in-app notification missing: %+v", notes.Items)
	}

	// The token is single use; a verified address cannot ask for another link.
	if r := anon.call(http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}); r.errCode(t) != "invalid_token" {
		t.Fatalf("reuse: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodPost, "/api/v1/me/email/verification", nil); r.status != http.StatusConflict || r.errCode(t) != "already_verified" {
		t.Fatalf("resend when verified: %d %s", r.status, r.body)
	}
}

func TestEmailVerificationResend(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	_, first := verificationLink(t, e, c.user.Email)

	// Inside the cooldown: 429 with Retry-After and no e-mail.
	before := len(e.mailTo(c.user.Email))
	r := c.call(http.MethodPost, "/api/v1/me/email/verification", nil)
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("cooldown: %d %s", r.status, r.body)
	}
	if n := len(e.mailTo(c.user.Email)); n != before {
		t.Fatalf("an e-mail was sent inside the cooldown (%d -> %d)", before, n)
	}

	e.clock.Advance(61 * time.Second)
	var sent service.VerificationSent
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/email/verification", nil, &sent)
	if sent.Status != "sent" || sent.Email == "" || strings.Contains(sent.Email, c.user.Email) || !sent.ResendAvailableAt.After(e.clock.Now()) {
		t.Fatalf("answer: %+v", sent)
	}
	_, second := verificationLink(t, e, c.user.Email)
	if second == first {
		t.Fatal("a resend must issue a new token")
	}
	anon := e.client()
	if r := anon.call(http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": first}); r.errCode(t) != "invalid_token" {
		t.Fatalf("the replaced link must stop working: %d %s", r.status, r.body)
	}
	anon.must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": second}, nil)

	// Signed-out callers cannot resend.
	if r := e.client().call(http.MethodPost, "/api/v1/me/email/verification", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous resend: %d", r.status)
	}
}

func TestEmailVerificationLinkExpires(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	_, token := verificationLink(t, e, c.user.Email)
	e.clock.Advance(48*time.Hour + time.Minute)
	if r := e.client().call(http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}); r.errCode(t) != "invalid_token" {
		t.Fatalf("expired link: %d %s", r.status, r.body)
	}
}

func TestEmailVerificationDoesNotApplyToGoogleOrChangedAddress(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	_, token := verificationLink(t, e, c.user.Email)
	// The address changes before the link is used: the old link is dead.
	newEmail := uniq("moved") + "@example.com"
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/email-change", map[string]any{"email": newEmail, "password": strongPassword}, nil)
	confirm := mailTo(t, e.flushMail(), newEmail, "/confirm-email?token=")
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email-change/confirm", map[string]any{"token": tokenIn(t, confirm)}, nil)
	if r := e.client().call(http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}); r.errCode(t) != "invalid_token" {
		t.Fatalf("link for the previous address: %d %s", r.status, r.body)
	}
	// The change itself verified the new address (it received a link).
	moved := e.login(newEmail, strongPassword)
	var me verifyMe
	moved.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if !me.EmailVerified {
		t.Fatal("an address confirmed by an e-mail change is verified")
	}
}

func TestEmailVerificationAdmin(t *testing.T) {
	e := newEnv(t)
	adm := e.createAdmin()
	unverified := e.registerClient()
	verified := e.registerClient()
	_, tok := verificationLink(t, e, verified.user.Email)
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": tok}, nil)

	type row struct {
		ID              string  `json:"id"`
		EmailVerifiedAt *string `json:"email_verified_at"`
	}
	list := func(q string) map[string]row {
		var page struct {
			Items []row `json:"items"`
		}
		adm.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users?limit=200&role=client"+q, nil, &page)
		out := map[string]row{}
		for _, r := range page.Items {
			out[r.ID] = r
		}
		return out
	}
	notVerified := list("&verified=false")
	if _, ok := notVerified[unverified.user.ID.String()]; !ok || notVerified[unverified.user.ID.String()].EmailVerifiedAt != nil {
		t.Fatal("the unverified account must be in the verified=false list")
	}
	if _, ok := notVerified[verified.user.ID.String()]; ok {
		t.Fatal("a verified account in the verified=false list")
	}
	yes := list("&verified=true")
	if r, ok := yes[verified.user.ID.String()]; !ok || r.EmailVerifiedAt == nil {
		t.Fatal("the verified account must be in the verified=true list with its date")
	}
	if _, ok := yes[unverified.user.ID.String()]; ok {
		t.Fatal("an unverified account in the verified=true list")
	}
	if r := adm.call(http.MethodGet, "/api/v1/admin/users?verified=maybe", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad filter: %d %s", r.status, r.body)
	}

	// Detail shows the status; resend is audited; the cooldown and 409 apply.
	var detail row
	adm.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+verified.user.ID.String(), nil, &detail)
	if detail.EmailVerifiedAt == nil {
		t.Fatal("the detail must carry email_verified_at")
	}
	if r := adm.call(http.MethodPost, "/api/v1/admin/users/"+verified.user.ID.String()+"/email-verification", nil); r.status != http.StatusConflict || r.errCode(t) != "already_verified" {
		t.Fatalf("resend for a verified user: %d %s", r.status, r.body)
	}
	// The welcome link was just sent: the cooldown applies to the console too.
	if r := adm.call(http.MethodPost, "/api/v1/admin/users/"+unverified.user.ID.String()+"/email-verification", nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("resend inside the cooldown: %d %s", r.status, r.body)
	}
	e.clock.Advance(61 * time.Second)
	adm.must(http.StatusAccepted, http.MethodPost, "/api/v1/admin/users/"+unverified.user.ID.String()+"/email-verification", nil, nil)
	// The new link works like the welcome one.
	_, tok = verificationLink(t, e, unverified.user.Email)
	if e.auditCount("admin.email_verification_resent", unverified.user.ID) != 1 {
		t.Fatal("the admin resend must be audited")
	}
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": tok}, nil)
	adm.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+unverified.user.ID.String(), nil, &detail)
	if detail.EmailVerifiedAt == nil {
		t.Fatal("the detail must show the new verification")
	}
	// Unknown users are a 404.
	if r := adm.call(http.MethodPost, "/api/v1/admin/users/"+uuid.NewString()+"/email-verification", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown user: %d %s", r.status, r.body)
	}
}

func TestEmailVerificationForAdvisors(t *testing.T) {
	e := newEnv(t)
	psy := e.createPsychic(40)
	// Advisors are created by the team: no welcome link, but they may ask for one.
	for _, m := range e.mailTo(psy.user.Email) {
		if strings.Contains(m.Text, "/verify-email#token=") {
			t.Fatal("an advisor created by the team must not get the welcome link")
		}
	}
	var me verifyMe
	psy.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.EmailVerified {
		t.Fatal("an advisor account starts unverified")
	}
	var u domain.User
	e.app.DB.First(&u, "id = ?", psy.user.ID)
	if u.Role != domain.RolePsychic {
		t.Fatalf("role %s", u.Role)
	}
	psy.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/email/verification", nil, nil)
	_, token := verificationLink(t, e, psy.user.Email)
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}, nil)
	psy.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if !me.EmailVerified {
		t.Fatal("the advisor's address must be verified")
	}
}

// The OnVerified hooks (referral rewards) and the realtime event only run once
// the verification committed: another connection already sees the address
// verified when they run.
func TestEmailVerificationHooksRunAfterCommit(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	_, token := verificationLink(t, e, c.user.Email)
	type seen struct {
		user     uuid.UUID
		verified bool
	}
	got := make(chan seen, 1)
	e.app.Services.EmailVerification.OnVerified(func(ctx context.Context, userID uuid.UUID) {
		var at *time.Time
		// e.app.DB is not the verification's transaction: it only sees committed rows.
		e.app.DB.Raw("SELECT email_verified_at FROM users WHERE id = ?", userID).Scan(&at)
		got <- seen{user: userID, verified: at != nil}
	})
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}, nil)
	select {
	case s := <-got:
		if s.user != c.user.ID || !s.verified {
			t.Fatalf("the hook ran before the commit or for another user: %+v", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the OnVerified hook did not run")
	}
	// A second opening of the same link is refused and runs no hook.
	if r := e.client().call(http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": token}); r.errCode(t) != "invalid_token" {
		t.Fatalf("reuse: %d %s", r.status, r.body)
	}
	select {
	case s := <-got:
		t.Fatalf("a hook ran for a refused link: %+v", s)
	case <-time.After(300 * time.Millisecond):
	}
}
