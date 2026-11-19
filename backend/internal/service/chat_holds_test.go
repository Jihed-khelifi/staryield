package service

// revisions 6.0 (§26.1): the free-trial hold, without a database.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// holdRepo records what the hold entry writes; any other call panics (nil
// embedded interface), proving the decision touches nothing else.
type holdRepo struct {
	repository.ChatRepository
	sessions []domain.Session
	holds    []domain.ChatHold
}

func (r *holdRepo) SaveSession(_ context.Context, s *domain.Session) error {
	r.sessions = append(r.sessions, *s)
	return nil
}

func (r *holdRepo) CreateHold(_ context.Context, h *domain.ChatHold) error {
	r.holds = append(r.holds, *h)
	return nil
}

// noUsers resolves nobody: the views fall back to their generic names.
type noUsers struct{ repository.UserRepository }

func (noUsers) GetByID(context.Context, uuid.UUID) (*domain.User, error) {
	return nil, repository.ErrNotFound
}

func (noUsers) GetProfile(context.Context, uuid.UUID) (*domain.Profile, error) {
	return nil, repository.ErrNotFound
}

type noPsychics struct{ repository.PsychicRepository }

func (noPsychics) GetByUserID(context.Context, uuid.UUID) (*domain.Psychic, error) {
	return nil, repository.ErrNotFound
}

func holdFixture(settings func(context.Context) TrialHoldSettings) (*ChatService, *holdRepo, *eventLog) {
	repo, events := &holdRepo{}, &eventLog{}
	clk := clock.NewFake(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
	c := &ChatService{chat: repo, users: noUsers{}, psychicsRepo: noPsychics{}, events: events, tracker: analytics.Noop{}, clock: clk,
		cfg: ChatSettings{BillingInterval: time.Minute, SharePercent: 60}}
	c.SetTrialHold(settings)
	return c, repo, events
}

func activeSession(free int, credits int64, sponsored bool, now time.Time) *domain.Session {
	resumed := now.Add(-20 * time.Second)
	return &domain.Session{ID: uuid.New(), ConversationID: uuid.New(), ClientID: uuid.New(), PsychicID: uuid.New(), Status: domain.SessionActive,
		RatePerMinute: 40, ListRatePerMinute: 50, FreeMinutesUsed: free, CreditsCharged: credits, BilledMinutes: 1, ActiveSeconds: 40,
		Sponsored: sponsored, LastResumedAt: &resumed, NextBillingAt: &now}
}

// The decision table of the hold entry: trial or paid, the two switches, free
// mode (sponsored) and unreadable settings.
func TestHoldEntryDecisionTable(t *testing.T) {
	on := func(afterTrial, afterPaid bool) func(context.Context) TrialHoldSettings {
		return func(context.Context) TrialHoldSettings {
			return TrialHoldSettings{AfterTrial: afterTrial, AfterPaid: afterPaid, HoldSeconds: 60, PaymentGraceSeconds: 120}
		}
	}
	for _, tc := range []struct {
		name      string
		settings  func(context.Context) TrialHoldSettings
		free      int
		credits   int64
		sponsored bool
		held      bool
		reason    domain.HoldReason
	}{
		{"trial over, after_trial on", on(true, false), 1, 0, false, true, domain.HoldTrialEnded},
		{"several free minutes, after_trial on", on(true, false), 4, 0, false, true, domain.HoldTrialEnded},
		{"trial over, after_trial off", on(false, true), 1, 0, false, false, ""},
		{"paid credits out, after_paid on", on(false, true), 1, 120, false, true, domain.HoldCreditsExhausted},
		{"paid credits out, after_paid off", on(true, false), 1, 120, false, false, ""},
		{"no free minute ever, credits out, after_paid on", on(false, true), 0, 80, false, true, domain.HoldCreditsExhausted},
		{"nothing billed yet counts as paid, after_trial only", on(true, false), 0, 0, false, false, ""},
		{"nothing billed yet counts as paid, after_paid on", on(false, true), 0, 0, false, true, domain.HoldCreditsExhausted},
		{"both switches off", on(false, false), 1, 0, false, false, ""},
		{"a sponsored reading is never held (trial)", on(true, true), 1, 0, true, false, ""},
		{"a sponsored reading is never held (paid)", on(true, true), 0, 80, true, false, ""},
		{"no settings reader installed", nil, 1, 0, false, false, ""},
		{"unreadable settings are the zero value: no hold", func(context.Context) TrialHoldSettings { return TrialHoldSettings{} }, 1, 0, false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, repo, _ := holdFixture(tc.settings)
			now := c.clock.Now()
			s := activeSession(tc.free, tc.credits, tc.sponsored, now)
			var post afterCommit
			if err := c.holdOrEndLocked(context.Background(), s, now, &post); err != nil {
				t.Fatal(err)
			}
			if tc.held {
				if s.Status != domain.SessionHeld || len(repo.holds) != 1 || s.EndReason != "" || s.EndedAt != nil {
					t.Fatalf("want a held reading, got %+v with %d holds", s, len(repo.holds))
				}
				h := repo.holds[0]
				if h.Reason != tc.reason || h.Status != domain.HoldOpen || h.SessionID != s.ID || h.ClientID != s.ClientID || h.PsychicID != s.PsychicID ||
					h.ConversationID != s.ConversationID || h.PriceCredits != 40 || h.ListRateCredits != 50 || h.HeldMessages != 0 || !h.StartedAt.Equal(now) {
					t.Fatalf("hold: %+v", h)
				}
				if !h.ExpiresAt.Equal(now.Add(60*time.Second)) || !h.MaxExpiresAt.Equal(now.Add(180*time.Second)) {
					t.Fatalf("deadlines: %s / %s", h.ExpiresAt, h.MaxExpiresAt)
				}
				if s.HoldExpiresAt == nil || !s.HoldExpiresAt.Equal(h.ExpiresAt) || s.LastHoldID == nil || *s.LastHoldID != h.ID {
					t.Fatalf("session pointers: %+v", s)
				}
				// Hold time is neither billed nor active time: the stretch up to now is
				// accrued (capped by the paid minute), the clocks are cleared.
				if s.NextBillingAt != nil || s.LastResumedAt != nil || s.ActiveSeconds != 60 || s.BilledMinutes != 1 {
					t.Fatalf("clocks: next=%v resumed=%v active=%d billed=%d", s.NextBillingAt, s.LastResumedAt, s.ActiveSeconds, s.BilledMinutes)
				}
				return
			}
			if s.Status != domain.SessionEnded || s.EndReason != domain.EndCreditsExhausted || len(repo.holds) != 0 {
				t.Fatalf("want the reading ended with credits_exhausted, got %+v", s)
			}
		})
	}
}

