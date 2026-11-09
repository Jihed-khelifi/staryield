package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

func TestCountryResolverTrustsOnlyCloudflareProxies(t *testing.T) {
	request := func(peer, country string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = peer + ":443"
		if country != "" {
			r.Header.Set("CF-IPCountry", country)
		}
		return r
	}
	// Outside Cloudflare the header is client-controlled and ignored.
	for _, src := range []string{"direct", "xff", "x-real-ip"} {
		resolve := CountryResolver(config.HTTPConfig{IPSource: src, TrustedProxies: []string{"10.0.0.0/8"}})
		if got := resolve(request("10.1.2.3", "FR")); got != "" {
			t.Errorf("%s: header honoured: %q", src, got)
		}
	}
	resolve := CountryResolver(config.HTTPConfig{IPSource: "cloudflare", TrustedProxies: []string{"10.0.0.0/8", "not-a-cidr"}})
	for _, c := range []struct{ peer, header, want string }{
		{"10.1.2.3", "fr", "FR"},
		{"10.1.2.3", " DE ", "DE"},
		{"203.0.113.9", "FR", ""}, // not a trusted proxy: spoofed header
		{"10.1.2.3", "XX", ""},    // unknown
		{"10.1.2.3", "T1", ""},    // Tor
		{"10.1.2.3", "F1", ""},
		{"10.1.2.3", "FRA", ""},
		{"10.1.2.3", "", ""},
	} {
		if got := resolve(request(c.peer, c.header)); got != c.want {
			t.Errorf("peer %s header %q: got %q, want %q", c.peer, c.header, got, c.want)
		}
	}
}

// On Railway behind Cloudflare the edge reports the connecting address in
// X-Real-IP: only a Cloudflare edge there makes CF-IPCountry believable.
func TestCountryResolverOnRailwayNeedsACloudflareEdge(t *testing.T) {
	resolve := CountryResolver(config.HTTPConfig{IPSource: "cloudflare", TrustedProxies: []string{"10.0.0.0/8"}})
	request := func(edge string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.1.2.3:443" // Railway's edge
		r.Header.Set("X-Real-IP", edge)
		r.Header.Set("CF-IPCountry", "FR")
		return r
	}
	if got := resolve(request("172.70.4.5")); got != "FR" {
		t.Fatalf("through Cloudflare: %q", got)
	}
	if got := resolve(request("2606:4700:10::ac43:1")); got != "FR" {
		t.Fatalf("through Cloudflare over IPv6: %q", got)
	}
	// Sent to the *.up.railway.app domain: a forged header is ignored.
	if got := resolve(request("203.0.113.9")); got != "" {
		t.Fatalf("bypassing Cloudflare must not choose the country: %q", got)
	}
}
