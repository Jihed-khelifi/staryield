// Package seed loads idempotent reference data: the advisors and services
// shown in the design, the credit packs, and bootstrap accounts.
package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Account is a bootstrap login.
type Account struct {
	Email    string
	Password string
}

// Options controls what is seeded.
type Options struct {
	Admin           *Account
	PsychicPassword string // shared password for demo advisor logins; empty skips logins
	DemoClient      *Account
	DemoCredits     int64
}

type service struct {
	name, subtitle, description string
	price                       int64
}

type advisor struct {
	slug, name, headline, image, zodiac, philosophy string
	about, skills                                   []string
	rating                                          float64
	reviews, readings, years                        int
	rate                                            int64
	services                                        []service
}

var advisors = []advisor{
	{
		slug: "ramone", name: "Ramone", image: "https://assets.staryield.net/assets/figma/47a9f.png", zodiac: "Taurus",
		headline: "I help you see your path more clearly through Tarot and cosmic guidance.",
		skills:   []string{"Numerology", "Astrology", "Tarot Reading"}, rating: 4.9, reviews: 350, readings: 3500, years: 8, rate: 40,
		about: []string{
			"Are you seeking clarity and a deeper understanding of the options ahead of you? I believe that every answer you are searching for is already within you, and my goal is simply to help you see your path more clearly.",
			"I use Tarot because it allows me to translate the universal language of energy into practical steps for your life. By accessing cosmic guidance, numerological timelines, and intuitive energy readings, we will dissect current blockages and unlock your spiritual blueprint.",
		},
		philosophy: "\"There are no absolute deterministic outcomes—only waves of energy waiting to be realized. Every session aims to return power to your hands through conscious choices.\"",
		services: []service{
			{"Schedule a Reading", "30 Min Live Session", "Book a dedicated calendar slot for a private, uninterrupted deep-dive session focusing on career, love, or spiritual transition.", 1200},
			{"Candle Ritual", "Spiritual Attunement", "An ancient candle-burning manifestation ritual tailored to your specific numerological charts to draw alignment and luck.", 850},
		},
	},
	{
		slug: "theo", name: "Theo", image: "https://assets.staryield.net/assets/figma/1f3a8.png", zodiac: "Gemini",
		headline: "Channeling ancestral wisdom through crystal energy and dream interpretation.",
		skills:   []string{"Crystal Healing", "Mediumship", "Dream Analysis"}, rating: 4.8, reviews: 280, readings: 1200, years: 5, rate: 55,
		about: []string{
			"She channels ancestral wisdom passed down through generations of mystic tradition, forging a deep and sacred connection with spirit guides who dwell beyond the veil of ordinary perception. Through ancient rituals and meditative practices, she opens doorways to illuminate hidden truths that lie beneath the surface of everyday awareness. Drawing upon the vibrational power of carefully arranged crystal energy grids, she amplifies intuitive frequencies and harmonizes the flow of spiritual energy around you. Her gift for dream interpretation allows her to decode the symbolic language of your sleeping mind, mapping the vast and intricate terrain of your subconscious landscape. With gentle clarity, she reveals the profound messages your higher self is trying to send — guiding you toward deeper self-understanding, spiritual alignment, and the awakening of your true inner potential.",
		},
		philosophy: "\"The crystals don't lie — they amplify what your soul already knows. My role is to help you listen to the frequencies that have always been there.\"",
		services: []service{
			{"Schedule a Reading", "30 Min Live Session", "Book a dedicated calendar slot for a private, uninterrupted deep-dive session focusing on career, love, or spiritual transition.", 1200},
			{"Crystal Energy Session", "45 Min Deep Healing", "A guided crystal grid session aligned to your chakra map, designed to clear energetic debris and restore spiritual flow.", 1800},
			{"Dream Decode Package", "Symbolic Analysis", "Submit your recurring dreams for a detailed symbolic breakdown revealing patterns, warnings, and guidance from your subconscious mind.", 600},
		},
	},
	{
		slug: "solomon", name: "Solomon", image: "https://assets.staryield.net/assets/figma/54c44.png", zodiac: "Aries",
		headline: "Ancient wisdom meets modern intuition for life's biggest questions.",
		skills:   []string{"Palmistry", "Aura Reading", "Past Life"}, rating: 4.8, reviews: 671, readings: 1200, years: 5, rate: 45,
		about: []string{
			"Solomon reads the map written in your hands and the colors radiating from your energy field. Trained in past life regression therapy, he helps clients understand karmic cycles and ancestral patterns that shape present-day choices.",
			"In sessions, he moves between the physical landscape of the body and the subtle currents of the aura. He listens for the stories that the hands hold-lines that speak to resilience, the shape of the fingers that reveal how a person receives and acts on intuition, and the spaces between the fingers that describe the way a person navigates freedom and responsibility. When he looks at the energy field, he is not just seeing color; he is reading the vibration of a person's current state. A cool blue may indicate a need for stillness and trust, while a sharp orange may signal a call to creative action and the breaking of old patterns.",
		},
		philosophy: "\"Your palms hold the blueprint of every life you've lived. Understanding those lines is the first step to rewriting the ones ahead.\"",
		services: []service{
			{"Schedule a Reading", "30 Min Live Session", "Book a dedicated calendar slot for a private, uninterrupted deep-dive session focusing on career, love, or spiritual transition.", 1200},
			{"evil eye ritual", "45 Min Deep Healing", "A guided crystal grid session aligned to your chakra map, designed to clear energetic debris and restore spiritual flow.", 500},
			{"Soul mate sketch", "Symbolic Analysis", "Submit your recurring dreams for a detailed symbolic breakdown revealing patterns, warnings, and guidance from your subconscious mind.", 1590},
		},
	},
}