func TestHoldDurationIsBoundedAndTheGraceIsNeverNegative(t *testing.T) {
	for _, tc := range []struct {
		in          TrialHoldSettings
		hold, grace time.Duration
	}{
		{TrialHoldSettings{HoldSeconds: 60, PaymentGraceSeconds: 120}, 60 * time.Second, 120 * time.Second},
		{TrialHoldSettings{HoldSeconds: 0, PaymentGraceSeconds: -5}, 30 * time.Second, 0},
		{TrialHoldSettings{HoldSeconds: 10000, PaymentGraceSeconds: 10000}, 300 * time.Second, 300 * time.Second},
	} {
		if tc.in.HoldDuration() != tc.hold || tc.in.PaymentGrace() != tc.grace {
			t.Errorf("%+v: hold %s grace %s", tc.in, tc.in.HoldDuration(), tc.in.PaymentGrace())
		}
	}
}

func TestHeldEventsGoToBothParties(t *testing.T) {
	c, _, events := holdFixture(func(context.Context) TrialHoldSettings {
		return TrialHoldSettings{AfterTrial: true, HoldSeconds: 60, PaymentGraceSeconds: 120}
	})
	now := c.clock.Now()
	s := activeSession(1, 0, false, now)
	var post afterCommit
	if err := c.holdOrEndLocked(context.Background(), s, now, &post); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 0 {
		t.Fatal("nothing may be published before the commit")
	}
	post.run(context.Background())
	for who, id := range map[string]uuid.UUID{"client": s.ClientID, "advisor": s.PsychicID} {
		ev := events.of(realtime.EventSessionHeld, id)
		if len(ev) != 1 {
			t.Fatalf("%s: %d session.held events", who, len(ev))
		}
		hold, ok := ev[0].data["hold"].(HoldView)
		if !ok || hold.Status != "open" || hold.Reason != "trial_ended" {
			t.Fatalf("%s: %+v", who, ev[0].data)
		}
		// The advisor is shown the list rate, never the member discount.
		want := int64(40)
		if who == "advisor" {
			want = 50
		}
		if hold.PriceCredits != want {
			t.Fatalf("%s sees price %d, want %d", who, hold.PriceCredits, want)
		}
		sv, ok := ev[0].data["session"].(SessionView)
		if !ok || sv.Status != "held" || sv.Hold == nil || sv.Hold.ID != hold.ID {
			t.Fatalf("%s: session %+v", who, ev[0].data["session"])
		}
	}
}

