package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// GoogleDefaultIssuer is Google's OpenID Connect issuer.
const GoogleDefaultIssuer = "https://accounts.google.com"

// GoogleConfig controls Sign in with Google (revisions 4.0, §23.1): the
// server-side OpenID Connect authorization-code flow. The feature is on only
// when the client id, secret and redirect URL are all set.
type GoogleConfig struct {
	ClientID     string `env:"client_id"`
	ClientSecret string `env:"client_secret"`
	// RedirectURL is exactly the authorised redirect URI of the OAuth client
	// (https://api.staryield.net/api/v1/auth/google/callback).
	RedirectURL string `env:"redirect_url"`
	// Issuer is Google's issuer. Any other value is for tests (a local mock
	// provider) and is refused in production.
	Issuer string `env:"issuer"`
	// StateTTL is the lifetime of a sign-in attempt and of a pending
	// consent or link.
	StateTTL time.Duration `env:"state_ttl"`
}

// Enabled reports whether Google sign-in is configured.
func (g GoogleConfig) Enabled() bool {
	return g.ClientID != "" && g.ClientSecret != "" && g.RedirectURL != ""
}

// String redacts the client secret, so the configuration can be printed or
// logged without leaking it.
func (g GoogleConfig) String() string {
	secret := ""
	if g.ClientSecret != "" {
		secret = "[redacted]"
	}
	return fmt.Sprintf("{ClientID:%s ClientSecret:%s RedirectURL:%s Issuer:%s StateTTL:%s}",
		g.ClientID, secret, g.RedirectURL, g.Issuer, g.StateTTL)
}

// GoString implements fmt.GoStringer (the %#v form) with the same redaction.
func (g GoogleConfig) GoString() string { return "config.GoogleConfig" + g.String() }

// isLoopbackHost reports localhost and loopback IP literals.
func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// validateGoogle checks the Google sign-in settings (revisions 4.0).
func (c *Config) validateGoogle(add func(string, ...any)) {
	e := EnvVar
	g := c.Google
	set := 0
	for _, v := range []string{g.ClientID, g.ClientSecret, g.RedirectURL} {
		if v != "" {
			set++
		}
	}
	if set != 0 && set != 3 {
		add("%s, %s and %s must be set together", e("google.client_id"), e("google.client_secret"), e("google.redirect_url"))
	}
	if g.StateTTL < time.Minute || g.StateTTL > time.Hour {
		add("%s must be within 1m..1h", e("google.state_ttl"))
	}
	issuer, err := url.Parse(g.Issuer)
	switch {
	case err != nil || issuer.Host == "" || issuer.User != nil || issuer.RawQuery != "" || issuer.Fragment != "":
		add("%s must be an absolute https URL without credentials, query or fragment", e("google.issuer"))
	case issuer.Scheme != "https" && !(issuer.Scheme == "http" && isLoopbackHost(issuer.Hostname())):
		add("%s must be https (http is allowed only for a loopback test issuer)", e("google.issuer"))
	case c.IsProduction() && g.Issuer != GoogleDefaultIssuer:
		add("%s must be %s in production", e("google.issuer"), GoogleDefaultIssuer)
	}
	if g.RedirectURL == "" {
		return
	}
	ru, err := url.Parse(g.RedirectURL)
	switch {
	case err != nil || ru.Host == "" || ru.User != nil || ru.Fragment != "" || ru.RawQuery != "":
		add("%s must be an absolute URL without credentials, query or fragment", e("google.redirect_url"))
	case ru.Scheme == "https":
	case ru.Scheme == "http" && !c.IsProduction() && isLoopbackHost(ru.Hostname()):
	default:
		add("%s must be https (http is allowed only for localhost outside production)", e("google.redirect_url"))
	}
}
