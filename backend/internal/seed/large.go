package seed

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// LoadEmailDomain marks synthetic load-test accounts (reserved .test TLD).
const LoadEmailDomain = "load.staryield.test"

// LoadPassword is the password of every synthetic account.
const LoadPassword = "Load!Test#2026"

// LargeOptions sizes the synthetic dataset used for load and strain tests.
type LargeOptions struct {
	Users                   int // clients
	Psychics                int // advisors
	Conversations           int // unique client–advisor threads
	MessagesPerConversation int // average messages per conversation
	Reviews                 int // upper bound (one per settled session at most)
	LedgerRows              int // credit movements across all clients
	Seed                    uint64
	BatchSize               int
	// Force seeds again even if load data exists (adds a new, separately
	// tagged population).
	Force bool
	// Sealer encrypts generated sensitive data (client notes) exactly like
	// the API does, so the API can read it back. Generators that need it
	// skip their work when it is nil.
	Sealer *security.Sealer
}

// DefaultLargeOptions is the "large" profile.
func DefaultLargeOptions() LargeOptions {
	return LargeOptions{Users: 200_000, Psychics: 2_000, Conversations: 100_000, MessagesPerConversation: 20,
		Reviews: 150_000, LedgerRows: 1_000_000, Seed: 42, BatchSize: 2000}
}

// Validate checks the sizes.
func (o LargeOptions) Validate() error {
	switch {
	case o.Users < 1 || o.Psychics < 1:
		return errors.New("seed: need at least one client and one advisor")
	case o.Conversations < 0 || o.Conversations > o.Users*o.Psychics:
		return errors.New("seed: conversations must be within 0..users×psychics")
	case o.MessagesPerConversation < 0 || o.Reviews < 0 || o.LedgerRows < 0:
		return errors.New("seed: sizes must not be negative")
	case o.BatchSize < 1 || o.BatchSize > 5000:
		return errors.New("seed: batch size must be within 1..5000")
	}
	return nil
}

// ErrAlreadySeeded is returned when load data exists and Force is off.
var ErrAlreadySeeded = errors.New("seed: load data already present (use -force to add another population)")

// Generator extends the large dataset; later milestones register theirs
// (notes, subscriptions, waitlists…) with RegisterLarge.
type Generator struct {
	Name string
	Run  func(ctx context.Context, d *Dataset) error
}

var (
	genMu      sync.Mutex
	generators []Generator
)

// RegisterLarge adds a generator run after the built-in ones, in
// registration order.
func RegisterLarge(g Generator) {
	genMu.Lock()
	defer genMu.Unlock()
	generators = append(generators, g)
}

// SessionRef is a generated chat session generators can attach data to.
type SessionRef struct {
	ID, ConversationID, ClientID, PsychicID uuid.UUID
	StartedAt, EndedAt                      time.Time
	Settled                                 bool
}

// ConversationRef is a generated client–advisor thread.
type ConversationRef struct {
	ID, ClientID, PsychicID uuid.UUID
}

// Dataset is the generated population, shared with generators.
type Dataset struct {
	DB            *gorm.DB
	Rand          *mrand.Rand
	Opts          LargeOptions
	Tag           string
	Now           time.Time
	Log           *zap.Logger
	Clients       []uuid.UUID
	Psychics      []uuid.UUID
	Conversations []ConversationRef
	Sessions      []SessionRef
	// Sealer is LargeOptions.Sealer (may be nil).
	Sealer *security.Sealer
	// Counts records rows inserted per table.
	Counts map[string]int
}

// Report summarises a run.
type Report struct {
	Tag      string
	Counts   map[string]int
	Duration time.Duration
}

const historyWindow = 18 * 30 * 24 * time.Hour