// ---------------------------------------------------------------------------
// The redaction
// ---------------------------------------------------------------------------

func heldView(sender uuid.UUID, body string) MessageView {
	hold := uuid.New()
	return MessageView{ID: uuid.New(), Seq: 7, SenderID: &sender, Kind: "text", Body: body, Held: true, HoldID: &hold,
		bodyLen: utf8.RuneCountInString(body), Saved: true, LikedBy: []uuid.UUID{sender},
		Attachments: []AttachmentView{{ID: uuid.New()}}, ReplyTo: &ReplyView{ID: uuid.New(), Excerpt: "quoted"}}
}

func TestRedactionTable(t *testing.T) {
	advisor, client, admin, stranger := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	long := strings.Repeat("é", 3000)
	for _, tc := range []struct {
		name       string
		body       string
		sender     *uuid.UUID
		viewer     viewer
		locked     bool
		wantLength int
	}{
		{"the client sees it locked", "the cards say yes", &advisor, viewer{ID: client}, true, 17},
		{"the advisor sees it in full, flagged", "the cards say yes", &advisor, viewer{ID: advisor}, false, 0},
		{"an administrator sees it in full, flagged", "the cards say yes", &advisor, viewer{ID: admin, Admin: true}, false, 0},
		{"any other viewer is locked out too", "the cards say yes", &advisor, viewer{ID: stranger}, true, 17},
		{"body_length counts characters, not bytes", "héllo wörld ✨", &advisor, viewer{ID: client}, true, 13},
		{"body_length is capped at 2000", long, &advisor, viewer{ID: client}, true, 2000},
		{"a held message of unknown sender fails closed", "xyzzy-secret", nil, viewer{ID: client}, true, 12},
		{"a held message of unknown sender is still full for an administrator", "xyzzy-secret", nil, viewer{Admin: true}, false, 0},
		{"an empty caption (bare image) has no length", "", &advisor, viewer{ID: client}, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := heldView(advisor, tc.body)
			v.SenderID = tc.sender
			orig := v
			got := viewFor(v, tc.viewer)
			if got.Locked != tc.locked {
				t.Fatalf("locked = %v, want %v: %+v", got.Locked, tc.locked, got)
			}
			if tc.locked {
				if got.Body != "" || got.Attachments != nil || got.ReplyTo != nil || got.LikedBy != nil || got.Saved || got.Held ||
					got.BodyLength != tc.wantLength || got.HoldID == nil || got.ID != orig.ID || got.Seq != orig.Seq {
					t.Fatalf("locked view: %+v", got)
				}
				raw, _ := json.Marshal(got)
				if tc.body != "" && strings.Contains(string(raw), tc.body) {
					t.Fatalf("the text is in the JSON: %s", raw)
				}
				for _, banned := range []string{`"attachments"`, `"reply_to"`, `"liked_by"`, `"saved"`} {
					if strings.Contains(string(raw), banned) {
						t.Fatalf("%s in %s", banned, raw)
					}
				}
				if !strings.Contains(string(raw), `"locked":true`) || strings.Contains(string(raw), `"held"`) {
					t.Fatalf("flags: %s", raw)
				}
			} else if !got.Held || got.Body != tc.body || got.Attachments == nil || got.ReplyTo == nil || got.ReplyTo.Locked {
				t.Fatalf("full view: %+v", got)
			}
			// viewFor never changes the original (the other recipient's copy).
			if v.Body != tc.body || v.Locked || !v.Held || v.ReplyTo == nil || v.Attachments == nil {
				t.Fatalf("the original was modified: %+v", v)
			}
		})
	}
}

