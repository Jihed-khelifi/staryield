package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Tickets remember the signed-in device (refresh-token family); legacy
// two-part values still parse.
func TestFamilyTickets(t *testing.T) {
	mr := miniredis.RunT(t)
	stores := map[string]TicketStore{"memory": NewMemoryTickets(nil), "redis": NewRedisTickets(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:")}
	for name, ts := range stores {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ft, ok := ts.(FamilyTickets)
			if !ok {
				t.Fatal("store does not implement FamilyTickets")
			}
			u, fam := uuid.New(), uuid.New()
			tk, err := ft.IssueFamily(ctx, u, "client", fam, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			c, ok, err := ft.ConsumeClaims(ctx, tk)
			if err != nil || !ok || c.UserID != u || c.Role != "client" || c.Family != fam {
				t.Fatalf("claims: %+v %v %v", c, ok, err)
			}
			if _, ok, _ := ft.ConsumeClaims(ctx, tk); ok {
				t.Fatal("tickets must be single use")
			}
			// Issue without a family (legacy callers) → zero family.
			tk, _ = ts.Issue(ctx, u, "psychic", time.Minute)
			c, ok, _ = ft.ConsumeClaims(ctx, tk)
			if !ok || c.Family != uuid.Nil || c.Role != "psychic" {
				t.Fatalf("legacy ticket: %+v %v", c, ok)
			}
		})
	}
	for v, valid := range map[string]bool{
		uuid.NewString() + "|client":                       true,
		uuid.NewString() + "|client|" + uuid.NewString():   true,
		uuid.NewString() + "|client|not-a-uuid":            false,
		uuid.NewString():                                   false,
		"x|client":                                         false,
		uuid.NewString() + "|a|" + uuid.NewString() + "|b": false,
	} {
		if _, ok := parseTicketValue(v); ok != valid {
			t.Errorf("parseTicketValue(%q) = %v, want %v", v, ok, valid)
		}
	}
}

// "revoke:<family>" closes only that device's connections; a plain revoke
// still closes them all.
func TestHubFamilyRevoke(t *testing.T) {
	h := NewHub(zap.NewNop(), 8)
	u := uuid.New()
	famA, famB := uuid.New(), uuid.New()
	a1, a2, b, legacy := newTestClient(u, 16, false), newTestClient(u, 16, false), newTestClient(u, 16, false), newTestClient(u, 16, false)
	a1.Family, a2.Family, b.Family = famA, famA, famB
	other := newTestClient(uuid.New(), 16, false)
	other.Family = famA // same family id on another user must not matter
	for _, c := range []*Client{a1, a2, b, legacy, other} {
		if err := h.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	h.Dispatch(Delivery{UserID: u, Control: ControlRevoke + ":" + famA.String()})
	for _, c := range []*Client{a1, a2} {
		select {
		case <-c.Done():
		default:
			t.Fatal("device connection not closed")
		}
		if code, _ := c.closeInfo(); code != CloseUnauthorized {
			t.Fatalf("close code %d", code)
		}
	}
	for _, c := range []*Client{b, legacy, other} {
		select {
		case <-c.Done():
			t.Fatal("another device was closed")
		default:
		}
	}
	h.Dispatch(Delivery{UserID: u, Control: ControlRevoke + ":garbage"})
	select {
	case <-b.Done():
		t.Fatal("malformed family closed a connection")
	default:
	}
	h.Dispatch(Delivery{UserID: u, Control: ControlRevoke})
	for _, c := range []*Client{b, legacy} {
		select {
		case <-c.Done():
		default:
			t.Fatal("full revoke must close every connection")
		}
	}
}

func TestPublisherRevokeFamily(t *testing.T) {
	b := NewMemoryBroker(16)
	p := NewPublisher(b, time.Now)
	u, fam := uuid.New(), uuid.New()
	got := make(chan Delivery, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = b.Run(ctx, func(d Delivery) {
			if d.Control != "" {
				got <- d
			}
		})
	}()
	time.Sleep(20 * time.Millisecond)
	if err := p.RevokeFamily(ctx, u, fam); err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-got:
		if d.UserID != u || d.Control != ControlRevoke+":"+fam.String() {
			t.Fatalf("delivery: %+v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no control delivery")
	}
}
