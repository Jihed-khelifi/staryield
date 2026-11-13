package config

// RefundsConfig holds the operational switches of refund requests (revisions
// 5.0, §24.4). Refund decisions and courtesy credits are made in the admin
// console; these two values are bound to the published Refund Policy.
type RefundsConfig struct {
	// WindowDays is how many calendar days after a purchase a refund of unused
	// credits may be requested (STARYIELD_REFUNDS_WINDOW_DAYS, default 14). It
	// must equal the number stated in the Refund Policy; a test keeps the
	// default equal to the English policy text.
	WindowDays int `env:"window_days"`
	// RequestsEnabled turns in-app refund requests on and off
	// (STARYIELD_REFUNDS_REQUESTS_ENABLED, default true). Off: creating a
	// request answers 503 feature_unavailable; requests received by e-mail and
	// recorded by an administrator, and everything else, are unaffected.
	RequestsEnabled bool `env:"requests_enabled"`
}

// Refund window bounds (days).
const (
	RefundWindowMinDays     = 1
	RefundWindowMaxDays     = 365
	RefundWindowDefaultDays = 14
)

// validateRefunds checks the refund settings (revisions 5.0).
func (c *Config) validateRefunds(add func(string, ...any)) {
	if c.Refunds.WindowDays < RefundWindowMinDays || c.Refunds.WindowDays > RefundWindowMaxDays {
		add("%s must be within %d..%d days", EnvVar("refunds.window_days"), RefundWindowMinDays, RefundWindowMaxDays)
	}
}
