package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// refreshTokens is an in-memory refresh-token table (revisions 3.3).
type refreshTokens struct {
	repository.TokenRepository
	rows     map[uuid.UUID]*domain.RefreshToken
	byHash   map[string]uuid.UUID
	locks    []uuid.UUID // ids locked FOR UPDATE, in order
	families int         // RevokeFamily calls
}

func newRefreshTokens() *refreshTokens {
	return &refreshTokens{rows: map[uuid.UUID]*domain.RefreshToken{}, byHash: map[string]uuid.UUID{}}
}

func (r *refreshTokens) CreateRefresh(_ context.Context, t *domain.RefreshToken) error {
	c := *t
	r.rows[t.ID] = &c
	r.byHash[t.TokenHash] = t.ID
	return nil
}

func (r *refreshTokens) GetRefreshByHashForUpdate(_ context.Context, hash string) (*domain.RefreshToken, error) {
	id, ok := r.byHash[hash]
	if !ok {
		return nil, repository.ErrNotFound
	}
	r.locks = append(r.locks, id)
	c := *r.rows[id]
	return &c, nil
}

func (r *refreshTokens) RefreshFamilyState(_ context.Context, family, presented uuid.UUID, since time.Time) (repository.RefreshFamilyState, error) {
	st := repository.RefreshFamilyState{Live: true}
	var open []uuid.UUID
	for _, t := range r.rows {
		if t.FamilyID != family {
			continue
		}
		switch {
		case t.RevokedAt == nil:
			open = append(open, t.ID)
		case t.RevokeReason != "rotated" && t.RevokeReason != "superseded":
			st.Live = false
		case t.RevokeReason == "rotated" && t.ID != presented && !t.RevokedAt.Before(since):
			st.Advanced = true
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].String() < open[j].String() })
	r.locks = append(r.locks, open...)
	st.Open = len(open)
	return st, nil
}

func (r *refreshTokens) SupersedeOpenRefresh(_ context.Context, family uuid.UUID, at time.Time) (int64, error) {
	var n int64
	for _, t := range r.rows {
		if t.FamilyID == family && t.RevokedAt == nil {
			t.RevokedAt, t.RevokeReason = &at, "superseded"
			n++
		}
	}
	return n, nil
}

func (r *refreshTokens) RevokeRefresh(_ context.Context, id uuid.UUID, reason string, replacedBy *uuid.UUID, at time.Time) error {
	if t := r.rows[id]; t != nil && t.RevokedAt == nil {
		t.RevokedAt, t.RevokeReason, t.ReplacedBy = &at, reason, replacedBy
	}
	return nil
}

func (r *refreshTokens) RevokeFamily(_ context.Context, family uuid.UUID, reason string, at time.Time) (int64, error) {
	r.families++
	var n int64
	for _, t := range r.rows {
		if t.FamilyID == family && t.RevokedAt == nil {
			t.RevokedAt, t.RevokeReason = &at, reason
			n++
		}
	}
	return n, nil
}

type refreshFixture struct {
	*identityFixture
	auth   *AuthService
	tokens *refreshTokens
}