func TestRedactionLeavesOrdinaryMessagesAlone(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	hold := uuid.New()
	views := []MessageView{
		{ID: uuid.New(), Kind: "text", Body: "hello", SenderID: &a, bodyLen: 5, Saved: true, LikedBy: []uuid.UUID{a, b}},
		{ID: uuid.New(), Kind: "system", Body: "session.started"},
		// Revealed: held is false again, hold_id stays and nothing is hidden.
		{ID: uuid.New(), Kind: "text", Body: "revealed", SenderID: &a, HoldID: &hold, bodyLen: 8},
		// A quote of a message that is not hidden keeps its excerpt.
		{ID: uuid.New(), Kind: "text", Body: "re", SenderID: &b, ReplyTo: &ReplyView{ID: uuid.New(), Excerpt: "hello", SenderID: &a}},
	}
	want, _ := json.Marshal(views)
	redactMessages(views, viewer{ID: b})
	got, _ := json.Marshal(views)
	if string(want) != string(got) {
		t.Fatalf("ordinary messages changed:\n%s\n%s", want, got)
	}
}

func TestQuoteOfALockedMessage(t *testing.T) {
	advisor, client := uuid.New(), uuid.New()
	quote := func() *ReplyView {
		return &ReplyView{ID: uuid.New(), Seq: 5, SenderID: &advisor, Kind: "text", Excerpt: "the hidden words", held: true}
	}
	reply := MessageView{ID: uuid.New(), Kind: "text", Body: "thanks", SenderID: &client, ReplyTo: quote()}
	for who, tc := range map[string]struct {
		w      viewer
		locked bool
	}{
		"the client":  {viewer{ID: client}, true},
		"the advisor": {viewer{ID: advisor}, false},
		"an admin":    {viewer{ID: uuid.New(), Admin: true}, false},
	} {
		got := viewFor(reply, tc.w)
		if got.ReplyTo == nil || got.ReplyTo.Locked != tc.locked {
			t.Fatalf("%s: %+v", who, got.ReplyTo)
		}
		if tc.locked && (got.ReplyTo.Excerpt != "" || got.ReplyTo.ID != reply.ReplyTo.ID || got.ReplyTo.Seq != 5) {
			t.Fatalf("%s: locked quote %+v", who, got.ReplyTo)
		}
		if !tc.locked && got.ReplyTo.Excerpt != "the hidden words" {
			t.Fatalf("%s: %+v", who, got.ReplyTo)
		}
		// The shared quote of the original is never modified.
		if reply.ReplyTo.Locked || reply.ReplyTo.Excerpt == "" {
			t.Fatalf("the original quote was modified: %+v", reply.ReplyTo)
		}
	}
	raw, _ := json.Marshal(viewFor(reply, viewer{ID: client}))
	if strings.Contains(string(raw), "hidden words") || !strings.Contains(string(raw), `"locked":true`) {
		t.Fatalf("client JSON: %s", raw)
	}
}

func TestInboxPreviewIsLockedForTheClientOnly(t *testing.T) {
	advisor, client := uuid.New(), uuid.New()
	hold := uuid.New()
	preview := func() *MessageView {
		return &MessageView{ID: uuid.New(), Kind: "text", Body: "a preview of the hidden text", SenderID: &advisor, Held: true, HoldID: &hold, bodyLen: 300}
	}
	items := []ConversationView{{LastMessage: preview()}, {}, {LastMessage: preview()}}
	redactLastMessages(items, viewer{ID: client})
	for _, i := range []int{0, 2} {
		lm := items[i].LastMessage
		if !lm.Locked || lm.Body != "" || lm.BodyLength != 300 || lm.Held {
			t.Fatalf("client preview: %+v", lm)
		}
	}
	if items[1].LastMessage != nil {
		t.Fatal("an empty conversation grew a preview")
	}
	items = []ConversationView{{LastMessage: preview()}}
	redactLastMessages(items, viewer{ID: advisor})
	if lm := items[0].LastMessage; lm.Locked || lm.Body == "" || !lm.Held {
		t.Fatalf("advisor preview: %+v", lm)
	}
}

