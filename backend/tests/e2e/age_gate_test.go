package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: 18+ age gate (§24.6).

// registerClientWithoutBirth signs up like the web app's first sign-up step:
// no date of birth (the birth step comes next).
func (e *env) registerClientWithoutBirth() *client {
	e.t.Helper()
	c := e.client()
	var out struct {
		User *service.UserView `json:"user"`
	}
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("nobirth") + "@example.com", "password": strongPassword, "accept_terms": true, "locale": "en",
	}, &out)
	c.user, c.pw = out.User, strongPassword
	return c
}

func (c *client) ageStatus() string {
	c.e.t.Helper()
	var me service.UserView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	return string(me.AgeStatus)
}

// refusedWith asserts a 403 with the given age code.
func refusedWith(t *testing.T, r response, code string) {
	t.Helper()
	if r.status != http.StatusForbidden || r.errCode(t) != code {
		t.Fatalf("want 403 %s, got %d %s", code, r.status, r.body)
	}
}

func (c *client) startChatProbe() response {
	return c.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": "nobody", "request_key": uniq("req-key-")})
}

func TestRegisterWithoutBirthDateNeedsOne(t *testing.T) {
	e := newEnv(t)
	var out struct {
		User *service.UserView `json:"user"`
	}
	c := e.client()
	r := c.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("nb") + "@example.com", "password": strongPassword, "accept_terms": true, "locale": "en"}, &out)
	if out.User.AgeStatus != domain.AgeBirthDateRequired {
		t.Fatalf("register: age_status %q (%s)", out.User.AgeStatus, r.body)
	}
	c.user = out.User
	if got := c.ageStatus(); got != "birth_date_required" {
		t.Fatalf("/me: %s", got)
	}

	// The gated action answers 403 birth_date_required with the next path.
	rr := c.startChatProbe()
	refusedWith(t, rr, "birth_date_required")
	var body struct {
		Error struct {
			Details map[string]any `json:"details"`
			Message string         `json:"message"`
		} `json:"error"`
	}
	rr.decode(t, &body)
	if body.Error.Details["next"] != "/signup/birth" {
		t.Fatalf("details: %v", body.Error.Details)
	}

	// Sign-in and /me carry age_status.
	var login struct {
		User *service.UserView `json:"user"`
	}
	c2 := e.client()
	c2.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": out.User.Email, "password": strongPassword}, &login)
	if login.User.AgeStatus != domain.AgeBirthDateRequired {
		t.Fatalf("login: %+v", login.User)
	}
	// POST /auth/refresh returns no user; the web app re-reads /me, which carries it.
	c2.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	if got := c2.ageStatus(); got != "birth_date_required" {
		t.Fatalf("after refresh: %s", got)
	}

	// Never gated: profile and settings, wallet, notifications, support,
	// export and closure endpoints (those need a password, but pass the gate).
	for _, p := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me/settings"}, {http.MethodGet, "/api/v1/wallet"}, {http.MethodGet, "/api/v1/me/notifications"},
		{http.MethodGet, "/api/v1/me/support/tickets"}, {http.MethodGet, "/api/v1/me/exports"}, {http.MethodGet, "/api/v1/chat/unread-count"},
		{http.MethodGet, "/api/v1/credits/purchases"}, {http.MethodGet, "/api/v1/billing/payment-methods"},
	} {
		if r := c.call(p.method, p.path, nil); r.status != http.StatusOK {
			t.Errorf("%s %s must stay open: %d %s", p.method, p.path, r.status, r.body)
		}
	}
	if r := c.call(http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": "wrong"}); r.errCode(t) == "birth_date_required" {
		t.Errorf("deactivation must stay open: %s", r.body)
	}
	if r := c.call(http.MethodPost, "/api/v1/support/tickets", map[string]any{"topic": "general", "subject": "Hello there", "email": "x@example.com",
		"message": "I need some help please.", "consent": true}); r.status != http.StatusCreated {
		t.Errorf("support must stay open: %d %s", r.status, r.body)
	}
}