var packs = []domain.CreditPack{
	{Name: "Starter", Credits: 250, PriceCents: 999, Currency: "USD", SortOrder: 1},
	{Name: "Seeker", Credits: 600, PriceCents: 1999, Currency: "USD", SortOrder: 2, Featured: true},
	{Name: "Devotee", Credits: 1500, PriceCents: 3999, Currency: "USD", SortOrder: 3},
}

// Run seeds reference data. It is safe to run repeatedly.
func Run(ctx context.Context, db *gorm.DB, hasher *security.PasswordHasher, opts Options, log *zap.Logger) error {
	txm := database.NewTxManager(db)
	users := repository.NewUserRepository(db)
	wallets := repository.NewWalletRepository(db)
	psychics := repository.NewPsychicRepository(db)
	credits := repository.NewCreditRepository(db)
	now := time.Now().UTC()

	existing, err := credits.ListPacks(ctx, false)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		for _, p := range packs {
			p.ID, p.Active, p.CreatedAt, p.UpdatedAt = domain.NewID(), true, now, now
			if err := credits.CreatePack(ctx, &p); err != nil {
				return fmt.Errorf("seed pack: %w", err)
			}
		}
		log.Info("seeded credit packs", zap.Int("count", len(packs)))
	}

	ensureUser := func(ctx context.Context, email, password string, role domain.Role, name string, freeMinutes int) (*domain.User, bool, error) {
		if u, err := users.GetByEmail(ctx, email); err == nil {
			return u, false, nil
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, false, err
		}
		if msg := security.ValidatePassword(password, email); msg != "" {
			return nil, false, fmt.Errorf("seed password for %s: %s", email, msg)
		}
		hash, err := hasher.Hash(password)
		if err != nil {
			return nil, false, err
		}
		u := &domain.User{ID: domain.NewID(), Email: email, PasswordHash: hash, Role: role, Status: domain.UserActive,
			DisplayName: name, Locale: "en", TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
		if err := users.Create(ctx, u); err != nil {
			return nil, false, err
		}
		if err := users.CreateProfile(ctx, &domain.Profile{UserID: u.ID, Mood: "Empowered", Theme: "Warm Beige", NotifyHoroscope: true, UpdatedAt: now}); err != nil {
			return nil, false, err
		}
		if err := wallets.Create(ctx, &domain.Wallet{UserID: u.ID, FreeMinutes: freeMinutes, UpdatedAt: now}); err != nil {
			return nil, false, err
		}
		return u, true, nil
	}

	for i, a := range advisors {
		if _, err := psychics.GetBySlug(ctx, a.slug); err == nil {
			continue
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		password := opts.PsychicPassword
		if password == "" {
			password = "Locked-" + security.MustRandomToken(24) + "1!"
		}
		err := txm.WithinTx(ctx, func(ctx context.Context) error {
			u, _, err := ensureUser(ctx, a.slug+"@advisors.staryield.net", password, domain.RolePsychic, a.name, 0)
			if err != nil {
				return err
			}
			p := &domain.Psychic{UserID: u.ID, Slug: a.slug, Name: a.name, Headline: a.headline, About: a.about, Philosophy: a.philosophy,
				ImageURL: a.image, Zodiac: a.zodiac, Skills: a.skills, Rating: a.rating, ReviewsCount: a.reviews, ReadingsCount: a.readings,
				YearsExperience: a.years, RatePerMinute: a.rate, Availability: domain.AvailabilityOffline, Listed: true, SortOrder: i,
				CreatedAt: now, UpdatedAt: now}
			if err := psychics.Create(ctx, p); err != nil {
				return err
			}
			for j, s := range a.services {
				if err := psychics.CreateService(ctx, &domain.PsychicService{ID: domain.NewID(), PsychicID: u.ID, Name: s.name, Subtitle: s.subtitle,
					Description: s.description, PriceCredits: s.price, Active: true, SortOrder: j, CreatedAt: now, UpdatedAt: now}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("seed advisor %s: %w", a.slug, err)
		}
		log.Info("seeded advisor", zap.String("slug", a.slug), zap.String("login", a.slug+"@advisors.staryield.net"))
	}

	if opts.Admin != nil && opts.Admin.Email != "" {
		_, made, err := ensureUser(ctx, opts.Admin.Email, opts.Admin.Password, domain.RoleAdmin, "Administrator", 0)
		if err != nil {
			return fmt.Errorf("seed admin: %w", err)
		}
		if made {
			log.Info("seeded admin account", zap.String("email", opts.Admin.Email))
		}
	}
	if opts.DemoClient != nil && opts.DemoClient.Email != "" {
		u, made, err := ensureUser(ctx, opts.DemoClient.Email, opts.DemoClient.Password, domain.RoleClient, "Cassandra Moon", 3)
		if err != nil {
			return fmt.Errorf("seed demo client: %w", err)
		}
		if made && opts.DemoCredits > 0 {
			err := txm.WithinTx(ctx, func(ctx context.Context) error {
				w, err := wallets.GetForUpdate(ctx, u.ID)
				if err != nil {
					return err
				}
				w.Balance += opts.DemoCredits
				w.UpdatedAt = now
				if err := wallets.AppendLedger(ctx, &domain.LedgerEntry{ID: domain.NewID(), UserID: u.ID, EntryType: domain.LedgerAdminAdjustment,
					Amount: opts.DemoCredits, BalanceAfter: w.Balance, HeldAfter: w.Held, ReferenceType: "seed",
					IdempotencyKey: "seed_demo_credits:" + u.ID.String(), Description: "Demo credits", CreatedAt: now}); err != nil {
					return err
				}
				return wallets.Save(ctx, w)
			})
			if err != nil {
				return fmt.Errorf("seed demo credits: %w", err)
			}
		}
		if made {
			log.Info("seeded demo client", zap.String("email", opts.DemoClient.Email))
		}
	}
	return nil
}
