package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// ---- in-memory ReferralRepository ------------------------------------------

type memReferralRepo struct {
	mu       sync.Mutex
	codes    map[uuid.UUID]string
	byCode   map[string]uuid.UUID
	active   map[uuid.UUID]bool // active clients
	refs     map[uuid.UUID]*domain.Referral
	verified map[uuid.UUID]bool
	sessions map[uuid.UUID]uuid.UUID // client -> settled reading with minutes
	visitors map[string]uuid.UUID    // anonymous id -> linked user
	networks map[uuid.UUID]string    // inviter -> keyed ip hash of an active device
	names    map[uuid.UUID]string
	wallets  *memLedger
}

func newMemReferralRepo(w *memLedger) *memReferralRepo {
	return &memReferralRepo{codes: map[uuid.UUID]string{}, byCode: map[string]uuid.UUID{}, active: map[uuid.UUID]bool{},
		refs: map[uuid.UUID]*domain.Referral{}, verified: map[uuid.UUID]bool{}, sessions: map[uuid.UUID]uuid.UUID{},
		visitors: map[string]uuid.UUID{}, networks: map[uuid.UUID]string{}, names: map[uuid.UUID]string{}, wallets: w}
}

func (r *memReferralRepo) CodeOf(_ context.Context, id uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.codes[id]; ok {
		return c, nil
	}
	return "", repository.ErrNotFound
}

func (r *memReferralRepo) CreateCode(_ context.Context, id uuid.UUID, code string, _ time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.codes[id]; ok {
		return false, nil
	}
	if _, ok := r.byCode[code]; ok {
		return false, nil
	}
	r.codes[id], r.byCode[code] = code, id
	return true, nil
}

func (r *memReferralRepo) InviterByCode(_ context.Context, code string) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.byCode[code]; ok && r.active[id] {
		return id, nil
	}
	return uuid.Nil, repository.ErrNotFound
}

func (r *memReferralRepo) Create(_ context.Context, ref *domain.Referral) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.refs {
		if x.RefereeID != nil && ref.RefereeID != nil && *x.RefereeID == *ref.RefereeID {
			return false, nil
		}
	}
	cp := *ref
	r.refs[ref.ID] = &cp
	return true, nil
}

func (r *memReferralRepo) byReferee(id uuid.UUID) *domain.Referral {
	for _, x := range r.refs {
		if x.RefereeID != nil && *x.RefereeID == id {
			return x
		}
	}
	return nil
}

func (r *memReferralRepo) ByReferee(_ context.Context, id uuid.UUID) (*domain.Referral, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if x := r.byReferee(id); x != nil {
		cp := *x
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}

func (r *memReferralRepo) ByRefereeForUpdate(ctx context.Context, id uuid.UUID) (*domain.Referral, error) {
	return r.ByReferee(ctx, id)
}

func (r *memReferralRepo) ForUpdate(_ context.Context, id uuid.UUID) (*domain.Referral, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if x, ok := r.refs[id]; ok {
		cp := *x
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}

func (r *memReferralRepo) Save(_ context.Context, ref *domain.Referral) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.refs[ref.ID]; !ok {
		return repository.ErrNotFound
	}
	cp := *ref
	r.refs[ref.ID] = &cp
	return nil
}

func (r *memReferralRepo) JoinedSince(_ context.Context, id uuid.UUID, since time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, x := range r.refs {
		if x.ReferrerID == id && !x.JoinedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

func (r *memReferralRepo) SharesSessionNetwork(_ context.Context, id uuid.UUID, hash string, _ time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return hash != "" && r.networks[id] == hash, nil
}

func (r *memReferralRepo) VisitorOwner(_ context.Context, anon string) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.visitors[anon], nil
}

func (r *memReferralRepo) EmailVerified(_ context.Context, id uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.verified[id], nil
}

func (r *memReferralRepo) QualifyingSession(_ context.Context, id uuid.UUID) (*uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[id]; ok {
		return &s, nil
	}
	return nil, nil
}

func (r *memReferralRepo) LockInviter(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.codes[id]; !ok {
		return repository.ErrNotFound
	}
	return nil
}

func (r *memReferralRepo) Rewarded(_ context.Context, id uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, x := range r.refs {
		if x.ReferrerID == id && (x.Status == domain.ReferralQualified || x.Status == domain.ReferralGranted) {
			n++
		}
	}
	return n, nil
}

func (r *memReferralRepo) ReadyToQualify(_ context.Context, limit int) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var rows []*domain.Referral
	for _, x := range r.refs {
		if x.Status == domain.ReferralJoined && x.RefereeID != nil && r.verified[*x.RefereeID] {
			if _, ok := r.sessions[*x.RefereeID]; ok {
				rows = append(rows, x)
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].JoinedAt.Before(rows[j].JoinedAt) })
	var out []uuid.UUID
	for _, x := range rows {
		if len(out) < limit {
			out = append(out, *x.RefereeID)
		}
	}
	return out, nil
}

func (r *memReferralRepo) Stats(_ context.Context, id uuid.UUID) (repository.ReferralStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var s repository.ReferralStats
	for _, x := range r.refs {
		if x.ReferrerID != id {
			continue
		}
		switch x.Status {
		case domain.ReferralJoined:
			s.Joined++
		case domain.ReferralQualified:
			s.Joined, s.Qualified, s.PendingMinutes = s.Joined+1, s.Qualified+1, s.PendingMinutes+int64(x.RewardMinutes)
		case domain.ReferralGranted:
			s.Joined, s.Qualified, s.GrantedMinutes = s.Joined+1, s.Qualified+1, s.GrantedMinutes+int64(x.RewardMinutes)
		}
	}
	return s, nil
}

func (r *memReferralRepo) ClaimGrantable(_ context.Context, limit int) ([]domain.Referral, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var rows []domain.Referral
	for _, x := range r.refs {
		if x.Status == domain.ReferralQualified && !x.Flagged && r.active[x.ReferrerID] {
			rows = append(rows, *x)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].QualifiedAt.Equal(*rows[j].QualifiedAt) {
			return rows[i].QualifiedAt.Before(*rows[j].QualifiedAt)
		}
		return rows[i].ID.String() < rows[j].ID.String()
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (r *memReferralRepo) MarkGranted(_ context.Context, ids []uuid.UUID, at time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, id := range ids {
		if x := r.refs[id]; x != nil && x.Status == domain.ReferralQualified {
			x.Status, x.GrantedAt = domain.ReferralGranted, &at
			n++
		}
	}
	return n, nil
}

func (r *memReferralRepo) PendingGrants(_ context.Context) (int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var rows, minutes int64
	for _, x := range r.refs {
		if x.Status == domain.ReferralQualified && !x.Flagged && r.active[x.ReferrerID] {
			rows, minutes = rows+1, minutes+int64(x.RewardMinutes)
		}
	}
	return rows, minutes, nil
}

func (r *memReferralRepo) List(_ context.Context, f repository.ReferralFilter) ([]domain.Referral, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var rows []domain.Referral
	for _, x := range r.refs {
		if f.Status != "" && x.Status != f.Status {
			continue
		}
		if f.Flagged != nil && x.Flagged != *f.Flagged {
			continue
		}
		if f.Cursor != nil && !(x.CreatedAt.Before(f.Cursor.At) || x.CreatedAt.Equal(f.Cursor.At) && x.ID.String() < f.Cursor.ID) {
			continue
		}
		rows = append(rows, *x)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.After(rows[j].CreatedAt)
		}
		return rows[i].ID.String() > rows[j].ID.String()
	})
	if f.Limit > 0 && len(rows) > f.Limit {
		rows = rows[:f.Limit]
	}
	return rows, nil
}

func (r *memReferralRepo) Totals(_ context.Context) ([]repository.ReferralStatusTotals, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	by := map[domain.ReferralStatus]*repository.ReferralStatusTotals{}
	for _, x := range r.refs {
		t := by[x.Status]
		if t == nil {
			t = &repository.ReferralStatusTotals{Status: x.Status}
			by[x.Status] = t
		}
		t.Count++
		t.Minutes += int64(x.RewardMinutes)
		if x.Flagged {
			t.Flagged++
		}
	}
	var out []repository.ReferralStatusTotals
	for _, t := range by {
		out = append(out, *t)
	}
	return out, nil
}

func (r *memReferralRepo) TopInviters(_ context.Context, limit int) ([]repository.ReferralInviterCount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	by := map[uuid.UUID]*repository.ReferralInviterCount{}
	for _, x := range r.refs {
		if x.Status != domain.ReferralQualified && x.Status != domain.ReferralGranted {
			continue
		}
		t := by[x.ReferrerID]
		if t == nil {
			t = &repository.ReferralInviterCount{ReferrerID: x.ReferrerID}
			by[x.ReferrerID] = t
		}
		t.Qualified++
		t.Minutes += int64(x.RewardMinutes)
	}
	var out []repository.ReferralInviterCount
	for _, t := range by {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Qualified > out[j].Qualified })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *memReferralRepo) Names(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		out[id] = r.names[id]
	}
	return out, nil
}