func TestLockedFromGuardsLikesSavesAndAttachments(t *testing.T) {
	advisor, client := uuid.New(), uuid.New()
	held := &domain.Message{SenderID: &advisor, Held: true}
	if !lockedFrom(held, viewer{ID: client}) || lockedFrom(held, viewer{ID: advisor}) || lockedFrom(held, viewer{ID: client, Admin: true}) {
		t.Fatal("lockedFrom: the client is locked out, the sender and an admin are not")
	}
	if lockedFrom(&domain.Message{SenderID: &advisor}, viewer{ID: client}) {
		t.Fatal("an ordinary message is never locked")
	}
	if !lockedFrom(&domain.Message{Held: true}, viewer{ID: client}) {
		t.Fatal("a held message of unknown sender fails closed")
	}
}

func TestHoldViewRedactsThePriceForTheAdvisor(t *testing.T) {
	h := &domain.ChatHold{ID: uuid.New(), ClientID: uuid.New(), PsychicID: uuid.New(), Status: domain.HoldOpen, PriceCredits: 24, ListRateCredits: 30, HeldMessages: 3}
	if got := holdView(h, viewer{ID: h.ClientID}).PriceCredits; got != 24 {
		t.Fatalf("client: %d", got)
	}
	if got := holdView(h, viewer{ID: h.PsychicID}).PriceCredits; got != 30 {
		t.Fatalf("advisor: %d", got)
	}
	if got := holdView(h, viewer{Admin: true}).PriceCredits; got != 24 {
		t.Fatalf("admin: %d", got)
	}
}

// ---------------------------------------------------------------------------
// Push, settings and statistics
// ---------------------------------------------------------------------------

func TestHeldMessagePushSaysTopUpAndNeverTheText(t *testing.T) {
	conv := uuid.New()
	for _, tc := range []struct{ locale, name, want string }{
		{"en", "Luna", "Luna sent you a message. Top up to read it."},
		{"fr", "Luna", "Luna vous a envoyé un message. Rechargez pour le lire."},
		{"es", "Luna", "Luna te ha enviado un mensaje. Recarga para leerlo."},
		{"en", "", "Someone sent you a message. Top up to read it."},
		{"fr", "", "Vous avez reçu un message. Rechargez pour le lire."},
	} {
		p := HeldMessagePush("https://staryield.net", tc.locale, tc.name, "luna", conv)
		if p.Kind != PushKindChatMessage || p.Body != tc.want || p.Tag != "chat:"+conv.String() || !strings.Contains(p.URL, "reader=luna") {
			t.Errorf("%s/%q: %+v", tc.locale, tc.name, p)
		}
	}
	// The ordinary push is unchanged.
	if p := ChatMessagePush("https://staryield.net", "en", "Luna", "luna", false, conv); p.Body != "Open the chatroom to read it." {
		t.Fatalf("ordinary push: %+v", p)
	}
}

