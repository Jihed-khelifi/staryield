package e2e

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Post-commit hooks run once the outermost transaction commits, with a
// context detached from it, never for a rolled-back transaction, and at once
// outside a transaction.
func TestAfterCommitRunsOnlyOnceCommitted(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	txm := database.NewTxManager(e.app.DB)
	var mu sync.Mutex
	var ran []string
	record := func(name string) func(context.Context) {
		return func(c context.Context) {
			if database.InTx(c) {
				t.Errorf("hook %s received the transaction's context", name)
			}
			mu.Lock()
			ran = append(ran, name)
			mu.Unlock()
		}
	}
	err := txm.WithinTx(ctx, func(ctx context.Context) error {
		database.AfterCommit(ctx, record("outer"))
		return txm.WithinTx(ctx, func(ctx context.Context) error {
			database.AfterCommit(ctx, record("nested"))
			mu.Lock()
			defer mu.Unlock()
			if len(ran) != 0 {
				t.Error("a hook ran before the commit")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := txm.WithinTx(ctx, func(ctx context.Context) error {
		database.AfterCommit(ctx, record("rolled back"))
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("rollback: %v", err)
	}
	database.AfterCommit(ctx, record("no transaction"))
	if len(ran) != 3 || ran[0] != "outer" || ran[1] != "nested" || ran[2] != "no transaction" {
		t.Fatalf("hooks: %v", ran)
	}
}

// The outbox wakes its delivery loop only after the enqueuing transaction
// committed, so the wake-up finds the row (before, it found nothing and the
// message waited for the next poll).
func TestOutboxWakesAfterCommit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	m := e.app.Services.Mail
	// Drain a stale wake-up left by other work of this environment.
	select {
	case <-m.Wake():
	default:
	}
	txm := database.NewTxManager(e.app.DB)
	to := "wake-" + domain.NewID().String()[:8] + "@example.com"
	err := txm.WithinTx(ctx, func(ctx context.Context) error {
		if err := m.Enqueue(ctx, service.EmailRequest{To: to, Locale: "en", Category: domain.EmailAccount, Template: "after_commit_test",
			Content: mail.Content{Subject: "Hello", Heading: "Hello", Paragraphs: []string{"Body"}}}); err != nil {
			return err
		}
		select {
		case <-m.Wake():
			t.Error("the delivery loop was woken before the row was committed")
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-m.Wake():
	default:
		t.Fatal("the delivery loop was not woken after the commit")
	}
	if got := e.mailTo(to); len(got) != 1 {
		t.Fatalf("delivered %d messages to %s", len(got), to)
	}
}