func TestAdultBirthDateOpensTheGate(t *testing.T) {
	e := newEnv(t)
	c := e.registerClientWithoutBirth()
	refusedWith(t, c.startChatProbe(), "birth_date_required")

	var me service.UserView
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": "1990-05-17"}, &me)
	if me.AgeStatus != domain.AgeAdult {
		t.Fatalf("after the adult date: %+v", me)
	}
	// The cached authorization state was refreshed at once: the very next
	// call passes the gate (it fails later for other reasons: no such advisor).
	if r := c.startChatProbe(); r.status == http.StatusForbidden {
		t.Fatalf("gate still closed: %s", r.body)
	}
	var row domain.User
	if err := e.app.DB.First(&row, "id = ?", c.user.ID).Error; err != nil || row.AdultConfirmedAt == nil || row.AgeRestrictedAt != nil {
		t.Fatalf("row: %+v %v", row, err)
	}
	// The settings route saves it the same way.
	c2 := e.registerClientWithoutBirth()
	c2.must(http.StatusOK, http.MethodPatch, "/api/v1/me/settings", map[string]any{"profile": map[string]any{"birth_date": "1985-01-01"}}, nil)
	if got := c2.ageStatus(); got != "adult" {
		t.Fatalf("settings route: %s", got)
	}
}

func TestRegisterWithAdultBirthDateIsAdultAndUnderAgeIsRefused(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	if c.user.AgeStatus != domain.AgeAdult {
		t.Fatalf("register with a date: %+v", c.user)
	}
	// Unchanged: sign-up with an under-age date is a 422 and creates nothing.
	email := uniq("kid") + "@example.com"
	r := e.client().call(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": email, "password": strongPassword, "accept_terms": true, "birth_date": time.Now().AddDate(-15, 0, 0).Format("2006-01-02")})
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("under-age register: %d %s", r.status, r.body)
	}
	var n int64
	e.app.DB.Model(&domain.User{}).Where("email = ?", email).Count(&n)
	if n != 0 {
		t.Fatalf("an under-age registration created %d accounts", n)
	}
}

func TestUnderAgeBirthDateRestrictsForGood(t *testing.T) {
	e := newEnv(t)
	for name, patch := range map[string]func(c *client, date string) response{
		"profile": func(c *client, d string) response {
			return c.call(http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": d})
		},
		"settings": func(c *client, d string) response {
			return c.call(http.MethodPatch, "/api/v1/me/settings", map[string]any{"profile": map[string]any{"birth_date": d}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := e.registerClientWithoutBirth()
			under := time.Now().AddDate(-16, 0, 0).Format("2006-01-02")
			r := patch(c, under)
			if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), "birth_date") {
				t.Fatalf("under-age date: %d %s", r.status, r.body)
			}
			// The refusal still persisted the restriction.
			if got := c.ageStatus(); got != "restricted" {
				t.Fatalf("status after the refusal: %s", got)
			}
			refusedWith(t, c.startChatProbe(), "age_restricted")
			// No retry with another date.
			refusedWith(t, patch(c, "1990-05-17"), "age_restricted")
			if got := c.ageStatus(); got != "restricted" {
				t.Fatalf("a retry must not lift it: %s", got)
			}
			// Other settings still save, and login reports the status.
			c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/settings", map[string]any{"profile": map[string]any{"nickname": "Moon"}}, nil)
			var login struct {
				User *service.UserView `json:"user"`
			}
			e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": c.user.Email, "password": strongPassword}, &login)
			if login.User.AgeStatus != domain.AgeRestricted {
				t.Fatalf("login: %+v", login.User)
			}
		})
	}
}