func (r *memReferralRepo) HasInvited(_ context.Context, id uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.refs {
		if x.ReferrerID == id {
			return true, nil
		}
	}
	return false, nil
}

// InvitersAmong is not modelled here (it reads the users table); the e2e
// suite covers the real query.
func (r *memReferralRepo) InvitersAmong(context.Context, time.Time, time.Time) (int64, error) {
	return 0, nil
}

func (r *memReferralRepo) PurgeForUser(_ context.Context, id uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	if c, ok := r.codes[id]; ok {
		delete(r.codes, id)
		delete(r.byCode, c)
		n++
	}
	for k, x := range r.refs {
		switch {
		case x.ReferrerID == id, x.RefereeID != nil && *x.RefereeID == id && x.Status == domain.ReferralJoined:
			delete(r.refs, k)
			n++
		case x.RefereeID != nil && *x.RefereeID == id:
			x.RefereeID, x.SignupIPHash, x.FirstSessionID = nil, "", nil
			n++
		}
	}
	return n, nil
}

// ---- fixture ---------------------------------------------------------------

type walletEvent struct {
	user   uuid.UUID
	typ    string
	wallet WalletView
}

type referralEvents struct {
	NoopPublisher
	mu     sync.Mutex
	events []walletEvent
}

func (e *referralEvents) Publish(_ context.Context, typ string, data any, ids ...uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range ids {
		ev := walletEvent{user: id, typ: typ}
		if w, ok := data.(WalletView); ok {
			ev.wallet = w
		}
		e.events = append(e.events, ev)
	}
	return nil
}

type referralFixture struct {
	t       *testing.T
	svc     *ReferralService
	site    *siteFixture
	repo    *memReferralRepo
	wallets *memLedger
	notes   *recordingNotifier
	events  *referralEvents
	audit   *siteAudit
	clk     *clock.Fake
	codes   []string // candidate codes handed out by NewCode (then random)
	ctx     context.Context
}

func newReferralFixture(t *testing.T) *referralFixture {
	t.Helper()
	site := newSiteFixture(t)
	site.repo.settings[domain.SiteSettingReferralProgram] = domain.SiteSetting{Key: domain.SiteSettingReferralProgram, Version: 1,
		Value:     domain.JSONMap{"enabled": true, "reward_minutes": float64(5), "max_rewards_per_referrer": float64(20), "launched_at": nil},
		UpdatedAt: site.clk.Now()}
	ledger := newMemLedger()
	f := &referralFixture{t: t, site: site, repo: newMemReferralRepo(ledger), wallets: ledger, notes: &recordingNotifier{},
		events: &referralEvents{}, audit: site.audit, clk: site.clk, ctx: context.Background()}
	wallet := NewWalletService(nil, ledger, ledger, site.clk)
	wallet.inTx = func(context.Context) bool { return true }
	f.svc = NewReferralService(ReferralDeps{Tx: inlineTx{}, Repo: f.repo, Audit: site.audit, Wallet: wallet, Program: site.svc.ReferralProgram,
		Launch: site.svc.LaunchReferralRewards, Notifier: f.notes, Events: f.events, Clock: site.clk,
		Cfg: ReferralSettings{FrontendURL: "https://staryield.test", IPHashSecret: "test-secret-0123456789"},
		NewCode: func() string {
			if len(f.codes) > 0 {
				c := f.codes[0]
				f.codes = f.codes[1:]
				return c
			}
			return randomReferralCode()
		}})
	return f
}

