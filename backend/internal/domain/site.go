package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// revisions 3.0: site settings (§15).

// Site setting keys (the CHECK constraint of site_settings lists the same).
const (
	SiteSettingBanners     = "banners"
	SiteSettingSignupOffer = "signup_offer"
	SiteSettingFreeMode    = "free_mode"
)

// Site configuration bounds.
const (
	// MaxSiteBanners is the most promo banners that can exist.
	MaxSiteBanners = 10
	// BannerTextMax and BannerCTAMax bound the localized copy (characters).
	BannerTextMax = 140
	BannerCTAMax  = 40
	// BannerHrefMax bounds a banner destination.
	BannerHrefMax = 300
	// BannerRotationMin, BannerRotationMax and BannerRotationDefault bound
	// the seconds each banner is shown.
	BannerRotationMin     = 4
	BannerRotationMax     = 30
	BannerRotationDefault = 6
	// SignupFreeMinutesMin and SignupFreeMinutesMax bound the sign-up offer.
	SignupFreeMinutesMin = 1
	SignupFreeMinutesMax = 60
	// FreeModeMaxDuration is the furthest ends_at may be from now.
	FreeModeMaxDuration = 90 * 24 * time.Hour
	// FreeModeConfirmation is the phrase an admin types to switch it on.
	FreeModeConfirmation = "ENABLE FREE MODE"
)

// Banner audiences and action types.
const (
	AudienceAll       = "all"
	AudienceAnonymous = "anonymous"
	AudienceSignedIn  = "signed_in"
	ActionLink        = "link"
	ActionNone        = "none"
)

// BannerStatus is the computed state of a banner.
type BannerStatus string

// Banner statuses.
const (
	BannerLive      BannerStatus = "live"
	BannerScheduled BannerStatus = "scheduled"
	BannerExpired   BannerStatus = "expired"
	BannerInactive  BannerStatus = "inactive"
)

// SiteLocales are the languages banner copy is written in (en is required).
var SiteLocales = []string{"en", "fr", "es"}

// SiteSetting is one settings document.
type SiteSetting struct {
	Key       string     `gorm:"primaryKey"`
	Value     JSONMap    `gorm:"type:jsonb;not null"`
	Version   int        `gorm:"not null"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid"`
	UpdatedAt time.Time
}

func (SiteSetting) TableName() string { return "site_settings" }

// L10n is localized copy keyed by locale ("en", "fr", "es").
type L10n map[string]string

// Value implements driver.Valuer.
func (l L10n) Value() (driver.Value, error) {
	if l == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]string(l))
	return string(b), err
}

// Scan implements sql.Scanner.
func (l *L10n) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case nil:
		*l = L10n{}
		return nil
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("domain: L10n: unsupported source")
	}
	out := map[string]string{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*l = out
	return nil
}

// Pick returns the copy for locale, falling back to English.
func (l L10n) Pick(locale string) string {
	if v := l[locale]; v != "" {
		return v
	}
	return l["en"]
}

// SiteBanner is one promo bar item.
type SiteBanner struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Text       L10n      `gorm:"type:jsonb;not null"`
	CTALabel   L10n      `gorm:"column:cta_label;type:jsonb;not null"`
	ActionType string    `gorm:"not null"`
	ActionHref string    `gorm:"not null"`
	Audience   string    `gorm:"not null"`
	Active     bool      `gorm:"not null"`
	StartsAt   *time.Time
	EndsAt     *time.Time
	SortOrder  int        `gorm:"not null"`
	Version    int        `gorm:"not null"`
	CreatedBy  *uuid.UUID `gorm:"type:uuid"`
	UpdatedBy  *uuid.UUID `gorm:"type:uuid"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SiteBanner) TableName() string { return "site_banners" }

// StatusAt computes the banner's state at now.
func (b *SiteBanner) StatusAt(now time.Time) BannerStatus {
	switch {
	case !b.Active:
		return BannerInactive
	case b.EndsAt != nil && !now.Before(*b.EndsAt):
		return BannerExpired
	case b.StartsAt != nil && now.Before(*b.StartsAt):
		return BannerScheduled
	}
	return BannerLive
}
