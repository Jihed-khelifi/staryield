package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.5): the purchase stream.

type purchaseStream struct {
	events chan realtime.Envelope
	closed chan struct{}
	header http.Header
	status int
}

// openPurchaseStream connects to GET /credits/purchases/:id/stream with the client's cookies.
func openPurchaseStream(t *testing.T, ctx context.Context, c *client, id uuid.UUID) *purchaseStream {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.e.srv.URL+"/api/v1/credits/purchases/"+id.String()+"/stream", nil)
	req.Header.Set("Origin", c.origin)
	req.Header.Set("Accept", "text/event-stream")
	res, err := (&http.Client{Jar: c.http.Jar}).Do(req)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	s := &purchaseStream{events: make(chan realtime.Envelope, 64), closed: make(chan struct{}), header: res.Header, status: res.StatusCode}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		close(s.closed)
		return s
	}
	go func() {
		defer close(s.closed)
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				var env realtime.Envelope
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &env) == nil {
					s.events <- env
				}
			}
		}
	}()
	return s
}

func (s *purchaseStream) next(t *testing.T, within time.Duration) realtime.Envelope {
	t.Helper()
	select {
	case e := <-s.events:
		return e
	case <-time.After(within):
		t.Fatal("no stream frame in time")
	}
	return realtime.Envelope{}
}

func (s *purchaseStream) waitClosed(t *testing.T, within time.Duration) {
	t.Helper()
	select {
	case <-s.closed:
	case <-time.After(within):
		t.Fatal("the stream did not close")
	}
}

func purchaseOf(t *testing.T, e realtime.Envelope) service.PurchaseView {
	t.Helper()
	var v service.PurchaseView
	if err := json.Unmarshal(e.Data, &v); err != nil {
		t.Fatalf("frame %s: %v", e.Data, err)
	}
	return v
}

// fastStream shortens the purchase stream's timings for a test.
func fastStream(w *opsWorld, poll, lifetime time.Duration) {
	w.e.app.Services.Credits.WithStreamSettings(service.PurchaseStreamSettings{PollEvery: poll, MaxChecks: 20, MaxLifetime: lifetime})
	w.e.t.Cleanup(func() { w.e.app.Services.Credits.WithStreamSettings(service.DefaultPurchaseStream()) })
}

// buyWithToken starts a 250-credit purchase paying with a sandbox token.
func (w *opsWorld) buyWithToken(c *client, token string) (service.PurchaseView, response) {
	w.e.t.Helper()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		if p.Code == "test_250" {
			r := c.call(http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": p.ID, "payment_token": token, "provider": "sandbox_a",
				"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()})
			var v service.PurchaseView
			if r.status == http.StatusCreated || r.status == http.StatusOK {
				r.decode(w.e.t, &v)
			}
			return v, r
		}
	}
	w.e.t.Fatal("no test_250 pack")
	return service.PurchaseView{}, response{}
}

