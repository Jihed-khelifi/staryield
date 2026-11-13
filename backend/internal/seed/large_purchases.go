package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: the large profile's credit purchases and welcome-offer
// redemptions (§24.1, §24.2). They give the popularity aggregation of the
// pricing snapshot and the top-up catalog's eligibility check real volume:
// run it with  -extend "credit purchases"  on a dataset that predates it.
func init() {
	RegisterLarge(Generator{Name: "credit purchases", Run: func(_ context.Context, d *Dataset) error { return d.creditPurchases() }})
}

// creditPurchases gives ~30% of clients 1–12 purchases of the canonical packs
// over the history window (~8% declined, ~2% refunded). Of the paying
// clients ~15% opened with the welcome pack and hold a redeemed offer.
func (d *Dataset) creditPurchases() error {
	var packs []domain.CreditPack
	if err := d.DB.Where("code IN ?", []string{"welcome_600", "quick_150", "starter_315", "popular_675", "deep_1350", "premium_2400"}).
		Find(&packs).Error; err != nil || len(packs) != 6 {
		return fmt.Errorf("credit purchases: the canonical packs are missing (migration 0026): %v", err)
	}
	var welcome domain.CreditPack
	var standard []domain.CreditPack
	for _, p := range packs {
		if p.Kind == domain.PackIntro {
			welcome = p
		} else {
			standard = append(standard, p)
		}
	}
	purchases := make([]domain.CreditPurchase, 0, d.Opts.BatchSize)
	offers := make([]domain.OfferRedemption, 0, d.Opts.BatchSize/4)
	flush := func() error {
		// The offers reference their purchases.
		if err := d.insert("credit_purchases", &purchases, len(purchases), true); err != nil {
			return err
		}
		err := d.insert("offer_redemptions", &offers, len(offers), true)
		purchases, offers = purchases[:0], offers[:0]
		return err
	}
	for ci, client := range d.Clients {
		if d.Rand.Float64() >= 0.3 {
			continue
		}
		n := 1 + d.Rand.IntN(12)
		start := d.Now.Add(-time.Duration(d.Rand.Int64N(int64(historyWindow))))
		for k := 0; k < n; k++ {
			at := start.Add(time.Duration(k) * 9 * 24 * time.Hour)
			if at.After(d.Now) {
				at = d.Now.Add(-time.Duration(d.Rand.Int64N(int64(24 * time.Hour))))
			}
			p := standard[d.Rand.IntN(len(standard))]
			intro := k == 0 && d.Rand.Float64() < 0.15
			if intro {
				p = welcome
			}
			pu := domain.CreditPurchase{ID: d.ID(at), UserID: client, PackID: p.ID, Credits: p.Credits, PriceCents: p.PriceCents,
				Currency: "USD", Status: domain.PurchaseSucceeded, Provider: "sandbox_a",
				IdempotencyKey: fmt.Sprintf("load:%s:buy:%d:%d", d.Tag, ci, k), CreatedAt: at, UpdatedAt: at}
			done := at.Add(2 * time.Second)
			pu.CompletedAt = &done
			switch r := d.Rand.Float64(); {
			case r < 0.08:
				pu.Status, pu.FailureReason, pu.CompletedAt = domain.PurchaseFailed, "card_declined", nil
			case r < 0.1:
				pu.Status = domain.PurchaseRefunded
			}
			purchases = append(purchases, pu)
			if intro && pu.Status != domain.PurchaseFailed {
				sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", d.Tag, ci)))
				offers = append(offers, domain.OfferRedemption{ID: d.ID(at), UserID: client, PackID: welcome.ID, PurchaseID: pu.ID,
					Provider: "sandbox_a", CardKey: hex.EncodeToString(sum[:]), Status: domain.OfferRedeemed, CreatedAt: at, UpdatedAt: at,
					RedeemedAt: &done})
			}
		}
		if len(purchases) >= d.Opts.BatchSize-12 {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("credit purchases generated", zap.Int("purchases", d.Counts["credit_purchases"]), zap.Int("offers", d.Counts["offer_redemptions"]))
	return nil
}
