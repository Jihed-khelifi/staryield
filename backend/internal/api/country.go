package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// CountryResolver returns how to read the trusted edge country of a
// request (payment routing): Cloudflare's CF-IPCountry header, honoured
// only when the deployment runs behind Cloudflare and the TCP peer is one
// of the trusted proxies, so clients cannot choose their own country.
func CountryResolver(cfg config.HTTPConfig) func(*http.Request) string {
	if cfg.IPSource != "cloudflare" {
		return func(*http.Request) string { return "" }
	}
	var nets []*net.IPNet
	for _, cidr := range cfg.TrustedProxies {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, n)
		}
	}
	direct := echo.ExtractIPDirect()
	return func(r *http.Request) string {
		if !inNets(direct(r), nets) {
			return ""
		}
		c := strings.ToUpper(strings.TrimSpace(r.Header.Get("CF-IPCountry")))
		if len(c) != 2 || c == "XX" || c == "T1" || c[0] < 'A' || c[0] > 'Z' || c[1] < 'A' || c[1] > 'Z' {
			return ""
		}
		return c
	}
}
