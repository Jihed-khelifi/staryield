package e2e

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// revisions 5.0: 18+ age gate (§24.6).
//
// This test walks the router's route table. Every route under /api/v1 that is
// not an admin route must either answer a client without an adult date of
// birth with 403 birth_date_required (and a restricted client with 403
// age_restricted), or appear in ageExempt with the reason it is never gated.
// A new route that is neither fails here, so a future client action cannot
// silently skip the gate. Exempt entries that match no route fail too, so the
// list cannot rot.

type ageExemption struct {
	// Route is "METHOD /path" (exact, as registered) or "* /prefix" for every
	// route whose path starts with the prefix.
	Route  string
	Reason string
}

// agePending are entries for routes another change adds: they may match
// nothing yet; every other entry must match a registered route. (Empty: the
// refund routes below are registered and final.)
var agePending = map[string]bool{}

var ageExempt = []ageExemption{
	// Sign-in, sign-up and the like: nobody is signed in yet, or may not be.
	{"* /api/v1/auth/", "authentication, sign-up, sign-in methods and reactivation (never gated)"},
	{"* /api/v1/webhooks/", "provider webhooks (no principal)"},
	{"* /api/v1/analytics/", "anonymous analytics ingestion"},
	{"POST /api/v1/client-logs", "anonymous client error logging"},
	{"GET /api/v1/time", "public"},
	{"GET /api/v1/site/config", "public site configuration"},
	{"GET /api/v1/consent-policy", "public"},
	{"GET /api/v1/push/config", "public VAPID key"},
	{"GET /api/v1/credit-packs", "public price list (the gated top-up catalogue is under /credits)"},
	{"GET /api/v1/pricing", "public pricing page data, no account needed (revisions 5.0, §24.1)"},
	{"GET /api/v1/billing/plans", "public membership plans"},
	{"* /api/v1/astrology/", "public calculators (no account needed)"},
	{"* /api/v1/aztro", "public horoscope"},
	{"GET /api/v1/psychics", "public directory"},
	{"GET /api/v1/psychics/featured", "public directory"},
	{"GET /api/v1/psychics/:slug", "public directory"},
	{"GET /api/v1/psychics/:slug/reviews", "public reviews"},
	{"DELETE /api/v1/psychics/:slug/waitlist", "leaving a waiting list stays possible"},
	{"POST /api/v1/support/tickets", "support (never gated)"},
	{"* /api/v1/support/tickets/", "support by private link (never gated)"},

	// The signed-in person's own data and rights.
	{"* /api/v1/me", "profile, birth data, settings, notifications, support, devices, exports, deactivation, closure (never gated: privacy rights)"},
	{"GET /api/v1/custom-requests", "reading requests (the profile overview lists them), already placed"},
	{"GET /api/v1/custom-requests/:id", "reading a request already placed"},
	{"POST /api/v1/custom-requests/:id/cancel", "cancelling a request refunds the held credits"},
	{"POST /api/v1/custom-requests/:id/dispute", "a dispute about a request already delivered"},
	{"POST /api/v1/custom-requests/:id/accept", "advisor-only (RequireRole psychic)"},
	{"POST /api/v1/custom-requests/:id/decline", "advisor-only (RequireRole psychic)"},
	{"POST /api/v1/custom-requests/:id/deliver", "advisor-only (RequireRole psychic)"},
	{"GET /api/v1/wallet", "balance and history are the person's own data"},
	{"GET /api/v1/wallet/ledger", "balance and history are the person's own data"},
	{"GET /api/v1/credits/purchases", "purchase history and receipts stay available (refunds)"},
	{"* /api/v1/billing/refund-requests", "refund requests (§24.4) stay available to everyone: list, create, withdraw"},
	{"GET /api/v1/billing/refund-quote", "what a payment can be refunded for (§24.4) stays available to everyone"},
	{"GET /api/v1/billing/client-config", "read-only provider configuration"},
	{"GET /api/v1/billing/payment-methods", "read-only: saved cards"},
	{"DELETE /api/v1/billing/payment-methods/:id", "removing a saved card must stay possible"},
	{"POST /api/v1/billing/payment-methods/:id/default", "no money moves: choosing which saved card is the default"},
	{"GET /api/v1/billing/subscription", "read-only"},
	{"POST /api/v1/billing/subscription/cancel", "stopping a charge must stay possible"},
	{"GET /api/v1/billing/transactions", "read-only history"},

	// Connection plumbing; the actions are gated where they happen.
	{"POST /api/v1/realtime/ticket", "connection ticket (the WebSocket's client frames are gated in the gateway, TestWebSocketActionsAreGated)"},
	{"GET /api/v1/realtime/ws", "WebSocket handshake (the ticket authenticates; client frames are gated)"},
	{"GET /api/v1/realtime/stream", "SSE event stream (intents go over gated REST)"},
	{"GET /api/v1/chat/unread-count", "polled by the app shell for every signed-in user"},

	// Not reachable by a client.
	{"* /api/v1/psychic/", "advisor-only routes (RequireRole psychic)"},
}