// Large builds the synthetic dataset. It is deterministic for a seed: the
// same options generate the same people, conversations and amounts (only the
// population tag differs between forced runs).
func Large(ctx context.Context, db *gorm.DB, hasher *security.PasswordHasher, opts LargeOptions, log *zap.Logger) (*Report, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	var existing int64
	if err := db.WithContext(ctx).Model(&domain.User{}).Where("email LIKE ?", "%@"+LoadEmailDomain).Count(&existing).Error; err != nil {
		return nil, err
	}
	tag := fmt.Sprintf("s%d", opts.Seed)
	if existing > 0 {
		if !opts.Force {
			return nil, ErrAlreadySeeded
		}
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		tag += "x" + hex.EncodeToString(b)
	}
	hash, err := hasher.Hash(LoadPassword)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	// The tag is part of the stream so a forced population never repeats the
	// IDs of an earlier one; a fresh database always gets tag "s<seed>".
	h := fnv.New64a()
	_, _ = h.Write([]byte(tag))
	d := &Dataset{DB: db.WithContext(ctx), Rand: mrand.New(mrand.NewPCG(opts.Seed, h.Sum64())), Opts: opts, Tag: tag,
		Now: time.Now().UTC().Truncate(time.Second), Log: log, Sealer: opts.Sealer, Counts: map[string]int{}}
	steps := []Generator{
		{Name: "advisors", Run: func(ctx context.Context, d *Dataset) error { return d.advisors(hash) }},
		{Name: "clients", Run: func(ctx context.Context, d *Dataset) error { return d.clients(hash) }},
		{Name: "chats", Run: func(ctx context.Context, d *Dataset) error { return d.chats() }},
		{Name: "advisor aggregates", Run: func(ctx context.Context, d *Dataset) error { return d.aggregates() }},
	}
	genMu.Lock()
	steps = append(steps, generators...)
	genMu.Unlock()
	for _, g := range steps {
		t := time.Now()
		if err := g.Run(ctx, d); err != nil {
			return nil, fmt.Errorf("seed %s: %w", g.Name, err)
		}
		log.Info("load data generated", zap.String("step", g.Name), zap.Duration("elapsed", time.Since(t)))
	}
	return &Report{Tag: tag, Counts: d.Counts, Duration: time.Since(start)}, nil
}

// ID returns a UUIDv7 for a row created at `at`, with seeded random bits:
// time-ordered like production IDs, reproducible for a seed.
func (d *Dataset) ID(at time.Time) uuid.UUID {
	var u uuid.UUID
	ms := uint64(at.UnixMilli())
	u[0], u[1], u[2], u[3], u[4], u[5] = byte(ms>>40), byte(ms>>32), byte(ms>>24), byte(ms>>16), byte(ms>>8), byte(ms)
	binary.BigEndian.PutUint64(u[8:], d.Rand.Uint64())
	r := d.Rand.Uint32()
	u[6], u[7] = 0x70|byte(r&0x0f), byte(r>>8)
	u[8] = 0x80 | (u[8] & 0x3f)
	return u
}

// Past returns a time uniformly within the history window.
func (d *Dataset) Past() time.Time {
	return d.Now.Add(-time.Duration(d.Rand.Int64N(int64(historyWindow)))).Truncate(time.Second)
}

// After returns a time between t and now, biased towards t.
func (d *Dataset) After(t time.Time, max time.Duration) time.Time {
	span := d.Now.Sub(t)
	if span <= 0 {
		return d.Now
	}
	if max > 0 && span > max {
		span = max
	}
	f := d.Rand.Float64()
	return t.Add(time.Duration(f * f * float64(span))).Truncate(time.Second)
}

func (d *Dataset) pick(weights ...int) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	n := d.Rand.IntN(total)
	for i, w := range weights {
		if n < w {
			return i
		}
		n -= w
	}
	return len(weights) - 1
}

func (d *Dataset) insert(table string, rows any, n int, selectAll bool) error {
	if n == 0 {
		return nil
	}
	q := d.DB
	if selectAll {
		// Explicit zero values (false switches) must not be replaced by
		// column defaults.
		q = q.Select("*")
	}
	if err := q.CreateInBatches(rows, d.Opts.BatchSize).Error; err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	d.Counts[table] += n
	return nil
}