func TestAgeReviewByAdmin(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	c := e.registerClientWithoutBirth()
	under := time.Now().AddDate(-16, 0, 0).Format("2006-01-02")
	c.call(http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": under})
	if got := c.ageStatus(); got != "restricted" {
		t.Fatalf("status: %s", got)
	}
	path := "/api/v1/admin/users/" + c.user.ID.String() + "/age-review"

	// Admin views show age_status.
	var detail service.AdminUserView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+c.user.ID.String(), nil, &detail)
	if detail.AgeStatus != domain.AgeRestricted {
		t.Fatalf("admin detail: %+v", detail.AdminUserRow)
	}
	var list struct {
		Items []service.AdminUserRow `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users?q="+c.user.Email, nil, &list)
	if len(list.Items) != 1 || list.Items[0].AgeStatus != domain.AgeRestricted {
		t.Fatalf("admin list: %+v", list.Items)
	}

	// Step-up is required; validation comes after it.
	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodPost, path, map[string]any{"decision": "clear", "reason": "Entered the wrong date"}); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	for name, body := range map[string]map[string]any{
		"bad decision": {"decision": "forgive", "reason": "Entered the wrong date"},
		"short reason": {"decision": "clear", "reason": "no"},
		"long reason":  {"decision": "clear", "reason": strings.Repeat("x", 501)},
		"no reason":    {"decision": "clear"},
	} {
		if r := admin.call(http.MethodPost, path, body); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
	// Advisors and administrators are not reviewed.
	psy := e.createPsychic(5)
	if r := admin.call(http.MethodPost, "/api/v1/admin/users/"+psy.user.ID.String()+"/age-review", map[string]any{"decision": "restrict", "reason": "Just checking"}); r.status != http.StatusUnprocessableEntity {
		t.Errorf("advisor review: %d %s", r.status, r.body)
	}

	// Clear: back to birth_date_required (not adult), and the under-age date is gone.
	var out service.AdminUserView
	admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"decision": "clear", "reason": "Entered the wrong date"}, &out)
	if out.AgeStatus != domain.AgeBirthDateRequired {
		t.Fatalf("after clear: %+v", out.AdminUserRow)
	}
	if got := c.ageStatus(); got != "birth_date_required" {
		t.Fatalf("client sees: %s", got)
	}
	refusedWith(t, c.startChatProbe(), "birth_date_required")
	var profile string
	e.app.DB.Raw("SELECT coalesce(birth_date::text, '') FROM user_profiles WHERE user_id = ?", c.user.ID).Scan(&profile)
	if profile != "" {
		t.Fatalf("the under-age date must be dropped: %q", profile)
	}
	// The person enters an adult date and is through.
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": "1992-02-02"}, nil)
	if got := c.ageStatus(); got != "adult" {
		t.Fatalf("after re-entering: %s", got)
	}

	// Restrict by hand: no more client actions; audit trail for both.
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"decision": "restrict", "reason": "Reported as a minor"}, &out)
	if out.AgeStatus != domain.AgeRestricted {
		t.Fatalf("after restrict: %+v", out.AdminUserRow)
	}
	refusedWith(t, c.startChatProbe(), "age_restricted")
	var audits []struct{ Action, Metadata string }
	e.app.DB.Table("audit_logs").Select("action, metadata::text AS metadata").
		Where("action = 'user.age_review' AND target_id = ?", c.user.ID.String()).Order("created_at").Scan(&audits)
	if len(audits) != 2 || !strings.Contains(audits[0].Metadata, `"decision": "clear"`) || !strings.Contains(audits[1].Metadata, `"decision": "restrict"`) ||
		!strings.Contains(audits[1].Metadata, "Reported as a minor") {
		t.Fatalf("audit: %+v", audits)
	}
}

func TestAdvisorsAndAdminsAreAlwaysAdult(t *testing.T) {
	e := newEnv(t)
	psy := e.createPsychic(5)
	if psy.user.AgeStatus != domain.AgeAdult {
		t.Fatalf("advisor: %+v", psy.user)
	}
	if got := psy.ageStatus(); got != "adult" {
		t.Fatalf("advisor /me: %s", got)
	}
	// An advisor passes the gate on a shared chat route (and fails later on its merits).
	if r := psy.call(http.MethodGet, "/api/v1/chat/sessions", nil); r.status != http.StatusOK {
		t.Fatalf("advisor chat list: %d %s", r.status, r.body)
	}
}

func TestGoogleSignUpNeedsABirthDate(t *testing.T) {
	e, f := newGoogleEnv(t)
	c, _ := e.googleAccount(f, uniq("gage")+"@example.com")
	if c.user.AgeStatus != domain.AgeBirthDateRequired {
		t.Fatalf("google sign-up: %+v", c.user)
	}
	refusedWith(t, c.startChatProbe(), "birth_date_required")
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": "1991-03-03"}, nil)
	if got := c.ageStatus(); got != "adult" {
		t.Fatalf("after the birth step: %s", got)
	}
}

func TestWebSocketActionsAreGated(t *testing.T) {
	e := newEnv(t)
	c := e.registerClientWithoutBirth()
	ws := c.connect("") // connecting is allowed (presence, wallet and notification events)
	frames := map[string]any{
		realtime.FrameSessionJoin: map[string]any{"session_id": uuid.New()},
		realtime.FrameMessageSend: map[string]any{"conversation_id": uuid.New(), "body": "hello"},
		realtime.FrameTyping:      map[string]any{"session_id": uuid.New(), "typing": true},
		realtime.FrameMessageRead: map[string]any{"conversation_id": uuid.New()},
	}
	for typ, data := range frames {
		ev := ws.request(typ, data)
		if ev.Type != realtime.EventError || ev.Err == nil || ev.Err.Code != "birth_date_required" || ev.Err.Details["next"] != "/signup/birth" {
			t.Fatalf("%s without a date: %+v", typ, ev)
		}
	}
	// Leaving is always possible (cleanup of a session left open).
	if ev := ws.request(realtime.FrameSessionLeave, map[string]any{"session_id": uuid.New()}); ev.Err != nil && strings.HasPrefix(ev.Err.Code, "birth_date") {
		t.Fatalf("leave must not be gated: %+v", ev)
	}

	// The same open connection: once the date is saved the frames pass the
	// gate (they fail on their merits: the session does not exist).
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": "1990-05-17"}, nil)
	for typ, data := range frames {
		if ev := ws.request(typ, data); ev.Err != nil && (ev.Err.Code == "birth_date_required" || ev.Err.Code == "age_restricted") {
			t.Fatalf("%s as an adult: %+v", typ, ev.Err)
		}
	}

	// An administrator restricting the account closes the gate on the live socket.
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/users/"+c.user.ID.String()+"/age-review",
		map[string]any{"decision": "restrict", "reason": "Reported as a minor"}, nil)
	// Restricting revokes the realtime connection (auth.revoked); a fresh one
	// is refused the same way.
	_ = ws.conn.Close()
	ws2 := c.connect("")
	ev := ws2.request(realtime.FrameMessageSend, map[string]any{"conversation_id": uuid.New(), "body": "hello"})
	if ev.Err == nil || ev.Err.Code != "age_restricted" {
		t.Fatalf("restricted: %+v", ev)
	}
}

// With the status cache warm, the gate costs no database query at all: a
// refused request touches PostgreSQL zero times.
func TestAgeGateRunsWithoutAQuery(t *testing.T) {
	e := newEnv(t)
	c := e.registerClientWithoutBirth()
	refusedWith(t, c.startChatProbe(), "birth_date_required") // warms the cached status
	log := recordSQL(e, func() {
		for i := 0; i < 5; i++ {
			refusedWith(t, c.startChatProbe(), "birth_date_required")
		}
	})
	if n := len(log.stmt); n != 0 {
		t.Fatalf("5 gated requests ran %d statements: %v", n, log.stmt)
	}
	// Writing the date invalidates the cache: the next request reloads the status once.
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": "1990-05-17"}, nil)
	log = recordSQL(e, func() {
		if r := c.startChatProbe(); r.status == http.StatusForbidden {
			t.Fatalf("gate still closed: %s", r.body)
		}
	})
	if n := log.count(`FROM "users"`); n < 1 {
		t.Fatalf("expected the status to be reloaded after the write: %v", log.stmt)
	}
}