func (x ageExemption) matches(method, path string) bool {
	m, p, _ := strings.Cut(x.Route, " ")
	if m != "*" && m != method {
		return false
	}
	if m == "*" {
		return strings.HasPrefix(path, p)
	}
	return path == p
}

func exempt(method, path string) (ageExemption, bool) {
	for _, x := range ageExempt {
		if x.matches(method, path) {
			return x, true
		}
	}
	return ageExemption{}, false
}

var httpMethods = map[string]bool{http.MethodGet: true, http.MethodPost: true, http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true}

func fillParams(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		switch {
		case p == "*":
			parts[i] = "x"
		case strings.HasPrefix(p, ":"):
			if p == ":slug" || p == ":provider" || p == ":bucket" || p == ":sign" || p == ":code" || p == ":reference" {
				parts[i] = "x"
			} else {
				parts[i] = uuid.NewString()
			}
		}
	}
	return strings.Join(parts, "/")
}

func TestEveryClientRouteIsGatedOrExplicitlyExempt(t *testing.T) {
	e := newEnv(t)
	r := e.app.Router.(*echo.Echo)

	noDate := e.registerClientWithoutBirth()
	restricted := e.registerClientWithoutBirth()
	restricted.call(http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_date": "2020-01-01"})
	if got := restricted.ageStatus(); got != "restricted" {
		t.Fatalf("setup: %s", got)
	}

	used := map[int]bool{}
	var gated, skipped, failures []string
	for _, rt := range r.Routes() {
		if !httpMethods[rt.Method] || !strings.HasPrefix(rt.Path, "/api/v1/") || strings.HasPrefix(rt.Path, "/api/v1/admin") {
			continue
		}
		key := rt.Method + " " + rt.Path
		if x, ok := exempt(rt.Method, rt.Path); ok {
			for i := range ageExempt {
				if ageExempt[i] == x {
					used[i] = true
				}
			}
			skipped = append(skipped, key)
			continue
		}
		path := fillParams(rt.Path)
		if got := noDate.call(rt.Method, path, nil); got.status != http.StatusForbidden || got.errCode(t) != "birth_date_required" {
			failures = append(failures, key+": a client without a date got "+http.StatusText(got.status)+" "+string(got.body))
			continue
		}
		if got := restricted.call(rt.Method, path, nil); got.status != http.StatusForbidden || got.errCode(t) != "age_restricted" {
			failures = append(failures, key+": a restricted client got "+http.StatusText(got.status)+" "+string(got.body))
			continue
		}
		gated = append(gated, key)
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("these client routes are neither age-gated nor listed in ageExempt (add mw.AgeGate() to the route, or exempt it with a reason):\n  %s",
			strings.Join(failures, "\n  "))
	}
	for i, x := range ageExempt {
		if !used[i] && !agePending[x.Route] {
			t.Errorf("ageExempt entry %q matches no route (remove it): %s", x.Route, x.Reason)
		}
	}
	// Floor: the routes the contract names must be among the gated ones.
	have := map[string]bool{}
	for _, g := range gated {
		have[g] = true
	}
	for _, want := range []string{
		"POST /api/v1/chat/sessions", "POST /api/v1/chat/sessions/:id/messages", "POST /api/v1/chat/conversations/:id/messages",
		"GET /api/v1/chat/conversations", "POST /api/v1/chat/sessions/:id/review", "POST /api/v1/psychics/:slug/waitlist",
		"POST /api/v1/custom-requests", "POST /api/v1/credits/purchases", "POST /api/v1/billing/payment-methods",
		"POST /api/v1/billing/subscriptions", "PATCH /api/v1/billing/subscription", "POST /api/v1/realtime/presence",
	} {
		if !have[want] {
			t.Errorf("%s must be gated", want)
		}
	}
	t.Logf("%d routes gated, %d exempt", len(gated), len(skipped))
}
