package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// A member's chat discount lowers what the client pays per minute; the
// advisor still earns their share of the list rate (platform-funded) and
// never sees the client's discount.
func TestMemberChatDiscountIsPlatformFunded(t *testing.T) {
	e := newEnv(t)
	e.pinSignupFreeMinutes(3) // revisions 3.0: these assertions are written for 3 promotional minutes
	e.ensurePacks()
	_, version := e.billingPlan(999, func(v *service.PlanVersionInput) { v.ChatDiscountBps = 2000 }) // 20 %
	psychic := e.createPsychic(100)
	pws := psychic.goOnline()
	client := e.registerClient()
	client.buyCredits(250)
	client.subscribe(version, client.attachCard("sbx_a_visa_4242_12_2035").ID)
	cws := client.connect("")

	var requested service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{
		"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &requested)
	if requested.RatePerMinute != 80 || requested.ListRatePerMinute != 100 || requested.DiscountPct != 20 {
		t.Fatalf("client view of a discounted session: %+v", requested)
	}
	ev := pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	if got := sessionFrom(t, ev.Data); got.RatePerMinute != 100 || got.DiscountPct != 0 {
		t.Fatalf("advisor must see their own rate and no discount: %+v", got)
	}
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	cws.waitFor(realtime.EventSessionStarted, 5*time.Second)

	// Three free signup minutes, then two paid minutes at 80.
	e.tick(time.Minute + time.Second)
	e.tick(time.Minute)
	e.tick(time.Minute)
	e.tick(time.Minute)
	for {
		b := pws.waitFor(realtime.EventSessionBilled, 5*time.Second)
		if strings.Contains(string(b.Data), `"minute":5`) {
			if !strings.Contains(string(b.Data), `"charged":100`) {
				t.Fatalf("advisor's billed event should carry the list rate: %s", b.Data)
			}
			break
		}
	}
	var sum service.SessionSummary
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/end", nil, &sum)
	if sum.Session.BilledMinutes != 5 || sum.Session.FreeMinutesUsed != 3 || *sum.Session.CreditsCharged != 160 {
		t.Fatalf("client summary: %+v", sum.Session)
	}
	if w := client.wallet(); w.Balance != 90 {
		t.Fatalf("client paid 2 × 80: balance %d", w.Balance)
	}
	pws.waitFor(realtime.EventSessionSettled, 5*time.Second)
	var earnings service.EarningsReport
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/earnings", nil, &earnings)
	if earnings.Summary.GrossCredits != 200 || earnings.Summary.NetCredits != 120 {
		t.Fatalf("advisor earns 60%% of 2 × 100: %+v", earnings.Summary)
	}
}