// A 3-D Secure card answers pending with next_action; the stream sends that state at once, then the
// server's own checks with the processor settle it (credits once, card and 3-D Secure stored) and it closes.
func TestPurchaseStreamSettlesAThreeDSecurePurchaseItself(t *testing.T) {
	w := newOpsWorld(t)
	fastStream(w, 200*time.Millisecond, 30*time.Second)
	before := w.c.wallet().Balance
	p, r := w.buyWithToken(w.c, "sbx_a_visa_3220_12_2035")
	if r.status != http.StatusCreated || p.Status != "pending" || p.NextAction == nil || p.NextAction.Type != "sandbox_3ds" || p.Wallet == nil || p.Wallet.Balance != before {
		t.Fatalf("pending purchase: %d %s", r.status, r.body)
	}
	if got := w.c.wallet().Balance; got != before {
		t.Fatalf("nothing is credited before the customer finishes: %d", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := openPurchaseStream(t, ctx, w.c, p.ID)
	if s.status != http.StatusOK || !strings.HasPrefix(s.header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream: %d %v", s.status, s.header)
	}
	first := s.next(t, 3*time.Second)
	if first.Type != "purchase.updated" || first.ID != "" || purchaseOf(t, first).Status != "pending" || purchaseOf(t, first).ID != p.ID {
		t.Fatalf("first frame: %+v %s", first, first.Data)
	}
	final := s.next(t, 10*time.Second) // the sandbox completes the challenge 2 s after the charge
	v := purchaseOf(t, final)
	if final.Type != "purchase.updated" || v.Status != "succeeded" || v.Wallet == nil || v.Wallet.Balance != before+250 || v.Card == nil ||
		v.Card.Brand != "visa" || v.Card.Last4 != "3220" || v.CompletedAt == nil {
		t.Fatalf("final frame: %s", final.Data)
	}
	s.waitClosed(t, 3*time.Second)

	if got := w.c.wallet().Balance; got != before+250 {
		t.Fatalf("credited once: %d, want %d", got, before+250)
	}
	var tx struct {
		Status, CardBrand, CardLast4, RiskLevel, ThreeDS string
	}
	w.e.app.DB.Raw("SELECT status, card_brand, card_last4, risk_level, three_ds FROM payment_transactions WHERE credit_purchase_id = ?", p.ID).Scan(&tx)
	if tx.Status != "succeeded" || tx.CardBrand != "visa" || tx.CardLast4 != "3220" || tx.RiskLevel != "normal" || tx.ThreeDS != "authenticated" {
		t.Fatalf("stored details: %+v", tx)
	}
	// A stream opened on a settled purchase sends the final state and closes at once; the history agrees.
	again := openPurchaseStream(t, ctx, w.c, p.ID)
	if v := purchaseOf(t, again.next(t, 3*time.Second)); v.Status != "succeeded" || v.Card == nil {
		t.Fatalf("settled purchase: %+v", v)
	}
	again.waitClosed(t, 3*time.Second)
	var history service.Page[service.PurchaseView]
	w.c.must(http.StatusOK, http.MethodGet, "/api/v1/credits/purchases", nil, &history)
	if history.Items[0].ID != p.ID || history.Items[0].Status != "succeeded" {
		t.Fatalf("history: %+v", history.Items[0])
	}
	w.e.invariant(w.c.user.ID)
}

// A failed 3-D Secure step ends as failed (with the reason), credits nothing and closes the stream.
func TestPurchaseStreamReportsAFailedThreeDSecureStep(t *testing.T) {
	w := newOpsWorld(t)
	fastStream(w, 200*time.Millisecond, 30*time.Second)
	before := w.c.wallet().Balance
	p, r := w.buyWithToken(w.c, "sbx_a_visa_3221_12_2035")
	if r.status != http.StatusCreated || p.Status != "pending" || p.NextAction == nil {
		t.Fatalf("pending: %d %s", r.status, r.body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := openPurchaseStream(t, ctx, w.c, p.ID)
	if v := purchaseOf(t, s.next(t, 3*time.Second)); v.Status != "pending" {
		t.Fatalf("first: %+v", v)
	}
	v := purchaseOf(t, s.next(t, 10*time.Second))
	if v.Status != "failed" || v.FailureReason != "authentication_failed" || v.Wallet == nil || v.Wallet.Balance != before {
		t.Fatalf("failed frame: %+v", v)
	}
	s.waitClosed(t, 3*time.Second)
	if st := w.text("SELECT status FROM payment_transactions WHERE credit_purchase_id = ?", p.ID); st != "failed" {
		t.Fatalf("payment status: %s", st)
	}
	if got := w.c.wallet().Balance; got != before {
		t.Fatalf("a failed step credits nothing: %d", got)
	}
}

// A saved card that needs 3-D Secure goes through the same stream.
func TestPurchaseStreamWithASavedThreeDSecureCard(t *testing.T) {
	w := newOpsWorld(t)
	fastStream(w, 200*time.Millisecond, 30*time.Second)
	card := w.c.attachCard("sbx_a_visa_3220_11_2034")
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	w.c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	var pack service.PackView
	for _, p := range packs.Items {
		if p.Code == "test_250" {
			pack = p
		}
	}
	before := w.c.wallet().Balance
	var p service.PurchaseView
	w.c.must(http.StatusCreated, http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": pack.ID, "payment_method_id": card.ID,
		"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &p)
	if p.Status != "pending" || p.NextAction == nil || p.NextAction.Type != "sandbox_3ds" {
		t.Fatalf("saved card: %+v", p)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := openPurchaseStream(t, ctx, w.c, p.ID)
	s.next(t, 3*time.Second)
	if v := purchaseOf(t, s.next(t, 10*time.Second)); v.Status != "succeeded" || v.Wallet == nil || v.Wallet.Balance != before+250 || v.Card == nil || v.Card.Last4 != "3220" {
		t.Fatalf("final: %+v", v)
	}
	s.waitClosed(t, 3*time.Second)
}

// Webhooks settle a pending purchase too: the settlement is published as purchase.updated and the open
// stream relays it at once (polling is switched off here to prove it).
func TestPurchaseStreamRelaysASettlementMadeElsewhere(t *testing.T) {
	w := newOpsWorld(t)
	fastStream(w, time.Hour, 30*time.Second)
	before := w.c.wallet().Balance
	p, _ := w.buyWithToken(w.c, "sbx_a_visa_3220_12_2035")
	if p.Status != "pending" {
		t.Fatalf("pending expected: %+v", p)
	}
	ref := w.text("SELECT coalesce(provider_ref, '') FROM payment_transactions WHERE credit_purchase_id = ?", p.ID)
	if ref == "" {
		t.Fatal("a pending charge must keep its provider reference (webhooks find it by it)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := openPurchaseStream(t, ctx, w.c, p.ID)
	if v := purchaseOf(t, s.next(t, 3*time.Second)); v.Status != "pending" {
		t.Fatalf("first: %+v", v)
	}
	done := make(chan response, 1)
	go func() { // another goroutine (another API instance, the provider) completes the purchase
		time.Sleep(300 * time.Millisecond)
		done <- signedWebhook(w.e, "sandbox_a", payment.WebhookEvent{ID: "evt_" + uniq("relay"), Type: "charge.succeeded", ProviderRef: ref,
			Card: &payment.CardMeta{Brand: "visa", Last4: "3220", Country: "US"}, ThreeDS: "authenticated"}, "")
	}()
	v := purchaseOf(t, s.next(t, 5*time.Second))
	if v.Status != "succeeded" || v.Wallet == nil || v.Wallet.Balance != before+250 {
		t.Fatalf("relayed frame: %+v", v)
	}
	if r := <-done; r.status != http.StatusOK {
		t.Fatalf("webhook: %d %s", r.status, r.body)
	}
	s.waitClosed(t, 3*time.Second)
	if got := w.c.wallet().Balance; got != before+250 {
		t.Fatalf("credited once: %d", got)
	}
}

// After the limit the stream says purchase.timeout and closes; the purchase stays pending.
func TestPurchaseStreamTimesOut(t *testing.T) {
	w := newOpsWorld(t)
	fastStream(w, time.Hour, 600*time.Millisecond)
	p, _ := w.buyWithToken(w.c, "sbx_a_visa_3220_12_2035")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := openPurchaseStream(t, ctx, w.c, p.ID)
	if v := purchaseOf(t, s.next(t, 3*time.Second)); v.Status != "pending" {
		t.Fatalf("first: %+v", v)
	}
	timeout := s.next(t, 5*time.Second)
	if timeout.Type != "purchase.timeout" {
		t.Fatalf("timeout frame: %+v", timeout)
	}
	s.waitClosed(t, 3*time.Second)
	if st := w.text("SELECT status FROM credit_purchases WHERE id = ?", p.ID); st != "pending" {
		t.Fatalf("a timed-out stream settles nothing: %s", st)
	}
}

// Only the buyer can open the stream (404 for everyone else, so ids cannot be probed).
func TestPurchaseStreamIsOwnerOnly(t *testing.T) {
	w := newOpsWorld(t)
	p, _ := w.buyWithToken(w.c, "sbx_a_visa_3220_12_2035")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stranger := w.e.registerClient()
	for name, c := range map[string]*client{"another client": stranger} {
		if s := openPurchaseStream(t, ctx, c, p.ID); s.status != http.StatusNotFound {
			t.Fatalf("%s: %d", name, s.status)
		}
	}
	if s := openPurchaseStream(t, ctx, w.c, uuid.New()); s.status != http.StatusNotFound {
		t.Fatalf("unknown purchase: %d", s.status)
	}
	if s := openPurchaseStream(t, ctx, w.e.client(), p.ID); s.status != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", s.status)
	}
	if r := w.c.call(http.MethodGet, "/api/v1/credits/purchases/not-a-uuid/stream", nil); r.status != http.StatusNotFound {
		t.Fatalf("bad id: %d", r.status)
	}
}

// Several streams on one purchase (two tabs) settle it once.
func TestPurchaseStreamsOfOnePurchaseCreditOnce(t *testing.T) {
	w := newOpsWorld(t)
	fastStream(w, 150*time.Millisecond, 30*time.Second) // 20 checks must outlast the sandbox's 2 s challenge
	before := w.c.wallet().Balance
	p, _ := w.buyWithToken(w.c, "sbx_a_visa_3220_12_2035")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := openPurchaseStream(t, ctx, w.c, p.ID)
			if s.status != http.StatusOK {
				t.Errorf("stream status %d", s.status)
				return
			}
			for {
				select {
				case e := <-s.events:
					if purchaseOf(t, e).Status == "succeeded" {
						return
					}
				case <-time.After(15 * time.Second):
					t.Error("a stream never saw the purchase succeed")
					return
				}
			}
		}()
	}
	wg.Wait()
	if got := w.c.wallet().Balance; got != before+250 {
		t.Fatalf("credited %d times: balance %d, want %d", (w.c.wallet().Balance-before)/250, got, before+250)
	}
	if n := w.count("SELECT count(*) FROM credit_ledger WHERE entry_type = 'purchase' AND reference_id = ?", p.ID); n != 1 {
		t.Fatalf("ledger entries: %d", n)
	}
	w.e.invariant(w.c.user.ID)
}

// A charge.succeeded event without card, risk and 3-D Secure details makes the server ask the provider once
// (RetrieveCharge) and store what it answers, before the purchase completes.
func TestChargeSucceededWithoutDetailsAsksTheProvider(t *testing.T) {
	w := newOpsWorld(t)
	p, _ := w.buyWithToken(w.c, "sbx_a_visa_3220_12_2035")
	ref := w.text("SELECT coalesce(provider_ref, '') FROM payment_transactions WHERE credit_purchase_id = ?", p.ID)
	time.Sleep(2200 * time.Millisecond) // the sandbox finishes its 3-D Secure challenge 2 s after the charge
	w.mustSend(payment.WebhookEvent{ID: "evt_" + uniq("nodetails"), Type: "charge.succeeded", ProviderRef: ref})
	var tx struct{ Status, CardLast4, RiskLevel, ThreeDS string }
	w.e.app.DB.Raw("SELECT status, card_last4, risk_level, three_ds FROM payment_transactions WHERE credit_purchase_id = ?", p.ID).Scan(&tx)
	if tx.Status != "succeeded" || tx.CardLast4 != "3220" || tx.RiskLevel != "normal" || tx.ThreeDS != "authenticated" {
		t.Fatalf("details fetched from the provider: %+v", tx)
	}
}