var (
	firstNames = []string{"Nova", "Luna", "Aria", "Milo", "Iris", "Orion", "Selene", "Theo", "Maya", "Leo", "Zara", "Elio", "Nadia", "Rafael",
		"Ines", "Mateo", "Camille", "Hugo", "Sofia", "Jules", "Amara", "Kai", "Lea", "Noah", "Clara", "Omar", "Yara", "Ezra", "Lina", "Arlo"}
	cities = []struct {
		name, country, tz string
		lat, lon          float64
	}{
		{"New York, United States", "US", "America/New_York", 40.7128, -74.006},
		{"Memphis, Tennessee, United States", "US", "America/Chicago", 35.1495, -90.049},
		{"Los Angeles, United States", "US", "America/Los_Angeles", 34.0522, -118.2437},
		{"Paris, France", "FR", "Europe/Paris", 48.8566, 2.3522},
		{"Madrid, Spain", "ES", "Europe/Madrid", 40.4168, -3.7038},
		{"Mexico City, Mexico", "MX", "America/Mexico_City", 19.4326, -99.1332},
		{"London, United Kingdom", "GB", "Europe/London", 51.5072, -0.1276},
		{"Montréal, Canada", "CA", "America/Toronto", 45.5019, -73.5674},
		{"Buenos Aires, Argentina", "AR", "America/Argentina/Buenos_Aires", -34.6037, -58.3816},
		{"Tunis, Tunisia", "TN", "Africa/Tunis", 36.8065, 10.1815},
	}
	skills    = []string{"Tarot Reading", "Astrology", "Numerology", "Aura Reading", "Palmistry", "Dream Analysis", "Crystal Reading", "Mediumship", "Past Life"}
	zodiac    = []string{"Aries", "Taurus", "Gemini", "Cancer", "Leo", "Virgo", "Libra", "Scorpio", "Sagittarius", "Capricorn", "Aquarius", "Pisces"}
	headlines = []string{"Clarity for love, career and the crossroads in between.", "Grounded tarot guidance for real decisions.",
		"Astrological timing for the seasons of your life.", "Intuitive readings with a practical plan."}
	phrases = []string{"Hello, I have a question about my relationship.", "Welcome. Let us look at what the cards reveal.",
		"I sense a strong bond, but also tension that needs attention.", "That resonates so much. What should I do?",
		"The Lovers paired with the Five of Cups speaks of love and unmet expectations.", "Give it until the next full moon.",
		"Thank you, this helps a lot.", "Trust your intuition on this one.", "Your path is lit by the stars.", "Can we look at my career next?"}
	reviewComments = []string{"Kind and thoughtful.", "Helped me see things clearly.", "Very insightful reading.", "Good, but a little rushed.",
		"Exactly what I needed today.", "Not what I expected."}
)

// newAccount builds a user with profile and wallet (the caller sets the
// wallet balance and inserts).
func (d *Dataset) newAccount(email, hash string, role domain.Role, name string, created time.Time) (domain.User, domain.Profile, domain.Wallet) {
	locale := []string{"en", "fr", "es"}[d.pick(70, 15, 15)]
	status := domain.UserActive
	deactivatedBy := ""
	switch d.pick(970, 15, 10, 5) {
	case 1:
		status, deactivatedBy = domain.UserDeactivated, domain.DeactivatedBySelf
	case 2:
		status, deactivatedBy = domain.UserDeactivated, domain.DeactivatedByAdmin
	case 3:
		status = domain.UserBanned
	}
	last := d.After(created, 0)
	u := domain.User{ID: d.ID(created), Email: email, PasswordHash: hash, Role: role, Status: status, DisplayName: name, Locale: locale,
		TermsAcceptedAt: created, LastLoginAt: &last, DeactivatedBy: deactivatedBy, CreatedAt: created, UpdatedAt: last}
	if status != domain.UserActive {
		u.StatusChangedAt = &last
	}
	u.AdultConfirmedAt = &created // revisions 5.0: the profile below has an adult birth date (§24.6)
	city := cities[d.Rand.IntN(len(cities))]
	birth := time.Date(1950+d.Rand.IntN(56), time.Month(1+d.Rand.IntN(12)), 1+d.Rand.IntN(28), 0, 0, 0, 0, time.UTC)
	p := domain.Profile{UserID: u.ID, BirthDate: &birth, BirthPlace: city.name, BirthPlaceLabel: city.name, BirthTimezone: city.tz,
		BirthLatitude: &city.lat, BirthLongitude: &city.lon, CountryCode: city.country, Location: city.name[:min(len(city.name), 60)],
		Mood: "Empowered", Theme: []string{"Warm Beige", "Light Cream"}[d.pick(85, 15)], Timezone: city.tz,
		HouseSystem:     []domain.HouseSystem{domain.HousePlacidus, domain.HouseWholeSign, domain.HouseEqual}[d.pick(80, 15, 5)],
		NotifyHoroscope: d.Rand.IntN(10) < 7, NotifyEmail: d.Rand.IntN(10) < 8, NotifyPush: d.Rand.IntN(10) < 3,
		NotifyMarketing: d.Rand.IntN(10) < 2, ShareBirthDetailsWithAdvisors: d.Rand.IntN(10) < 4, CreatedAt: created, UpdatedAt: last}
	if d.Rand.IntN(10) < 7 {
		p.BirthTime = fmt.Sprintf("%02d:%02d", d.Rand.IntN(24), d.Rand.IntN(60))
	}
	w := domain.Wallet{UserID: u.ID, FreeMinutes: d.Rand.IntN(4), CreatedAt: created, UpdatedAt: last}
	return u, p, w
}

