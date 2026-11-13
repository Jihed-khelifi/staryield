package service

import (
	"sort"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: pricing catalog (§24.1). Badges are computed from the live
// prices and purchase counts, never stored, so a claim always matches what
// the page shows.

// Badge types.
const (
	BadgeNewClientOffer = "new_client_offer"
	BadgeMostPopular    = "most_popular"
	BadgeBestValue      = "best_value"
	BadgeSave           = "save"
)

// PackBadge is a computed claim about a pack.
type PackBadge struct {
	Type string `json:"type"`
	// Percent is the savings of a `save` badge: a whole number >= 1, floored
	// so it is never overstated.
	Percent int `json:"percent,omitempty"`
}

// badgePack is the part of a pack the badge rules read.
type badgePack struct {
	ID         uuid.UUID
	Kind       string
	Reference  bool
	Credits    int64
	PriceCents int64
}

func badgePackOf(p *domain.CreditPack) badgePack {
	return badgePack{ID: p.ID, Kind: p.Kind, Reference: p.Reference, Credits: p.Credits, PriceCents: p.PriceCents}
}

// savePercent is floor(100 × (ref_price × credits − price × ref_credits) ÷
// (ref_price × credits)) in integer math, i.e. the whole percent a pack is
// cheaper per credit than the reference. It is 0 when the pack is not
// cheaper (never negative) or the reference is unusable.
func savePercent(price, credits, refPrice, refCredits int64) int {
	den := refPrice * credits
	if den <= 0 || refCredits <= 0 || price <= 0 {
		return 0
	}
	num := 100 * (den - price*refCredits)
	if num <= 0 {
		return 0
	}
	return int(num / den)
}

// cheaperPerCredit reports whether a costs strictly less per credit than b.
func cheaperPerCredit(a, b badgePack) bool {
	return a.PriceCents*b.Credits < b.PriceCents*a.Credits
}

// computeBadges returns the badges of every active pack (keyed by pack ID),
// in the order new_client_offer, most_popular, best_value, save.
//
//   - new_client_offer: intro packs.
//   - most_popular: the standard pack with the most succeeded purchases in
//     the window, only when the standard packs together have at least
//     minPurchases and the leader leads strictly. Until then nobody has it:
//     the claim must be substantiated.
//   - best_value: the standard pack with the strictly lowest price per
//     credit (a comparison needs at least two standard packs).
//   - save: standard packs cheaper per credit than the reference pack.
func computeBadges(packs []badgePack, counts map[uuid.UUID]int64, minPurchases int) map[uuid.UUID][]PackBadge {
	out := make(map[uuid.UUID][]PackBadge, len(packs))
	var standard []badgePack
	var ref *badgePack
	for i := range packs {
		p := packs[i]
		if p.Kind == domain.PackIntro {
			out[p.ID] = append(out[p.ID], PackBadge{Type: BadgeNewClientOffer})
			continue
		}
		standard = append(standard, p)
		if p.Reference {
			ref = &packs[i]
		}
	}

	// most_popular
	var total int64
	for _, p := range standard {
		total += counts[p.ID]
	}
	if len(standard) > 0 && total >= int64(minPurchases) && total > 0 {
		sorted := append([]badgePack(nil), standard...)
		sort.SliceStable(sorted, func(i, j int) bool { return counts[sorted[i].ID] > counts[sorted[j].ID] })
		if len(sorted) == 1 || counts[sorted[0].ID] > counts[sorted[1].ID] {
			out[sorted[0].ID] = append(out[sorted[0].ID], PackBadge{Type: BadgeMostPopular})
		}
	}

	// best_value
	if len(standard) >= 2 {
		best := 0
		unique := true
		for i := 1; i < len(standard); i++ {
			switch {
			case cheaperPerCredit(standard[i], standard[best]):
				best, unique = i, true
			case !cheaperPerCredit(standard[best], standard[i]):
				unique = false // tied with the current best
			}
		}
		if unique {
			out[standard[best].ID] = append(out[standard[best].ID], PackBadge{Type: BadgeBestValue})
		}
	}

	// save
	if ref != nil {
		for _, p := range standard {
			if p.ID == ref.ID {
				continue
			}
			if pct := savePercent(p.PriceCents, p.Credits, ref.PriceCents, ref.Credits); pct >= 1 {
				out[p.ID] = append(out[p.ID], PackBadge{Type: BadgeSave, Percent: pct})
			}
		}
	}
	return out
}