// client creates an active client with a wallet and a code.
func (f *referralFixture) client(name string) (uuid.UUID, string) {
	f.t.Helper()
	id := domain.NewID()
	f.repo.active[id], f.repo.names[id] = true, name
	f.wallets.wallets[id] = &domain.Wallet{UserID: id}
	code, err := f.svc.EnsureCode(f.ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id, code
}

// friend signs up with a code and returns the new account.
func (f *referralFixture) friend(code string, in AttributionInput) uuid.UUID {
	f.t.Helper()
	return f.friendWith(code, func(uuid.UUID) AttributionInput { return in })
}

// friendWith builds the attribution once the new account's id is known (the
// sign-up links a fresh browser to the account before the referral runs).
func (f *referralFixture) friendWith(code string, attribution func(id uuid.UUID) AttributionInput) uuid.UUID {
	f.t.Helper()
	id := domain.NewID()
	f.repo.active[id] = true
	in := attribution(id)
	in.ReferralCode = code
	f.svc.Attribute(reqctx.WithMeta(f.ctx, reqctx.Meta{IP: "203.0.113.9"}), id, &in)
	return id
}

func (f *referralFixture) readingBy(id uuid.UUID) {
	f.repo.mu.Lock()
	f.repo.sessions[id] = domain.NewID()
	f.repo.mu.Unlock()
	f.svc.OnSessionSettled(f.ctx, domain.Session{ClientID: id, BilledMinutes: 2})
}

func (f *referralFixture) verify(id uuid.UUID) {
	f.repo.mu.Lock()
	f.repo.verified[id] = true
	f.repo.mu.Unlock()
	f.svc.OnEmailVerified(f.ctx, id)
}

func (f *referralFixture) status(friend uuid.UUID) domain.Referral {
	f.t.Helper()
	r, err := f.repo.ByReferee(f.ctx, friend)
	if err != nil {
		f.t.Fatalf("no referral for %s: %v", friend, err)
	}
	return *r
}

func (f *referralFixture) setProgram(enabled bool, minutes, max int) {
	f.t.Helper()
	cur, err := f.site.svc.ReferralProgramSetting(f.site.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	v := cur.Version
	if _, err := f.site.svc.UpdateReferralProgram(f.site.ctx, ReferralProgramInput{Enabled: &enabled, RewardMinutes: &minutes,
		MaxRewardsPerReferrer: &max, Version: &v}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *referralFixture) launch() *ReferralLaunchResult {
	f.t.Helper()
	res, err := f.svc.Launch(f.site.ctx, ReferralLaunchConfirmation)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

func (f *referralFixture) freeMinutes(id uuid.UUID) int { return f.wallets.wallets[id].FreeMinutes }

func (f *referralFixture) kinds() []string {
	f.notes.mu.Lock()
	defer f.notes.mu.Unlock()
	var out []string
	for _, s := range f.notes.spec {
		out = append(out, s.Kind)
	}
	return out
}

// ---- codes -----------------------------------------------------------------

func TestReferralCodesUseTheUnambiguousAlphabet(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		c := randomReferralCode()
		if len(c) != 8 || !referralCodePattern.MatchString(c) || strings.ContainsAny(c, "0O1ILU") {
			t.Fatalf("bad code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 1990 {
		t.Fatalf("codes collide too often: %d distinct of 2000", len(seen))
	}
	if len(referralAlphabet) != 30 {
		t.Fatalf("alphabet has %d symbols", len(referralAlphabet))
	}
}

func TestNormalizeReferralCode(t *testing.T) {
	for in, want := range map[string]string{" abcd2345 ": "ABCD2345", "ABCD2345": "ABCD2345", "abcd-2345": "", "ABCD234": "", "ABCD23450": "",
		"ABCDO345": "", "ABCDI345": "", "": "", "ABCD234\n5": ""} {
		got, ok := NormalizeReferralCode(in)
		if (want == "") == ok || got != want && want != "" {
			t.Errorf("NormalizeReferralCode(%q) = %q, %v (want %q)", in, got, ok, want)
		}
	}
}

func TestEnsureCodeIsStableAndRetriesCollisions(t *testing.T) {
	f := newReferralFixture(t)
	first, code := f.client("Ada")
	if code == "" {
		t.Fatal("no code")
	}
	f.codes = []string{code, code, "BCDFGHJK"} // two collisions with Ada's code, then a free one
	other := domain.NewID()
	got, err := f.svc.EnsureCode(f.ctx, other)
	if err != nil || got != "BCDFGHJK" {
		t.Fatalf("collision retry: %q %v", got, err)
	}
	again, err := f.svc.EnsureCode(f.ctx, first)
	if err != nil || again != code {
		t.Fatalf("a second call must return the same code: %q %v", again, err)
	}
	// Endless collisions fail instead of looping.
	f.codes = nil
	f.svc.d.NewCode = func() string { return code }
	if _, err := f.svc.EnsureCode(f.ctx, domain.NewID()); err == nil {
		t.Fatal("expected an error after the retries")
	}
}

// ---- joining ---------------------------------------------------------------

func TestAttributeRecordsTheFriendAndNotifiesWithoutNames(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	friend := f.friend(" "+strings.ToLower(code)+" ", AttributionInput{})
	r := f.status(friend)
	if r.ReferrerID != inviter || r.Status != domain.ReferralJoined || r.Flagged || r.Code != code {
		t.Fatalf("referral: %+v", r)
	}
	if got := f.kinds(); len(got) != 1 || got[0] != NotifyReferralJoined {
		t.Fatalf("notifications: %v", got)
	}
	spec := f.notes.spec[0]
	if len(spec.Data) != 0 || spec.Link != "/profile/invite" {
		t.Fatalf("the joined notice must carry no data: %+v", spec)
	}
	// The same sign-up twice does not duplicate or notify again.
	f.svc.Attribute(f.ctx, friend, &AttributionInput{ReferralCode: code})
	if len(f.repo.refs) != 1 || len(f.kinds()) != 1 {
		t.Fatalf("duplicate: %d rows, %v", len(f.repo.refs), f.kinds())
	}
}

func TestAttributeIgnoresWhatItShould(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	// Nil input, empty, malformed, unknown codes.
	f.svc.Attribute(f.ctx, domain.NewID(), nil)
	for _, c := range []string{"", "   ", "short", "ABCDEFGH1", "OOOOOOOO", "ZZZZZZZZ"} {
		f.friend(c, AttributionInput{})
	}
	// The inviter cannot refer themselves.
	f.svc.Attribute(f.ctx, inviter, &AttributionInput{ReferralCode: code})
	// An inviter who is no longer an active client (banned, deleted, an advisor).
	f.repo.active[inviter] = false
	f.friend(code, AttributionInput{})
	f.repo.active[inviter] = true
	// The program is disabled.
	f.setProgram(false, 5, 20)
	f.friend(code, AttributionInput{})
	if len(f.repo.refs) != 0 || len(f.kinds()) != 0 {
		t.Fatalf("nothing should have been recorded: %d rows, %v", len(f.repo.refs), f.kinds())
	}
}

func TestAttributeRejectsABrowserAlreadyLinkedToTheInviter(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	f.repo.visitors["ANON-BROWSER-1"] = inviter
	friend := f.friend(code, AttributionInput{AnonymousID: "ANON-BROWSER-1"})
	r := f.status(friend)
	if r.Status != domain.ReferralRejected || r.RejectReason != domain.ReferralRejectSelf {
		t.Fatalf("self referral by browser: %+v", r)
	}
	if len(f.kinds()) != 0 {
		t.Fatalf("a rejected sign-up must not notify: %v", f.kinds())
	}
	// It never qualifies afterwards.
	f.verify(friend)
	f.readingBy(friend)
	if f.status(friend).Status != domain.ReferralRejected {
		t.Fatal("rejected referral qualified")
	}
	// A browser already linked to another account is held for review (one
	// person, several accounts); a fresh browser (linked to the new account
	// by the sign-up itself) is fine.
	f.repo.visitors["ANON-BROWSER-2"] = domain.NewID()
	if r := f.status(f.friend(code, AttributionInput{AnonymousID: "ANON-BROWSER-2"})); r.Status != domain.ReferralJoined || !r.Flagged ||
		len(r.Flags) != 1 || r.Flags[0] != domain.ReferralFlagSharedBrowser {
		t.Fatalf("browser of another account: %+v", r)
	}
	fresh := f.friendWith(code, func(id uuid.UUID) AttributionInput {
		f.repo.visitors["ANON-BROWSER-3"] = id
		return AttributionInput{AnonymousID: "ANON-BROWSER-3"}
	})
	if r := f.status(fresh); r.Status != domain.ReferralJoined || r.Flagged {
		t.Fatalf("fresh browser: %+v", r)
	}
}

func TestAttributeFlagsSharedNetworkAndBursts(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	f.repo.networks[inviter] = security.Keyed("test-secret-0123456789", "203.0.113.9", 32)
	friend := f.friend(code, AttributionInput{})
	r := f.status(friend)
	if !r.Flagged || len(r.Flags) != 1 || r.Flags[0] != domain.ReferralFlagSameNetwork || r.Status != domain.ReferralJoined {
		t.Fatalf("same network: %+v", r)
	}
	if len(f.kinds()) != 0 {
		t.Fatalf("a flagged sign-up is not announced: %v", f.kinds())
	}

	// More than 10 sign-ups in 24 hours: the 11th and later are held.
	f2 := newReferralFixture(t)
	_, code2 := f2.client("Bea")
	for i := 1; i <= 12; i++ {
		fr := f2.friend(code2, AttributionInput{})
		wantFlag := i > referralBurstLimit
		if got := f2.status(fr).Flagged; got != wantFlag {
			t.Fatalf("sign-up %d flagged=%v, want %v", i, got, wantFlag)
		}
	}
	// The window slides: a day later new sign-ups are not held.
	f2.clk.Advance(25 * time.Hour)
	if f2.status(f2.friend(code2, AttributionInput{})).Flagged {
		t.Fatal("burst window did not slide")
	}
	if n := len(f2.kinds()); n != 10+1 {
		t.Fatalf("only unflagged sign-ups notify: %d notifications", n)
	}
}

// ---- qualifying -------------------------------------------------------------

func TestQualificationIsIndependentOfTheOrderOfEvents(t *testing.T) {
	for name, order := range map[string]func(f *referralFixture, friend uuid.UUID){
		"verified then reading": func(f *referralFixture, friend uuid.UUID) { f.verify(friend); f.readingBy(friend) },
		"reading then verified": func(f *referralFixture, friend uuid.UUID) { f.readingBy(friend); f.verify(friend) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newReferralFixture(t)
			_, code := f.client("Ada")
			friend := f.friend(code, AttributionInput{})
			order(f, friend)
			r := f.status(friend)
			if r.Status != domain.ReferralQualified || r.RewardMinutes != 5 || r.QualifiedAt == nil || r.FirstSessionID == nil {
				t.Fatalf("referral: %+v", r)
			}
			// Before launch the inviter is told the minutes are pending, once.
			kinds := f.kinds()
			if len(kinds) != 2 || kinds[1] != NotifyReferralQualified {
				t.Fatalf("notifications: %v", kinds)
			}
			if d := f.notes.spec[1].Data; d["minutes"] != 5 || d["launched"] != false {
				t.Fatalf("qualified data: %v", d)
			}
			f.svc.Evaluate(f.ctx, friend) // idempotent
			f.verify(friend)
			f.readingBy(friend)
			if len(f.kinds()) != 2 || f.status(friend).Status != domain.ReferralQualified {
				t.Fatalf("a second evaluation changed something: %v", f.kinds())
			}
		})
	}
}

func TestOnlyAVerifiedFriendWithAReadingQualifies(t *testing.T) {
	f := newReferralFixture(t)
	_, code := f.client("Ada")
	a, b, c := f.friend(code, AttributionInput{}), f.friend(code, AttributionInput{}), f.friend(code, AttributionInput{})
	f.readingBy(a) // reading, not verified
	f.verify(b)    // verified, no reading
	f.verify(c)
	f.svc.OnSessionSettled(f.ctx, domain.Session{ClientID: c}) // a settled session without any minute
	f.svc.OnSessionSettled(f.ctx, domain.Session{ClientID: domain.NewID(), BilledMinutes: 3})
	for _, id := range []uuid.UUID{a, b, c} {
		if s := f.status(id).Status; s != domain.ReferralJoined {
			t.Fatalf("friend %s is %s", id, s)
		}
	}
	// A free minute alone is a reading too.
	f.repo.sessions[c] = domain.NewID()
	f.svc.OnSessionSettled(f.ctx, domain.Session{ClientID: c, FreeMinutesUsed: 1})
	if f.status(c).Status != domain.ReferralQualified {
		t.Fatal("a reading with a free minute did not qualify")
	}
}

func TestRewardMinutesAreTheSettingAtQualification(t *testing.T) {
	f := newReferralFixture(t)
	_, code := f.client("Ada")
	early, late := f.friend(code, AttributionInput{}), f.friend(code, AttributionInput{})
	f.verify(early)
	f.readingBy(early)
	f.setProgram(true, 8, 20)
	f.verify(late)
	f.readingBy(late)
	if f.status(early).RewardMinutes != 5 || f.status(late).RewardMinutes != 8 {
		t.Fatalf("minutes: %d and %d", f.status(early).RewardMinutes, f.status(late).RewardMinutes)
	}
}

func TestPerInviterCap(t *testing.T) {
	f := newReferralFixture(t)
	f.setProgram(true, 5, 3)
	inviter, code := f.client("Ada")
	var friends []uuid.UUID
	for i := 0; i < 5; i++ {
		fr := f.friend(code, AttributionInput{})
		friends = append(friends, fr)
		f.verify(fr)
		f.readingBy(fr)
	}
	for i, fr := range friends {
		want := domain.ReferralQualified
		if i >= 3 {
			want = domain.ReferralRejected
		}
		if got := f.status(fr); got.Status != want || (want == domain.ReferralRejected && got.RejectReason != domain.ReferralRejectCap) {
			t.Fatalf("friend %d: %+v", i, got)
		}
	}
	st, _ := f.svc.Mine(f.ctx, inviter)
	if st.Stats.Joined != 3 || st.Stats.Qualified != 3 || st.Stats.RemainingRewards != 0 || st.Stats.PendingMinutes != 15 {
		t.Fatalf("stats: %+v", st.Stats)
	}
	// Granted rewards keep counting toward the cap.
	f.launch()
	if err := f.svc.RunGrants(f.ctx); err != nil {
		t.Fatal(err)
	}
	extra := f.friend(code, AttributionInput{})
	f.verify(extra)
	f.readingBy(extra)
	if f.status(extra).Status != domain.ReferralRejected {
		t.Fatal("granted rewards must count toward the cap")
	}
	// A held friend over the cap is closed, and no longer waits for a review.
	f.repo.networks[inviter] = security.Keyed("test-secret-0123456789", "203.0.113.9", 32)
	held := f.friend(code, AttributionInput{})
	f.verify(held)
	f.readingBy(held)
	if r := f.status(held); r.Status != domain.ReferralRejected || r.Flagged || len(r.Flags) != 1 {
		t.Fatalf("held friend over the cap: %+v", r)
	}
}

func TestRewardsPauseWhileTheProgramIsDisabledAndTheSweepResumesThem(t *testing.T) {
	f := newReferralFixture(t)
	_, code := f.client("Ada")
	friend := f.friend(code, AttributionInput{})
	f.setProgram(false, 5, 20)
	f.verify(friend)
	f.readingBy(friend)
	if f.status(friend).Status != domain.ReferralJoined {
		t.Fatal("qualified while the program was disabled")
	}
	if err := f.svc.RunSweep(f.ctx); err != nil || f.status(friend).Status != domain.ReferralJoined {
		t.Fatalf("sweep while disabled: %v", err)
	}
	f.setProgram(true, 5, 20)
	if err := f.svc.RunSweep(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.status(friend).Status != domain.ReferralQualified {
		t.Fatalf("the sweep did not qualify the friend: %+v", f.status(friend))
	}
}

func TestSweepCatchesAMissedHook(t *testing.T) {
	f := newReferralFixture(t)
	_, code := f.client("Ada")
	friend := f.friend(code, AttributionInput{})
	// Both facts are true but no hook ran (a restart between commit and hook).
	f.repo.verified[friend], f.repo.sessions[friend] = true, domain.NewID()
	if err := f.svc.RunSweep(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.status(friend).Status != domain.ReferralQualified {
		t.Fatal("sweep did not qualify")
	}
}

// ---- launch and grants -------------------------------------------------------

func TestGrantsWaitForLaunchThenAddMinutesOnceAndAnnounceThem(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	var friends []uuid.UUID
	for i := 0; i < 3; i++ {
		fr := f.friend(code, AttributionInput{})
		friends = append(friends, fr)
		f.verify(fr)
		f.readingBy(fr)
	}
	if err := f.svc.RunGrants(f.ctx); err != nil || f.freeMinutes(inviter) != 0 {
		t.Fatalf("granted before launch: %d %v", f.freeMinutes(inviter), err)
	}
	pending, _ := f.svc.Mine(f.ctx, inviter)
	if pending.Program.Launched || pending.Stats.PendingMinutes != 15 || pending.Stats.GrantedMinutes != 0 {
		t.Fatalf("before launch: %+v", pending)
	}
	res := f.launch()
	if res.Rewards != 3 || res.Minutes != 15 || res.LaunchedAt.IsZero() {
		t.Fatalf("launch result: %+v", res)
	}
	select {
	case <-f.svc.Wake():
	default:
		t.Fatal("the launch did not wake the grant job")
	}
	for i := 0; i < 3; i++ { // idempotent: repeated runs add nothing more
		if err := f.svc.RunGrants(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.freeMinutes(inviter); got != 15 {
		t.Fatalf("free minutes: %d", got)
	}
	for _, fr := range friends {
		if r := f.status(fr); r.Status != domain.ReferralGranted || r.GrantedAt == nil {
			t.Fatalf("referral: %+v", r)
		}
	}
	var granted []NotificationSpec
	for _, s := range f.notes.spec {
		if s.Kind == NotifyReferralRewardGranted {
			granted = append(granted, s)
		}
	}
	if len(granted) != 1 || granted[0].Data["minutes"] != 15 || granted[0].Data["friends"] != 3 {
		t.Fatalf("one aggregated notice per inviter expected: %+v", granted)
	}
	var published int
	for _, e := range f.events.events {
		if e.typ == "wallet.updated" && e.user == inviter && e.wallet.FreeMinutes == 15 {
			published++
		}
	}
	if published != 1 {
		t.Fatalf("wallet.updated events: %+v", f.events.events)
	}
	mine, _ := f.svc.Mine(f.ctx, inviter)
	if !mine.Program.Launched || mine.Stats.GrantedMinutes != 15 || mine.Stats.PendingMinutes != 0 || mine.Stats.Qualified != 3 {
		t.Fatalf("after launch: %+v", mine)
	}
}

func TestQualifyingAfterLaunchIsGrantedByTheNextRunWithoutAPendingNotice(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	f.launch()
	<-f.svc.Wake()
	friend := f.friend(code, AttributionInput{})
	f.verify(friend)
	f.readingBy(friend)
	select {
	case <-f.svc.Wake():
	default:
		t.Fatal("a reward that qualifies after launch must wake the grant job")
	}
	if err := f.svc.RunGrants(f.ctx); err != nil || f.freeMinutes(inviter) != 5 {
		t.Fatalf("free minutes: %d, %v", f.freeMinutes(inviter), err)
	}
	for _, k := range f.kinds() {
		if k == NotifyReferralQualified {
			t.Fatalf("no 'pending until launch' notice after launch: %v", f.kinds())
		}
	}
}

func TestLaunchNeedsTheTypedConfirmationAndHappensOnce(t *testing.T) {
	f := newReferralFixture(t)
	for _, bad := range []string{"", "launch", "LAUNCH ", "yes"} {
		_, err := f.svc.Launch(f.site.ctx, bad)
		e, ok := apperror.As(err)
		if !ok || e.Status != http.StatusUnprocessableEntity && e.Status != http.StatusBadRequest || e.Details["fields"] == nil {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if f.site.svc.ReferralProgram(f.ctx).Launched() {
		t.Fatal("launched without the confirmation")
	}
	f.launch()
	first := f.site.svc.ReferralProgram(f.ctx).LaunchedAt
	_, err := f.svc.Launch(f.site.ctx, ReferralLaunchConfirmation)
	if e, ok := apperror.As(err); !ok || e.Status != http.StatusConflict || e.Code != CodeReferralLaunched {
		t.Fatalf("second launch: %v", err)
	}
	if got := f.site.svc.ReferralProgram(f.ctx).LaunchedAt; got == nil || !got.Equal(*first) {
		t.Fatal("the launch date changed")
	}
	if !auditHas(f.audit, "site.referral_program.launched") {
		t.Fatal("the launch is not audited")
	}
	// Editing the program afterwards keeps the launch date.
	f.setProgram(true, 7, 30)
	if got := f.site.svc.ReferralProgram(f.ctx); got.LaunchedAt == nil || got.RewardMinutes != 7 {
		t.Fatalf("program: %+v", got)
	}
}

func auditHas(a *siteAudit, action string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.rows {
		if r.Action == action {
			return true
		}
	}
	return false
}

func TestGrantsSkipFlaggedRowsAndInactiveInviters(t *testing.T) {
	f := newReferralFixture(t)
	f.repo.networks = map[uuid.UUID]string{}
	ada, adaCode := f.client("Ada")
	bea, beaCode := f.client("Bea")
	f.repo.networks[ada] = security.Keyed("test-secret-0123456789", "203.0.113.9", 32)
	flagged := f.friend(adaCode, AttributionInput{}) // same network: held for review
	clean := f.friend(beaCode, AttributionInput{})
	for _, fr := range []uuid.UUID{flagged, clean} {
		f.verify(fr)
		f.readingBy(fr)
	}
	f.repo.active[bea] = false // banned after the reading
	// Neither reward is promised to the admin: one is held, the other's
	// inviter is no longer active (the grant job would skip it).
	if res := f.launch(); res.Rewards != 0 || res.Minutes != 0 {
		t.Fatalf("launch result: %+v", res)
	}
	if sum, err := f.svc.Summary(f.ctx); err != nil || sum.ReadyRewards != 0 || sum.ReadyMinutes != 0 || sum.Qualified != 2 {
		t.Fatalf("summary: %+v %v", sum, err)
	}
	if err := f.svc.RunGrants(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.freeMinutes(ada) != 0 || f.freeMinutes(bea) != 0 {
		t.Fatalf("nothing should be granted: %d %d", f.freeMinutes(ada), f.freeMinutes(bea))
	}
	// A qualified flagged referral announced nothing before the decision.
	if n := f.notes.sent[ada]; n != 0 {
		t.Fatalf("a flagged referral announced %d times", n)
	}
	// Approving releases it.
	r := f.status(flagged)
	if _, err := f.svc.Approve(f.site.ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RunGrants(f.ctx); err != nil || f.freeMinutes(ada) != 5 {
		t.Fatalf("approved but not granted: %d %v", f.freeMinutes(ada), err)
	}
	if !auditHas(f.audit, "referral.approved") {
		t.Fatal("approval is not audited")
	}
}

func TestGrantsLoopOverManyBatchesAndInviters(t *testing.T) {
	f := newReferralFixture(t)
	f.setProgram(true, 2, 500)
	const inviters, perInviter = 12, 40 // 480 rewards: three batches of 200
	var owners []uuid.UUID
	for i := 0; i < inviters; i++ {
		id, code := f.client(fmt.Sprintf("Inviter %d", i))
		owners = append(owners, id)
		for j := 0; j < perInviter; j++ {
			fr := f.friend(code, AttributionInput{})
			// Bypass the burst hold: these are many legitimate friends over time.
			f.repo.refs[f.repo.byReferee(fr).ID].Flagged = false
			f.repo.refs[f.repo.byReferee(fr).ID].Flags = nil
			f.verify(fr)
			f.readingBy(fr)
		}
	}
	f.launch()
	if err := f.svc.RunGrants(f.ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range owners {
		if got := f.freeMinutes(id); got != perInviter*2 {
			t.Fatalf("inviter %s: %d minutes", id, got)
		}
	}
	for _, r := range f.repo.refs {
		if r.Status != domain.ReferralGranted {
			t.Fatalf("left behind: %+v", r)
		}
	}
}

// userNotes records who received which notification.
type userNotes struct {
	mu   sync.Mutex
	sent map[uuid.UUID][]NotificationSpec
}

func (n *userNotes) Notify(_ context.Context, spec NotificationSpec, ids ...uuid.UUID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, id := range ids {
		n.sent[id] = append(n.sent[id], spec)
	}
}

func TestGrantsAnnounceOncePerInviterPerRunAcrossBatches(t *testing.T) {
	f := newReferralFixture(t)
	f.setProgram(true, 3, 500)
	const inviters, perInviter = 3, 150 // 450 rewards: three batches of 200
	owners := make([]uuid.UUID, inviters)
	codes := make([]string, inviters)
	for i := range owners {
		owners[i], codes[i] = f.client(fmt.Sprintf("Inviter %d", i))
	}
	// Friends of every inviter qualify in turn, so each inviter's rewards are
	// spread over every batch (claimed oldest first).
	for j := 0; j < perInviter; j++ {
		for i := range owners {
			fr := f.friend(codes[i], AttributionInput{})
			f.repo.refs[f.repo.byReferee(fr).ID].Flagged = false // many legitimate friends over time
			f.repo.refs[f.repo.byReferee(fr).ID].Flags = nil
			f.clk.Advance(time.Second)
			f.verify(fr)
			f.readingBy(fr)
		}
	}
	f.launch()
	notes := &userNotes{sent: map[uuid.UUID][]NotificationSpec{}}
	f.svc.d.Notifier = notes
	if err := f.svc.RunGrants(f.ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range owners {
		if got := f.freeMinutes(id); got != perInviter*3 {
			t.Fatalf("inviter %s: %d minutes", id, got)
		}
		got := notes.sent[id]
		if len(got) != 1 || got[0].Kind != NotifyReferralRewardGranted || got[0].Data["minutes"] != perInviter*3 ||
			got[0].Data["friends"] != perInviter {
			t.Fatalf("inviter %s: one notice with the run's total expected, got %+v", id, got)
		}
		var balances []int
		for _, e := range f.events.events {
			if e.typ == "wallet.updated" && e.user == id {
				balances = append(balances, e.wallet.FreeMinutes)
			}
		}
		// Live balances follow every batch; the last one is the total.
		if len(balances) != 3 || balances[2] != perInviter*3 {
			t.Fatalf("inviter %s balances: %v", id, balances)
		}
	}
	// A second run has nothing left to grant and announces nothing.
	if err := f.svc.RunGrants(f.ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range owners {
		if n := len(notes.sent[id]); n != 1 {
			t.Fatalf("inviter %s: %d notices after an empty run", id, n)
		}
	}
}

func TestGrantOfAnInviterWithoutAWalletDoesNotBlockTheQueue(t *testing.T) {
	f := newReferralFixture(t)
	ada, adaCode := f.client("Ada")
	bea, beaCode := f.client("Bea")
	delete(f.wallets.wallets, ada)
	for _, code := range []string{adaCode, beaCode} {
		fr := f.friend(code, AttributionInput{})
		f.verify(fr)
		f.readingBy(fr)
	}
	f.launch()
	if err := f.svc.RunGrants(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.freeMinutes(bea) != 5 {
		t.Fatal("the other inviter was not served")
	}
	var closed int
	for _, r := range f.repo.refs {
		if r.ReferrerID == ada && r.Status == domain.ReferralRejected && r.RejectReason == rejectNoWallet {
			closed++
		}
	}
	if closed != 1 {
		t.Fatal("the row without a wallet was not closed")
	}
}

func TestGrantFreeMinutesNeedsATransaction(t *testing.T) {
	ledger := newMemLedger()
	w := NewWalletService(nil, ledger, ledger, clock.NewFake(time.Now()))
	w.inTx = func(context.Context) bool { return false }
	id := domain.NewID()
	ledger.wallets[id] = &domain.Wallet{UserID: id}
	if _, err := w.GrantFreeMinutes(context.Background(), id, 5); err != ErrNotInTransaction {
		t.Fatalf("outside a transaction: %v", err)
	}
	w.inTx = func(context.Context) bool { return true }
	if _, err := w.GrantFreeMinutes(context.Background(), id, 0); err == nil {
		t.Fatal("a zero grant must fail")
	}
	got, err := w.GrantFreeMinutes(context.Background(), id, 5)
	if err != nil || got.FreeMinutes != 5 || ledger.wallets[id].FreeMinutes != 5 {
		t.Fatalf("grant: %+v %v", got, err)
	}
}

// ---- the inviter's view and privacy ---------------------------------------------

func TestMineIsCountsOnly(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	friend := f.friend(code, AttributionInput{})
	f.friend(code, AttributionInput{})
	f.verify(friend)
	f.readingBy(friend)
	ctx := reqctx.WithPrincipal(reqctx.WithMeta(f.ctx, reqctx.Meta{Locale: "fr"}), reqctx.Principal{UserID: inviter, Role: domain.RoleClient})
	out, err := f.svc.Mine(ctx, inviter)
	if err != nil {
		t.Fatal(err)
	}
	if out.Link != "https://staryield.test/fr?ref="+code || out.Code != code {
		t.Fatalf("link: %q", out.Link)
	}
	want := ReferralStatsView{Joined: 2, Qualified: 1, PendingMinutes: 5, RemainingRewards: 19}
	if out.Stats != want {
		t.Fatalf("stats: %+v", out.Stats)
	}
	if out.Program != (ReferralProgramView{Enabled: true, RewardMinutes: 5, MaxRewards: 20}) {
		t.Fatalf("program: %+v", out.Program)
	}
	raw, _ := json.Marshal(out)
	for _, forbidden := range []string{friend.String(), "referee", "flag", "name", "email", "ip"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("the response leaks %q: %s", forbidden, raw)
		}
	}
	if _, err := f.svc.Mine(reqctx.WithPrincipal(f.ctx, reqctx.Principal{UserID: inviter, Role: domain.RolePsychic}), inviter); err == nil {
		t.Fatal("an advisor has no invite link")
	}
}

func TestPublicConfigAnnouncesTheProgram(t *testing.T) {
	f := newReferralFixture(t)
	res, err := f.site.svc.Config(f.ctx, "en")
	if err != nil {
		t.Fatal(err)
	}
	if got := res.View.Referrals; !got.Enabled || got.RewardMinutes != 5 || got.Launched {
		t.Fatalf("referrals: %+v", got)
	}
	f.launch()
	f.setProgram(false, 9, 20)
	res, _ = f.site.svc.Config(f.ctx, "en")
	if got := res.View.Referrals; got.Enabled || got.RewardMinutes != 9 || !got.Launched {
		t.Fatalf("referrals: %+v", got)
	}
}

// ---- admin -----------------------------------------------------------------------

func TestAdminListSummaryApproveAndReject(t *testing.T) {
	f := newReferralFixture(t)
	inviter, code := f.client("Ada")
	f.repo.networks[inviter] = security.Keyed("test-secret-0123456789", "203.0.113.9", 32)
	held := f.friend(code, AttributionInput{})
	f.repo.networks[inviter] = ""
	f.clk.Advance(time.Minute)
	open := f.friend(code, AttributionInput{})
	f.clk.Advance(time.Minute)
	done := f.friend(code, AttributionInput{})
	f.verify(done)
	f.readingBy(done)

	page, err := f.svc.List(f.site.ctx, "", nil, "", 2)
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" || page.Items[0].Referee == nil || page.Items[0].Referrer.Name != "Ada" {
		t.Fatalf("page: %+v %v", page, err)
	}
	next, err := f.svc.List(f.site.ctx, "", nil, page.NextCursor, 2)
	if err != nil || len(next.Items) != 1 || next.NextCursor != "" || next.Items[0].ID != f.status(held).ID {
		t.Fatalf("next page: %+v %v", next, err)
	}
	yes := true
	flagged, _ := f.svc.List(f.site.ctx, "", &yes, "", 10)
	if len(flagged.Items) != 1 || flagged.Items[0].Flags[0] != domain.ReferralFlagSameNetwork {
		t.Fatalf("flagged: %+v", flagged)
	}
	if q, _ := f.svc.List(f.site.ctx, "qualified", nil, "", 10); len(q.Items) != 1 {
		t.Fatalf("qualified: %+v", q)
	}
	if _, err := f.svc.List(f.site.ctx, "bogus", nil, "", 10); err == nil {
		t.Fatal("a bad status was accepted")
	}
	if _, err := f.svc.List(f.site.ctx, "", nil, "not-a-cursor", 10); err == nil {
		t.Fatal("a bad cursor was accepted")
	}

	sum, err := f.svc.Summary(f.site.ctx)
	if err != nil || sum.Joined != 2 || sum.Qualified != 1 || sum.Flagged != 1 || sum.PendingMinutes != 5 || sum.ReadyRewards != 1 ||
		len(sum.TopInviters) != 1 || sum.TopInviters[0].User.Name != "Ada" || sum.TopInviters[0].Qualified != 1 {
		t.Fatalf("summary: %+v %v", sum, err)
	}

	// Approve only what is held; reject only what is open.
	if _, err := f.svc.Approve(f.site.ctx, f.status(open).ID); err == nil {
		t.Fatal("an unflagged referral cannot be approved")
	}
	approved, err := f.svc.Approve(f.site.ctx, f.status(held).ID)
	if err != nil || approved.Flagged || approved.DecidedBy == nil || approved.Status != "joined" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	// Rejecting a held referral ends the hold; its flags stay as history.
	f.repo.networks[inviter] = security.Keyed("test-secret-0123456789", "203.0.113.9", 32)
	heldAgain := f.friend(code, AttributionInput{})
	f.repo.networks[inviter] = ""
	if rj, err := f.svc.Reject(f.site.ctx, f.status(heldAgain).ID, "Same household"); err != nil || rj.Flagged || len(rj.Flags) != 1 {
		t.Fatalf("reject a held referral: %+v %v", rj, err)
	}
	if still, _ := f.svc.List(f.site.ctx, "", &yes, "", 10); len(still.Items) != 0 {
		t.Fatalf("decided referrals are not held: %+v", still)
	}
	for _, bad := range []string{"", "no", strings.Repeat("x", 201)} {
		if _, err := f.svc.Reject(f.site.ctx, f.status(open).ID, bad); err == nil {
			t.Fatalf("reason %q accepted", bad)
		}
	}
	rejected, err := f.svc.Reject(f.site.ctx, f.status(open).ID, "  Same person  ")
	if err != nil || rejected.Status != "rejected" || rejected.RejectReason != "Same person" {
		t.Fatalf("reject: %+v %v", rejected, err)
	}
	if _, err := f.svc.Reject(f.site.ctx, f.status(open).ID, "Again"); err == nil {
		t.Fatal("a rejected referral cannot be rejected again")
	}
	if _, err := f.svc.Approve(f.site.ctx, uuid.New()); err == nil {
		t.Fatal("unknown referral")
	}
	// A rejected friend never qualifies; rejecting a qualified one frees a place under the cap.
	f.verify(open)
	f.readingBy(open)
	if f.status(open).Status != domain.ReferralRejected {
		t.Fatal("a rejected referral qualified")
	}
	if rj, err := f.svc.Reject(f.site.ctx, f.status(done).ID, "Abuse"); err != nil || rj.Status != "rejected" {
		t.Fatalf("reject qualified: %+v %v", rj, err)
	}
	if !auditHas(f.audit, "referral.rejected") {
		t.Fatal("rejection is not audited")
	}
}

func TestProgramSettingValidationAndVersions(t *testing.T) {
	f := newReferralFixture(t)
	yes, no := true, false
	zero, big, ok, sixty, five := 0, 501, 20, 60, 5
	for name, in := range map[string]ReferralProgramInput{
		"nothing":     {},
		"zero":        {Enabled: &yes, RewardMinutes: &zero, MaxRewardsPerReferrer: &ok},
		"too many":    {Enabled: &yes, RewardMinutes: &five, MaxRewardsPerReferrer: &big},
		"too long":    {Enabled: &yes, RewardMinutes: &big, MaxRewardsPerReferrer: &ok},
		"bad version": {Enabled: &yes, RewardMinutes: &five, MaxRewardsPerReferrer: &ok, Version: &zero},
	} {
		if _, err := f.site.svc.UpdateReferralProgram(f.site.ctx, in); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	got, err := f.site.svc.UpdateReferralProgram(f.site.ctx, ReferralProgramInput{Enabled: &no, RewardMinutes: &sixty, MaxRewardsPerReferrer: &ok})
	if err != nil || got.Enabled || got.RewardMinutes != 60 || got.Version != 2 || got.UpdatedBy == nil {
		t.Fatalf("update: %+v %v", got, err)
	}
	stale := 1
	_, err = f.site.svc.UpdateReferralProgram(f.site.ctx, ReferralProgramInput{Enabled: &yes, RewardMinutes: &five, MaxRewardsPerReferrer: &ok, Version: &stale})
	if e, isApp := apperror.As(err); !isApp || e.Status != http.StatusConflict {
		t.Fatalf("stale version: %v", err)
	}
	same, _ := f.site.svc.UpdateReferralProgram(f.site.ctx, ReferralProgramInput{Enabled: &no, RewardMinutes: &sixty, MaxRewardsPerReferrer: &ok})
	if same.Version != 2 {
		t.Fatal("an unchanged value must not be a new version")
	}
	if !auditHas(f.audit, "site.referral_program.updated") {
		t.Fatal("the update is not audited")
	}
}

func TestProgramFailsClosedWhenUnreadable(t *testing.T) {
	f := newReferralFixture(t)
	f.site.repo.fail = true
	fresh := f.site.instance() // a second instance with nothing cached locally
	f.site.kv = nil
	if p := fresh.ReferralProgram(f.ctx); p.Enabled || p.Launched() {
		t.Fatalf("an unreadable program must read as disabled: %+v", p)
	}
}

func TestPurgeForUser(t *testing.T) {
	f := newReferralFixture(t)
	ada, adaCode := f.client("Ada")
	bea, beaCode := f.client("Bea")
	joined := f.friend(adaCode, AttributionInput{})
	earned := f.friend(adaCode, AttributionInput{})
	f.verify(earned)
	f.readingBy(earned)
	ofBea := f.friend(beaCode, AttributionInput{})

	// Erasing the inviter deletes their code and the friends they invited.
	if n, err := f.svc.PurgeForUser(f.ctx, ada); err != nil || n != 1+2 {
		t.Fatalf("purge inviter: %d %v", n, err)
	}
	if _, err := f.repo.CodeOf(f.ctx, ada); err == nil {
		t.Fatal("the code survived")
	}
	if _, err := f.repo.ByReferee(f.ctx, joined); err == nil {
		t.Fatal("the invites survived")
	}
	// Erasing a friend who never finished deletes the row; one who did keeps only the decision.
	f.verify(ofBea)
	f.readingBy(ofBea)
	if _, err := f.svc.PurgeForUser(f.ctx, ofBea); err != nil {
		t.Fatal(err)
	}
	if st, _ := f.repo.Stats(f.ctx, bea); st.Qualified != 1 {
		t.Fatalf("the earned reward must stay after the friend is erased: %+v", st)
	}
	for _, r := range f.repo.refs {
		if r.RefereeID != nil {
			t.Fatalf("a friend's identity survived: %+v", r)
		}
	}
}