func (d *Dataset) advisors(hash string) error {
	n := d.Opts.Psychics
	chunk := d.Opts.BatchSize
	for start := 0; start < n; start += chunk {
		size := min(chunk, n-start)
		users := make([]domain.User, 0, size)
		profiles := make([]domain.Profile, 0, size)
		wallets := make([]domain.Wallet, 0, size)
		advisors := make([]domain.Psychic, 0, size)
		var services []domain.PsychicService
		for i := start; i < start+size; i++ {
			created := d.Past()
			name := firstNames[d.Rand.IntN(len(firstNames))]
			email := fmt.Sprintf("load-%s-p%06d@%s", d.Tag, i, LoadEmailDomain)
			u, p, w := d.newAccount(email, hash, domain.RolePsychic, name, created)
			u.Status, u.DeactivatedBy, u.StatusChangedAt = domain.UserActive, "", nil
			users, profiles, wallets = append(users, u), append(profiles, p), append(wallets, w)
			sk := make([]string, 0, 3)
			for _, j := range d.Rand.Perm(len(skills))[:1+d.Rand.IntN(3)] {
				sk = append(sk, skills[j])
			}
			availability := []domain.Availability{domain.AvailabilityOnline, domain.AvailabilityAway, domain.AvailabilityOffline}[d.pick(20, 10, 70)]
			lastOnline := d.After(created, 0)
			ps := domain.Psychic{UserID: u.ID, Slug: fmt.Sprintf("load-%s-p%06d", strings.ToLower(d.Tag), i), Name: fmt.Sprintf("%s %d", name, i),
				Headline: headlines[d.Rand.IntN(len(headlines))], About: domain.StringList{"Synthetic advisor for load testing."},
				Zodiac: zodiac[d.Rand.IntN(len(zodiac))], Skills: sk, Rating: 5, YearsExperience: 1 + d.Rand.IntN(30),
				RatePerMinute: int64(20 + d.Rand.IntN(61)), Availability: availability, Listed: d.Rand.IntN(100) < 95, SortOrder: 1000 + i,
				Featured: d.Rand.IntN(100) < 2, LastOnlineAt: &lastOnline, PhotoReviewStatus: domain.PhotoApproved, CreatedAt: created, UpdatedAt: lastOnline}
			advisors = append(advisors, ps)
			for k, s := range []string{"Schedule a Reading", "Love Spread"} {
				services = append(services, domain.PsychicService{ID: d.ID(created), PsychicID: u.ID, Name: s, Subtitle: "30 Min Live Session",
					Description: "A focused reading on one question.", PriceCredits: int64(500 + d.Rand.IntN(1500)), Active: true, SortOrder: k,
					Kind: domain.ServiceStandard, DeliveryDays: 7, CreatedAt: created, UpdatedAt: created})
			}
			if d.Rand.IntN(10) < 3 {
				services = append(services, domain.PsychicService{ID: d.ID(created), PsychicID: u.ID, Name: "Custom reading", Active: true,
					PriceCredits: int64(800 + d.Rand.IntN(2000)), SortOrder: 9, Kind: domain.ServiceCustom, DeliveryDays: 1 + d.Rand.IntN(14),
					CreatedAt: created, UpdatedAt: created})
			}
			d.Psychics = append(d.Psychics, u.ID)
		}
		if err := d.insertAccounts(users, profiles, wallets); err != nil {
			return err
		}
		if err := d.insert("psychics", &advisors, len(advisors), false); err != nil {
			return err
		}
		if err := d.insert("psychic_services", &services, len(services), true); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dataset) insertAccounts(users []domain.User, profiles []domain.Profile, wallets []domain.Wallet) error {
	if err := d.insert("users", &users, len(users), false); err != nil {
		return err
	}
	if err := d.insert("user_profiles", &profiles, len(profiles), true); err != nil {
		return err
	}
	return d.insert("wallets", &wallets, len(wallets), true)
}

func (d *Dataset) clients(hash string) error {
	n := d.Opts.Users
	chunk := d.Opts.BatchSize
	perUser, extra := 0, 0
	if d.Opts.LedgerRows > 0 {
		perUser, extra = d.Opts.LedgerRows/n, d.Opts.LedgerRows%n
	}
	for start := 0; start < n; start += chunk {
		size := min(chunk, n-start)
		users := make([]domain.User, 0, size)
		profiles := make([]domain.Profile, 0, size)
		wallets := make([]domain.Wallet, 0, size)
		var ledger []domain.LedgerEntry
		var lots []domain.CreditLot // revisions 5.0: credit lots
		for i := start; i < start+size; i++ {
			created := d.Past()
			name := firstNames[d.Rand.IntN(len(firstNames))] + " " + string(rune('A'+d.Rand.IntN(26))) + "."
			email := fmt.Sprintf("load-%s-c%07d@%s", d.Tag, i, LoadEmailDomain)
			u, p, w := d.newAccount(email, hash, domain.RoleClient, name, created)
			entries := perUser
			if i < extra {
				entries++
			}
			at := created
			for k := 0; k < entries; k++ {
				at = d.After(at, 30*24*time.Hour)
				e := domain.LedgerEntry{ID: d.ID(at), UserID: u.ID, ReferenceType: "seed", IdempotencyKey: fmt.Sprintf("load:%s:%d:%d", d.Tag, i, k), CreatedAt: at}
				if k == 0 || w.Balance < 100 || d.Rand.IntN(4) == 0 {
					e.EntryType, e.Amount, e.Description = domain.LedgerPurchase, []int64{250, 600, 1500}[d.pick(50, 35, 15)], "Credit pack"
				} else {
					e.EntryType, e.Amount, e.Description = domain.LedgerChatCharge, -min(w.Balance, int64(20+d.Rand.IntN(400))), "Live chat"
				}
				w.Balance += e.Amount
				e.BalanceAfter = w.Balance
				ledger = append(ledger, e)
			}
			if w.Balance > 0 {
				// revisions 5.0: the synthetic ledger has no purchase rows, so the balance
				// is one legacy bonus lot, exactly what migration 0025 gives such a wallet.
				lots = append(lots, domain.CreditLot{ID: d.ID(at), UserID: u.ID, Source: domain.LotBonus, Priority: domain.LotBonus.Priority(),
					Origin: domain.LotOriginLegacyBackfill, CreditsTotal: w.Balance, CreditsRemaining: w.Balance, CreatedAt: created, UpdatedAt: at})
			}
			users, profiles, wallets = append(users, u), append(profiles, p), append(wallets, w)
			d.Clients = append(d.Clients, u.ID)
		}
		if err := d.insertAccounts(users, profiles, wallets); err != nil {
			return err
		}
		if err := d.insert("credit_ledger", &ledger, len(ledger), false); err != nil {
			return err
		}
		if err := d.insert("credit_lots", &lots, len(lots), true); err != nil {
			return err
		}
		if start/chunk%10 == 0 {
			d.Log.Info("clients generated", zap.Int("done", start+size), zap.Int("total", n))
		}
	}
	return nil
}

func (d *Dataset) chats() error {
	n := d.Opts.Conversations
	if n == 0 {
		return nil
	}
	pairs := make(map[[2]int]bool, n)
	// Expected settled sessions ≈ conversations × 1.65 × 0.7: review rate
	// that hits the requested review count on average.
	reviewP := float64(d.Opts.Reviews) / (float64(n) * 1.65 * 0.7)
	chunk := max(1, d.Opts.BatchSize/4)
	for start := 0; start < n; start += chunk {
		size := min(chunk, n-start)
		convs := make([]domain.Conversation, 0, size)
		var sessions []domain.Session
		var messages []domain.Message
		var reviews []domain.Review
		var earnings []domain.Earning
		for len(convs) < size {
			ci, pi := d.Rand.IntN(len(d.Clients)), d.Rand.IntN(len(d.Psychics))
			if pairs[[2]int{ci, pi}] {
				continue
			}
			pairs[[2]int{ci, pi}] = true
			client, psychic := d.Clients[ci], d.Psychics[pi]
			created := d.Past()
			conv := domain.Conversation{ID: d.ID(created), ClientID: client, PsychicID: psychic, LastActivityAt: created, CreatedAt: created, UpdatedAt: created}
			at := created
			nSessions := 1 + d.pick(50, 35, 15)
			var started []int
			for s := 0; s < nSessions; s++ {
				at = d.After(at, 20*24*time.Hour)
				rate := int64(20 + d.Rand.IntN(61))
				sess := domain.Session{ID: d.ID(at), ConversationID: conv.ID, ClientID: client, PsychicID: psychic, RatePerMinute: rate, ListRatePerMinute: rate,
					RequestedAt: at, ExpiresAt: at.Add(90 * time.Second), CreatedAt: at, UpdatedAt: at}
				sess.RequestKey = "load-" + sess.ID.String()
				switch d.pick(70, 10, 8, 8, 4) {
				case 0, 1: // settled or ended
					accepted, begun := at.Add(20*time.Second), at.Add(30*time.Second)
					minutes := 1 + int(d.Rand.ExpFloat64()*12)
					if minutes > 90 {
						minutes = 90
					}
					ended := begun.Add(time.Duration(minutes) * time.Minute)
					free := 0
					if d.Rand.IntN(10) < 3 {
						free = min(minutes, 1+d.Rand.IntN(3))
					}
					credits := int64(minutes-free) * rate
					net := credits * 60 / 100
					sess.AcceptedAt, sess.StartedAt, sess.LastResumedAt, sess.EndedAt = &accepted, &begun, &begun, &ended
					sess.ActiveSeconds, sess.BilledMinutes, sess.FreeMinutesUsed = int64(minutes*60), minutes, free
					sess.CreditsCharged, sess.PsychicEarnings = credits, net
					sess.EndReason = []domain.EndReason{domain.EndClientEnded, domain.EndPsychicEnded, domain.EndCreditsExhausted}[d.pick(70, 20, 10)]
					sess.Status, sess.UpdatedAt = domain.SessionEnded, ended
					settled := d.pick(9, 1) == 0
					if settled {
						st := ended.Add(5 * time.Second)
						sess.Status, sess.SettledAt, sess.UpdatedAt = domain.SessionSettled, &st, st
						if credits > 0 {
							earnings = append(earnings, domain.Earning{ID: d.ID(st), PsychicID: psychic, SourceType: domain.EarningChatSession,
								SourceID: sess.ID, EntryKind: domain.EarningKindEarning, GrossCredits: credits, CommissionCredits: credits - net,
								NetCredits: net, SharePercent: 60, CreatedAt: st})
						}
						if d.Rand.Float64() < reviewP {
							ra := d.After(st, 7*24*time.Hour)
							rating := 5 - d.pick(60, 25, 8, 4, 3)
							rv := domain.Review{ID: d.ID(ra), SessionID: sess.ID, ClientID: client, PsychicID: psychic, Rating: rating,
								Status: domain.ReviewVisible, CreatedAt: ra, UpdatedAt: ra}
							if d.Rand.IntN(10) < 7 {
								rv.Comment = reviewComments[d.Rand.IntN(len(reviewComments))]
							}
							if d.Rand.IntN(100) < 3 {
								rv.Status = domain.ReviewHidden
							}
							reviews = append(reviews, rv)
						}
					}
					started = append(started, len(sessions))
					d.Sessions = append(d.Sessions, SessionRef{ID: sess.ID, ConversationID: conv.ID, ClientID: client, PsychicID: psychic,
						StartedAt: begun, EndedAt: ended, Settled: settled})
					at = ended
				case 2:
					sess.Status, sess.EndReason = domain.SessionDeclined, domain.EndDeclined
				case 3:
					sess.Status, sess.EndReason = domain.SessionExpired, domain.EndRequestTimeout
				default:
					sess.Status, sess.EndReason = domain.SessionCancelled, domain.EndCancelled
				}
				sessions = append(sessions, sess)
			}
			// Messages, spread over the sessions that took place: each session
			// gets a share, at increasing times inside its active window, with
			// client and advisor taking turns.
			if len(started) > 0 && d.Opts.MessagesPerConversation > 0 {
				total := max(1, d.Opts.MessagesPerConversation/2+d.Rand.IntN(d.Opts.MessagesPerConversation+1))
				var last *domain.Message
				// Messages are numbered 1..N per conversation (0007_chat_ordering).
				var seq int64
				var senders []uuid.UUID
				for k, idx := range started {
					s := &sessions[idx]
					share := total / len(started)
					if k < total%len(started) {
						share++
					}
					span := s.EndedAt.Sub(*s.StartedAt)
					for m := 0; m < share; m++ {
						mat := s.StartedAt.Add(span * time.Duration(m+1) / time.Duration(share+1)).Truncate(time.Millisecond)
						sender := client
						if m%2 == 1 {
							sender = psychic
						}
						seq++
						senders = append(senders, sender)
						sessionID := s.ID
						messages = append(messages, domain.Message{ID: d.ID(mat), ConversationID: conv.ID, SessionID: &sessionID, SenderID: &sender,
							Kind: domain.MessageText, Body: phrases[d.Rand.IntN(len(phrases))], Seq: seq, CreatedAt: mat, UpdatedAt: mat})
						last = &messages[len(messages)-1]
					}
				}
				if last != nil {
					// last_message_id is a foreign key: it is set once the
					// messages exist (see below); read markers are plain times.
					t := last.CreatedAt
					conv.UpdatedAt, conv.LastActivityAt = t, t
					read := t
					conv.PsychicLastReadAt = &read
					conv.LastSeq, conv.PsychicReadSeq = seq, seq
					conv.ClientReadSeq = seq
					if d.Rand.IntN(10) < 9 {
						conv.ClientLastReadAt = &read
					} else {
						// The client has not read the last few messages: count
						// the advisor's among them as unread.
						back := min(seq, int64(1+d.Rand.IntN(3)))
						conv.ClientReadSeq = seq - back
						for _, from := range senders[len(senders)-int(back):] {
							if from != client {
								conv.ClientUnread++
							}
						}
					}
				}
			}
			convs = append(convs, conv)
			d.Conversations = append(d.Conversations, ConversationRef{ID: conv.ID, ClientID: client, PsychicID: psychic})
		}
		for _, step := range []struct {
			table string
			rows  any
			n     int
		}{{"conversations", &convs, len(convs)}, {"chat_sessions", &sessions, len(sessions)}, {"chat_messages", &messages, len(messages)},
			{"psychic_reviews", &reviews, len(reviews)}, {"psychic_earnings", &earnings, len(earnings)}} {
			if err := d.insert(step.table, step.rows, step.n, false); err != nil {
				return err
			}
		}
		ids := make([]uuid.UUID, len(convs))
		for i := range convs {
			ids[i] = convs[i].ID
		}
		if err := d.DB.Model(&domain.Conversation{}).Where("id IN ?", ids).UpdateColumns(map[string]any{
			"last_message_id": gorm.Expr("(SELECT m.id FROM chat_messages m WHERE m.conversation_id = conversations.id ORDER BY m.created_at DESC, m.id DESC LIMIT 1)"),
			"last_message_at": gorm.Expr("(SELECT max(m.created_at) FROM chat_messages m WHERE m.conversation_id = conversations.id)"),
		}).Error; err != nil {
			return fmt.Errorf("conversation last message: %w", err)
		}
		if start/chunk%20 == 0 {
			d.Log.Info("conversations generated", zap.Int("done", start+size), zap.Int("total", n),
				zap.Int("messages", d.Counts["chat_messages"]))
		}
	}
	return nil
}

// aggregates recomputes the advisors' rating and reading counters from the
// generated reviews and sessions.
func (d *Dataset) aggregates() error {
	for start := 0; start < len(d.Psychics); start += 500 {
		ids := d.Psychics[start:min(start+500, len(d.Psychics))]
		q := d.DB.Model(&domain.Psychic{}).Where("user_id IN ?", ids)
		if err := q.UpdateColumns(map[string]any{
			"reviews_count":  gorm.Expr("(SELECT count(*) FROM psychic_reviews r WHERE r.psychic_id = psychics.user_id AND r.status = ?)", domain.ReviewVisible),
			"rating_sum":     gorm.Expr("(SELECT COALESCE(sum(r.rating), 0) FROM psychic_reviews r WHERE r.psychic_id = psychics.user_id AND r.status = ?)", domain.ReviewVisible),
			"readings_count": gorm.Expr("(SELECT count(*) FROM chat_sessions s WHERE s.psychic_id = psychics.user_id AND s.status = ?)", domain.SessionSettled),
		}).Error; err != nil {
			return err
		}
		if err := d.DB.Model(&domain.Psychic{}).Where("user_id IN ?", ids).UpdateColumn("rating",
			gorm.Expr("CASE WHEN reviews_count > 0 THEN LEAST(5, round(rating_sum / reviews_count, 1)) ELSE 5.0 END")).Error; err != nil {
			return err
		}
	}
	return nil
}

// Extend runs only the named registered generators (e.g. "waiting lists")
// against the synthetic population already in the database, so a large
// dataset seeded before a feature existed can gain that feature's data
// without being regenerated. Clients and advisors are loaded from the
// database; generators that need generated sessions or conversations see
// none.
func Extend(ctx context.Context, db *gorm.DB, names []string, opts LargeOptions, log *zap.Logger) (*Report, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range names {
		want[strings.TrimSpace(n)] = true
	}
	genMu.Lock()
	var steps []Generator
	for _, g := range generators {
		if want[g.Name] {
			steps = append(steps, g)
			delete(want, g.Name)
		}
	}
	genMu.Unlock()
	if len(want) > 0 {
		unknown := make([]string, 0, len(want))
		for n := range want {
			unknown = append(unknown, n)
		}
		return nil, fmt.Errorf("unknown generators: %s", strings.Join(unknown, ", "))
	}
	tag := fmt.Sprintf("s%dext", opts.Seed)
	h := fnv.New64a()
	_, _ = h.Write([]byte(tag))
	d := &Dataset{DB: db.WithContext(ctx), Rand: mrand.New(mrand.NewPCG(opts.Seed, h.Sum64())), Opts: opts, Tag: tag,
		Now: time.Now().UTC().Truncate(time.Second), Log: log, Sealer: opts.Sealer, Counts: map[string]int{}}
	pattern := "%@" + LoadEmailDomain
	if err := d.DB.Model(&domain.User{}).Where("email LIKE ? AND role = ?", pattern, domain.RoleClient).Order("id").Pluck("id", &d.Clients).Error; err != nil {
		return nil, err
	}
	if err := d.DB.Model(&domain.User{}).Where("email LIKE ? AND role = ?", pattern, domain.RolePsychic).Order("id").Pluck("id", &d.Psychics).Error; err != nil {
		return nil, err
	}
	if len(d.Clients) == 0 {
		return nil, fmt.Errorf("no synthetic population to extend (seed -profile=large first)")
	}
	start := time.Now()
	for _, g := range steps {
		t := time.Now()
		if err := g.Run(ctx, d); err != nil {
			return nil, fmt.Errorf("seed %s: %w", g.Name, err)
		}
		log.Info("load data generated", zap.String("step", g.Name), zap.Duration("elapsed", time.Since(t)))
	}
	return &Report{Tag: tag, Counts: d.Counts, Duration: time.Since(start)}, nil
}
