package domain

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// revisions 5.0: pricing catalog and welcome offer (§24.1, §24.2).

// Credit pack kinds.
const (
	PackStandard = "standard"
	PackIntro    = "intro"
)

// Pack copy and numeric bounds (the credit_packs CHECK constraints agree).
const (
	PackNameMax    = 60
	PackTaglineMax = 120
	PackLabelMax   = 40
	PackCreditsMin = 1
	PackCreditsMax = 1_000_000
	PackPriceMin   = 50
	PackPriceMax   = 10_000_000
)

// PackCodePattern is the immutable, machine-readable pack identifier.
var PackCodePattern = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

// SiteSettingPricing is the pricing settings document of site_settings.
const SiteSettingPricing = "pricing"

// OfferStatus is the state of a welcome-offer redemption.
type OfferStatus string

// Redemption states: reserved -> redeemed | released. Released and redeemed
// are terminal (a refund keeps a redeemed offer redeemed).
const (
	OfferReserved OfferStatus = "reserved"
	OfferRedeemed OfferStatus = "redeemed"
	OfferReleased OfferStatus = "released"
)

// Index names of offer_redemptions (a unique violation names the rule that
// refused the reservation).
const (
	OfferUserIndex = "offer_redemptions_user_active_idx"
	OfferCardIndex = "offer_redemptions_card_active_idx"
)

// OfferRedemption is one welcome-offer attempt of an account.
type OfferRedemption struct {
	ID         uuid.UUID   `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID   `gorm:"type:uuid;not null"`
	PackID     uuid.UUID   `gorm:"type:uuid;not null"`
	PurchaseID uuid.UUID   `gorm:"type:uuid;not null"`
	Provider   string      `gorm:"not null"`
	CardKey    string      `gorm:"not null"`
	Status     OfferStatus `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RedeemedAt *time.Time
	ReleasedAt *time.Time
}

func (OfferRedemption) TableName() string { return "offer_redemptions" }