func TestTrialHoldReaderFailsClosed(t *testing.T) {
	f := newSiteFixture(t)
	f.repo.settings[domain.SiteSettingTrialHold] = domain.SiteSetting{Key: "trial_hold", Version: 1,
		Value: domain.JSONMap{"after_trial": true, "after_paid": false, "hold_seconds": float64(60), "payment_grace_seconds": float64(120)}}
	f.svc.dropLocal()
	if got := f.svc.TrialHold(f.ctx); !got.AfterTrial || got.AfterPaid || got.HoldSeconds != 60 || got.PaymentGraceSeconds != 120 || !got.Applies(true) || got.Applies(false) {
		t.Fatalf("stored: %+v", got)
	}
	// Unreadable and nothing cached: no hold at all.
	g := newSiteFixture(t)
	g.repo.fail = true
	if got := g.svc.TrialHold(g.ctx); got.Applies(true) || got.Applies(false) {
		t.Fatalf("unreadable settings must never hold a reading: %+v", got)
	}
	// A malformed document never reads as more permissive, and the bounds hold.
	h := newSiteFixture(t)
	h.repo.settings[domain.SiteSettingTrialHold] = domain.SiteSetting{Key: "trial_hold", Version: 1,
		Value: domain.JSONMap{"after_trial": "yes", "after_paid": 1, "hold_seconds": float64(99999), "payment_grace_seconds": float64(-3)}}
	h.svc.dropLocal()
	got := h.svc.TrialHold(h.ctx)
	if got.Applies(true) || got.Applies(false) || got.HoldSeconds != 300 || got.PaymentGraceSeconds != 0 {
		t.Fatalf("malformed document: %+v", got)
	}
}

type fakeHoldStats struct {
	stats repository.HoldStats
	since time.Time
	calls int
	err   error
}

func (f *fakeHoldStats) HoldStatsSince(_ context.Context, since time.Time) (repository.HoldStats, error) {
	f.calls++
	f.since = since
	return f.stats, f.err
}

func TestTrialHoldStatsRatesAndWindow(t *testing.T) {
	f := newSiteFixture(t)
	f.repo.settings[domain.SiteSettingTrialHold] = domain.SiteSetting{Key: "trial_hold", Version: 1,
		Value: domain.JSONMap{"after_trial": true, "after_paid": false, "hold_seconds": float64(60), "payment_grace_seconds": float64(120)}}
	fake := &fakeHoldStats{stats: repository.HoldStats{Started: 8, Converted: 2, Expired: 6, Revealed: 3, CreditsFromConversion: 80, CreditsFromReveals: 120}}
	f.svc.d.HoldStats = fake

	cur, err := f.svc.AdminTrialHold(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	s := cur.Stats
	if s.WindowDays != 30 || s.Started != 8 || s.Converted != 2 || s.Expired != 6 || s.Revealed != 3 || s.ConversionRate != 0.25 || s.RevealRate != 0.375 ||
		s.CreditsFromConversion != 80 || s.CreditsFromReveals != 120 {
		t.Fatalf("stats: %+v", s)
	}
	if want := f.clk.Now().AddDate(0, 0, -30); !fake.since.Equal(want) {
		t.Fatalf("window starts %s, want %s", fake.since, want)
	}
	// Cached for 60 s: a second read does not hit the database.
	if _, err := f.svc.AdminTrialHold(f.ctx); err != nil || fake.calls != 1 {
		t.Fatalf("calls %d, err %v", fake.calls, err)
	}
	raw, _ := json.Marshal(cur)
	for _, key := range []string{`"after_trial":true`, `"version":1`, `"stats":{"window_days":30`, `"credits_from_conversions":80`, `"credits_from_reveals":120`, `"conversion_rate":0.25`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("%s missing from %s", key, raw)
		}
	}
	// No holds: rates are 0, never NaN.
	g := newSiteFixture(t)
	g.repo.settings[domain.SiteSettingTrialHold] = f.repo.settings[domain.SiteSettingTrialHold]
	g.svc.d.HoldStats = &fakeHoldStats{}
	cur, _ = g.svc.AdminTrialHold(g.ctx)
	if cur.Stats.ConversionRate != 0 || cur.Stats.RevealRate != 0 || cur.Stats.Started != 0 {
		t.Fatalf("empty stats: %+v", cur.Stats)
	}
}

