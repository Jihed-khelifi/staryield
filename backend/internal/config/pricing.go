package config

// PricingConfig holds the operational switches of the pricing module
// (revisions 5.0, §24). Prices, packs and the advisor rate range are not
// configuration: the admin console owns them (§24.0).
type PricingConfig struct {
	// OfferRequireCardCheck makes the welcome offer refuse a purchase whose
	// card fingerprint is unknown (STARYIELD_PRICING_OFFER_REQUIRE_CARD_CHECK,
	// default true). With false the offer is limited per account only and the
	// redemption is stored with an empty card key.
	OfferRequireCardCheck bool `env:"offer_require_card_check"`
}
