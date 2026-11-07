package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func TestPsychicBalanceForTheWalletPill(t *testing.T) {
	e := newEnv(t)
	e.pinSignupFreeMinutes(3) // revisions 3.0: these assertions are written for 3 promotional minutes
	client, psychic, cws, _, s := startedSession(t, e, 40, 250)

	var empty service.PsychicBalanceView
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/balance", nil, &empty)
	if empty.AvailableCredits != 0 || empty.AvailableCents != 0 || empty.Currency == "" {
		t.Fatalf("balance before any earnings: %+v", empty)
	}

	// Minutes 2–3 use free minutes, minute 4 is paid; ending settles earnings.
	// Each tick lands one second past a minute boundary (a minute is billed
	// once it has started).
	for minute := 2; minute <= 4; minute++ {
		step := time.Minute
		if minute == 2 {
			step += time.Second
		}
		e.tick(step)
		cws.waitMatch(realtime.EventSessionBilled, fmt.Sprintf(`"minute":%d,`, minute), 5*time.Second)
	}
	e.clock.Advance(20 * time.Second)
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil, nil)

	var after service.PsychicBalanceView
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/balance", nil, &after)
	cents := e.app.Config.Billing.PayoutCentsPerCredit
	if after.AvailableCredits <= 0 || after.AvailableCents != after.AvailableCredits*cents || after.LifetimeNetCredits < after.AvailableCredits {
		t.Fatalf("balance after a paid minute: %+v (cents per credit %d)", after, cents)
	}

	if r := client.call(http.MethodGet, "/api/v1/psychic/balance", nil); r.status != http.StatusForbidden {
		t.Fatalf("client reading an advisor balance: %d", r.status)
	}
	if r := e.client().call(http.MethodGet, "/api/v1/psychic/balance", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous balance: %d", r.status)
	}
}