func TestUpdateTrialHoldValidationVersionAndAudit(t *testing.T) {
	f := newSiteFixture(t)
	seed := domain.SiteSetting{Key: "trial_hold", Version: 1, UpdatedAt: f.clk.Now(),
		Value: domain.JSONMap{"after_trial": true, "after_paid": false, "hold_seconds": float64(60), "payment_grace_seconds": float64(120)}}
	f.repo.settings[domain.SiteSettingTrialHold] = seed
	valid := func() TrialHoldInput {
		return TrialHoldInput{AfterTrial: ptr(false), AfterPaid: ptr(true), HoldSeconds: ptr(90), PaymentGraceSeconds: ptr(30), Version: ptr(1)}
	}
	for name, mutate := range map[string]func(*TrialHoldInput){
		"no switch":    func(in *TrialHoldInput) { in.AfterTrial = nil },
		"hold 29":      func(in *TrialHoldInput) { in.HoldSeconds = ptr(29) },
		"hold 301":     func(in *TrialHoldInput) { in.HoldSeconds = ptr(301) },
		"grace -1":     func(in *TrialHoldInput) { in.PaymentGraceSeconds = ptr(-1) },
		"grace 301":    func(in *TrialHoldInput) { in.PaymentGraceSeconds = ptr(301) },
		"no version":   func(in *TrialHoldInput) { in.Version = nil },
		"version zero": func(in *TrialHoldInput) { in.Version = ptr(0) },
	} {
		in := valid()
		mutate(&in)
		if _, err := f.svc.UpdateTrialHold(f.ctx, in); !isCode(err, apperror.CodeValidation) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
	fields := siteFields(t, func() error { _, err := f.svc.UpdateTrialHold(f.ctx, TrialHoldInput{}); return err }())
	if len(fields) != 5 {
		t.Fatalf("fields: %v", fields)
	}
	if fields["hold_seconds"] != msgBannerRequired {
		t.Fatalf("fields: %v", fields)
	}
	for _, in := range []TrialHoldInput{{AfterTrial: ptr(true), AfterPaid: ptr(false), HoldSeconds: ptr(30), PaymentGraceSeconds: ptr(0), Version: ptr(1)},
		{AfterTrial: ptr(true), AfterPaid: ptr(false), HoldSeconds: ptr(300), PaymentGraceSeconds: ptr(300), Version: ptr(2)}} { // the bounds themselves are valid
		if _, err := f.svc.UpdateTrialHold(f.ctx, in); err != nil {
			t.Fatalf("%+v refused: %v", in, err)
		}
	}
	stale := valid()
	if _, err := f.svc.UpdateTrialHold(f.ctx, stale); !isCode(err, apperror.CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	} else if e, _ := apperror.As(err); e.Details["current_version"] != 3 {
		t.Fatalf("details: %v", e.Details)
	}
	ok := valid()
	ok.Version = ptr(3)
	out, err := f.svc.UpdateTrialHold(f.ctx, ok)
	if err != nil || out.Version != 4 || out.AfterTrial || !out.AfterPaid || out.HoldSeconds != 90 || out.PaymentGraceSeconds != 30 || out.UpdatedBy == nil || out.UpdatedBy.Name != "Ada Admin" {
		t.Fatalf("saved: %+v %v", out, err)
	}
	// Live at once on this instance.
	if got := f.svc.TrialHold(f.ctx); got.AfterTrial || !got.AfterPaid || got.HoldSeconds != 90 {
		t.Fatalf("hook: %+v", got)
	}
	// An unchanged save neither bumps the version nor writes an audit row.
	n := len(f.audit.actions())
	same := valid()
	same.Version = ptr(4)
	again, err := f.svc.UpdateTrialHold(f.ctx, same)
	if err != nil || again.Version != 4 || len(f.audit.actions()) != n {
		t.Fatalf("no-op save: %+v %v", again, err)
	}
	last := f.audit.rows[len(f.audit.rows)-1]
	if last.Action != "site.trial_hold.updated" || last.ActorID == nil || *last.ActorID != f.admin || last.TargetType != "site_setting" || last.TargetID != "trial_hold" {
		t.Fatalf("audit: %+v", last)
	}
	before, _ := last.Metadata["before"].(map[string]any)
	after, _ := last.Metadata["after"].(map[string]any)
	if before["hold_seconds"] != 300 || before["after_trial"] != true || after["hold_seconds"] != 90 || after["after_paid"] != true || after["payment_grace_seconds"] != 30 {
		t.Fatalf("audit metadata: %v", last.Metadata)
	}
}
