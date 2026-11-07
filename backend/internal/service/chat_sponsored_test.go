package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// sponsoredChatRepo records saved sessions; any other chat repository call
// fails the test (nil interface panic), proving the sponsored branch only
// saves the session.
type sponsoredChatRepo struct {
	repository.ChatRepository
	saved []domain.Session
}

func (r *sponsoredChatRepo) SaveSession(_ context.Context, s *domain.Session) error {
	r.saved = append(r.saved, *s)
	return nil
}

// readOnlyWallets serves a fixed wallet and fails on any write.
type readOnlyWallets struct {
	repository.WalletRepository
	w domain.Wallet
}

func (r readOnlyWallets) Get(context.Context, uuid.UUID) (*domain.Wallet, error) {
	w := r.w
	return &w, nil
}

type loggedEvent struct {
	typ  string
	data map[string]any
	to   []uuid.UUID
}

type eventLog struct {
	NoopPublisher
	mu     sync.Mutex
	events []loggedEvent
}

func (l *eventLog) Publish(_ context.Context, typ string, data any, to ...uuid.UUID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	m, _ := data.(map[string]any)
	l.events = append(l.events, loggedEvent{typ, m, to})
	return nil
}

func (l *eventLog) of(typ string, to uuid.UUID) []loggedEvent {
	var out []loggedEvent
	for _, e := range l.events {
		if e.typ == typ && len(e.to) == 1 && e.to[0] == to {
			out = append(out, e)
		}
	}
	return out
}

func sponsoredFixture(rate, list int64) (*ChatService, *sponsoredChatRepo, *eventLog, *domain.Session) {
	repo, events := &sponsoredChatRepo{}, &eventLog{}
	clk := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	// wallet (the WalletService) is deliberately nil: touching it panics.
	c := &ChatService{chat: repo, wallets: readOnlyWallets{w: domain.Wallet{Balance: 0, FreeMinutes: 3}}, events: events,
		tracker: analytics.Noop{}, clock: clk,
		cfg: ChatSettings{BillingInterval: time.Minute, SharePercent: 60, LowBalanceMinutes: 2, PayoutCentsPerCredit: 2, Currency: "USD"}}
	now := clk.Now()
	s := &domain.Session{ID: uuid.New(), ClientID: uuid.New(), PsychicID: uuid.New(), Status: domain.SessionActive, Sponsored: true,
		RatePerMinute: rate, ListRatePerMinute: list, StartedAt: &now, LastResumedAt: &now}
	return c, repo, events, s
}

func TestSponsoredMinuteNeverTouchesTheWallet(t *testing.T) {
	c, repo, events, s := sponsoredFixture(40, 50) // a member discount: pays 40, list 50
	ctx := context.Background()
	start := c.clock.Now()
	var post afterCommit
	for minute := 1; minute <= 3; minute++ {
		due := start.Add(time.Duration(minute-1) * time.Minute)
		if err := c.billMinuteLocked(ctx, s, due, due, &post); err != nil {
			t.Fatalf("minute %d: %v", minute, err)
		}
	}
	// The session keeps its normal accounting and counts the sponsored value.
	if s.BilledMinutes != 3 || s.SponsoredMinutes != 3 || s.SponsoredCredits != 120 {
		t.Fatalf("session: billed=%d sponsored=%d credits=%d", s.BilledMinutes, s.SponsoredMinutes, s.SponsoredCredits)
	}
	if s.CreditsCharged != 0 || s.FreeMinutesUsed != 0 || s.Status != domain.SessionActive || s.EndReason != "" {
		t.Fatalf("a sponsored chat charged or consumed something: %+v", s)
	}
	if s.NextBillingAt == nil || !s.NextBillingAt.Equal(start.Add(3*time.Minute)) {
		t.Fatalf("next billing: %v", s.NextBillingAt)
	}
	if len(repo.saved) != 3 {
		t.Fatalf("saves: %d", len(repo.saved))
	}
	// Even after free mode is switched off the branch keeps applying: it is
	// driven by the session row, never by the setting.
	c.freeModeActive = func(context.Context) bool { return false }
	if err := c.billMinuteLocked(ctx, s, start.Add(3*time.Minute), start.Add(3*time.Minute), &post); err != nil || s.SponsoredMinutes != 4 {
		t.Fatalf("after free mode ended: %v %+v", err, s)
	}

	post.run(ctx)
	clientEvents := events.of(realtime.EventSessionBilled, s.ClientID)
	psychicEvents := events.of(realtime.EventSessionBilled, s.PsychicID)
	if len(clientEvents) != 4 || len(psychicEvents) != 4 {
		t.Fatalf("billed events: client %d psychic %d", len(clientEvents), len(psychicEvents))
	}
	cp := clientEvents[0].data
	if cp["sponsored"] != true || cp["free"] != true || cp["charged"] != 0 || cp["minute"] != 1 {
		t.Fatalf("client payload: %v", cp)
	}
	if w, ok := cp["wallet"].(WalletView); !ok || w.Balance != 0 || w.FreeMinutes != 3 {
		t.Fatalf("the wallet is shown unchanged: %v", cp["wallet"])
	}
	pp := psychicEvents[0].data
	if pp["sponsored"] != true || pp["charged"] != int64(50) || pp["psychic_earnings"] != int64(30) || pp["psychic_earnings_cents"] != int64(60) {
		t.Fatalf("advisor payload (list rate, live earnings): %v", pp)
	}
	if last := psychicEvents[3].data["psychic_earnings"]; last != int64(120) { // 4 × 50 × 60 %
		t.Fatalf("live earnings after 4 minutes: %v", last)
	}
	// No low-balance warning (the client has no credits), no wallet update.
	for _, e := range events.events {
		if e.typ == realtime.EventSessionLowBalance || e.typ == realtime.EventWalletUpdated {
			t.Fatalf("unexpected %s for a sponsored chat", e.typ)
		}
	}
}

func TestSponsoredViewsAndAdvisorRedaction(t *testing.T) {
	c, _, _, s := sponsoredFixture(40, 50)
	s.BilledMinutes, s.SponsoredMinutes, s.SponsoredCredits, s.DiscountBps = 3, 3, 120, 2000
	ps := &participants{svc: c, cache: map[uuid.UUID]ParticipantView{
		s.ClientID: {ID: s.ClientID}, s.PsychicID: {ID: s.PsychicID}}}
	client := c.sessionView(context.Background(), ps, s, viewer{ID: s.ClientID})
	if !client.Sponsored || client.SponsoredMinutes != 3 || client.SponsoredCredits != 120 || *client.CreditsCharged != 0 {
		t.Fatalf("client view: %+v", client)
	}
	// The advisor never sees the client's discounted rate: the sponsored
	// value is expressed at the list rate.
	adv := c.sessionView(context.Background(), ps, s, viewer{ID: s.PsychicID})
	if !adv.Sponsored || adv.SponsoredMinutes != 3 || adv.SponsoredCredits != 150 || adv.CreditsCharged != nil || adv.RatePerMinute != 50 {
		t.Fatalf("advisor view: %+v", adv)
	}
	if adv.PsychicEarnings == nil || *adv.PsychicEarnings != 90 { // 3 × 50 × 60 %
		t.Fatalf("advisor live earnings: %v", adv.PsychicEarnings)
	}
	admin := c.sessionView(context.Background(), ps, s, viewer{Admin: true})
	if admin.SponsoredCredits != 120 || !admin.Sponsored {
		t.Fatalf("admin view: %+v", admin)
	}
}