func newRefreshFixture(t *testing.T) *refreshFixture {
	t.Helper()
	f := newIdentityFixture(t)
	tokens := newRefreshTokens()
	issuer, err := security.NewTokenIssuer(strings.Repeat("s", 32), "staryield", "staryield", 15*time.Minute, f.clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	auth := NewAuthService(memTx{db: f.db}, memUsers{db: f.db}, tokens, nil, nil, f.hasher, issuer, cache.NewMemory(f.clock.Now),
		realtime.NewMemoryTickets(f.clock.Now), f.mail, NoopPublisher{}, analytics.Noop{}, f.clock, AuthSettings{RefreshTTL: 24 * time.Hour})
	return &refreshFixture{identityFixture: f, auth: auth, tokens: tokens}
}

// session opens a family and returns its first raw refresh token.
func (f *refreshFixture) session(t *testing.T) string {
	t.Helper()
	res, err := f.auth.issueSession(context.Background(), &f.user, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.RefreshToken
}

func (f *refreshFixture) refresh(t *testing.T, raw string) (*AuthResult, error) {
	t.Helper()
	return f.auth.Refresh(context.Background(), raw)
}

func (f *refreshFixture) row(raw string) *domain.RefreshToken {
	return f.tokens.rows[f.tokens.byHash[security.HashToken(raw)]]
}

func wantExpired(t *testing.T, err error) {
	t.Helper()
	appErr, ok := apperror.As(err)
	if !ok || appErr.Status != 401 {
		t.Fatalf("want 401 session expired, got %v", err)
	}
}

func TestRefreshRotationWindows(t *testing.T) {
	if refreshReuseGrace != 2*time.Minute || refreshRecoveryWindow != 10*time.Minute {
		t.Fatalf("windows changed: %v %v", refreshReuseGrace, refreshRecoveryWindow)
	}

	t.Run("plain rotation revokes the presented token", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		res, err := f.refresh(t, first)
		if err != nil {
			t.Fatal(err)
		}
		if r := f.row(first); r.RevokedAt == nil || r.RevokeReason != "rotated" || r.ReplacedBy == nil {
			t.Fatalf("presented token: %+v", r)
		}
		if f.row(res.RefreshToken).RevokedAt != nil {
			t.Fatal("successor must be valid")
		}
	})

	t.Run("within the grace both refreshes succeed and the successor stays valid", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		a, err := f.refresh(t, first)
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(refreshReuseGrace - time.Second)
		b, err := f.refresh(t, first)
		if err != nil {
			t.Fatalf("second refresh inside the grace: %v", err)
		}
		if f.row(a.RefreshToken).RevokedAt != nil || f.row(b.RefreshToken).RevokedAt != nil {
			t.Fatal("both successors must stay valid")
		}
		if f.row(a.RefreshToken).FamilyID != f.row(first).FamilyID || f.row(b.RefreshToken).FamilyID != f.row(first).FamilyID {
			t.Fatal("successors must stay in the family")
		}
		if _, err := f.refresh(t, a.RefreshToken); err != nil {
			t.Fatalf("successor A: %v", err)
		}
		if f.tokens.families != 0 {
			t.Fatal("no family revocation expected")
		}
	})

	t.Run("recovery window revokes the unused successor as superseded", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		lost, err := f.refresh(t, first) // the response never reached the browser
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(5 * time.Minute)
		f.tokens.locks = nil
		recovered, err := f.refresh(t, first)
		if err != nil {
			t.Fatalf("lost-response recovery: %v", err)
		}
		if r := f.row(lost.RefreshToken); r.RevokedAt == nil || r.RevokeReason != "superseded" {
			t.Fatalf("successor: %+v", r)
		}
		if f.row(recovered.RefreshToken).RevokedAt != nil {
			t.Fatal("recovered token must be valid")
		}
		// lock discipline: presented row first, then the family's open rows
		if len(f.tokens.locks) < 2 || f.tokens.locks[0] != f.row(first).ID || f.tokens.locks[1] != f.row(lost.RefreshToken).ID {
			t.Fatalf("lock order: %v", f.tokens.locks)
		}
		// presenting the superseded successor is reuse
		_, err = f.refresh(t, lost.RefreshToken)
		wantExpired(t, err)
		if f.row(recovered.RefreshToken).RevokedAt == nil {
			t.Fatal("reuse must revoke the family")
		}
	})

	t.Run("successor already used is reuse", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		a, _ := f.refresh(t, first)
		b, err := f.refresh(t, a.RefreshToken) // the browser moved on
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(5 * time.Minute)
		_, err = f.refresh(t, first)
		wantExpired(t, err)
		if f.row(b.RefreshToken).RevokedAt == nil || f.row(b.RefreshToken).RevokeReason != "reuse_detected" {
			t.Fatalf("family must be revoked: %+v", f.row(b.RefreshToken))
		}
		if f.tokens.families != 1 {
			t.Fatalf("families %d", f.tokens.families)
		}
	})

	t.Run("older than the recovery window is reuse", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		a, _ := f.refresh(t, first)
		f.clock.Advance(refreshRecoveryWindow + time.Second)
		_, err := f.refresh(t, first)
		wantExpired(t, err)
		if f.row(a.RefreshToken).RevokedAt == nil {
			t.Fatal("family must be revoked")
		}
	})

	t.Run("recovery boundary at exactly the window is reuse", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		if _, err := f.refresh(t, first); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(refreshRecoveryWindow)
		_, err := f.refresh(t, first)
		wantExpired(t, err)
	})

	t.Run("logged out token is plain expiry, not reuse", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		at := f.clock.Now()
		f.row(first).RevokedAt, f.row(first).RevokeReason = &at, "logout"
		_, err := f.refresh(t, first)
		wantExpired(t, err)
		if f.tokens.families != 0 {
			t.Fatal("logout must not count as reuse")
		}
	})

	t.Run("a signed-out family is never reopened inside the grace", func(t *testing.T) {
		for _, reason := range []string{"logout", "password_reset", "logout_all", "account_deactivated"} {
			t.Run(reason, func(t *testing.T) {
				f := newRefreshFixture(t)
				first := f.session(t)
				if _, err := f.refresh(t, first); err != nil {
					t.Fatal(err)
				}
				f.clock.Advance(10 * time.Second)
				if _, err := f.tokens.RevokeFamily(context.Background(), f.row(first).FamilyID, reason, f.clock.Now()); err != nil {
					t.Fatal(err)
				}
				rows := len(f.tokens.rows)
				_, err := f.refresh(t, first)
				wantExpired(t, err)
				if len(f.tokens.rows) != rows {
					t.Fatal("no token may be issued for a signed-out family")
				}
				if f.tokens.families != 1 {
					t.Fatalf("a signed-out family is expiry, not reuse (RevokeFamily calls %d)", f.tokens.families)
				}
			})
		}
	})

	t.Run("a signed-out family is never reopened inside the recovery window", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		if _, err := f.refresh(t, first); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(5 * time.Minute)
		if _, err := f.tokens.RevokeFamily(context.Background(), f.row(first).FamilyID, "logout", f.clock.Now()); err != nil {
			t.Fatal(err)
		}
		rows := len(f.tokens.rows)
		_, err := f.refresh(t, first)
		wantExpired(t, err)
		if len(f.tokens.rows) != rows {
			t.Fatal("no token may be issued for a signed-out family")
		}
	})

	t.Run("recovery is refused when a sibling successor was used", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		if _, err := f.refresh(t, first); err != nil { // successor A (lost)
			t.Fatal(err)
		}
		f.clock.Advance(30 * time.Second)
		b, err := f.refresh(t, first) // sibling B inside the grace (received)
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(time.Minute)
		c, err := f.refresh(t, b.RefreshToken) // the browser moved on with B
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(3 * time.Minute) // first was rotated 4.5 min ago
		_, err = f.refresh(t, first)
		wantExpired(t, err)
		if r := f.row(c.RefreshToken); r.RevokedAt == nil || r.RevokeReason != "reuse_detected" {
			t.Fatalf("replay must revoke the family: %+v", r)
		}
	})

	t.Run("recovery supersedes every unused sibling", func(t *testing.T) {
		f := newRefreshFixture(t)
		first := f.session(t)
		a, _ := f.refresh(t, first)
		f.clock.Advance(30 * time.Second)
		b, _ := f.refresh(t, first)
		f.clock.Advance(4 * time.Minute)
		recovered, err := f.refresh(t, first)
		if err != nil {
			t.Fatalf("recovery: %v", err)
		}
		for _, lost := range []string{a.RefreshToken, b.RefreshToken} {
			if r := f.row(lost); r.RevokedAt == nil || r.RevokeReason != "superseded" {
				t.Fatalf("sibling must be superseded: %+v", r)
			}
		}
		if f.row(recovered.RefreshToken).RevokedAt != nil {
			t.Fatal("recovered token must be valid")
		}
	})
}
