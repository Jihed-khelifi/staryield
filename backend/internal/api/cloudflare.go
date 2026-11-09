package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// cloudflareRanges are Cloudflare's published edge ranges
// (https://www.cloudflare.com/ips-v4/ and /ips-v6/, checked 2026-11-09).
var cloudflareRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
}

var cloudflareNets = mustNets(cloudflareRanges)

func mustNets(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("api: invalid CIDR " + cidr)
		}
		out = append(out, n)
	}
	return out
}

func validIP(s string) string {
	s = strings.TrimSpace(s)
	if net.ParseIP(s) == nil {
		return ""
	}
	return s
}

// cloudflareHop returns the address that reached our proxy chain and whether
// the request provably came through Cloudflare, so Cloudflare's headers
// (CF-Connecting-IP, CF-IPCountry) can be believed:
//   - the TCP peer must be a trusted proxy, otherwise nothing is believed;
//   - a peer that is a Cloudflare edge connected directly;
//   - a trusted proxy that reports the connecting address in X-Real-IP
//     (Railway's edge) must report a Cloudflare edge. A request sent to the
//     *.up.railway.app domain, or to Railway's edge directly, reports the
//     sender's own address, so a forged CF-Connecting-IP is ignored;
//   - a trusted proxy without X-Real-IP (e.g. cloudflared in the private
//     network) is Cloudflare's own hop.
func cloudflareHop(r *http.Request, trusted []*net.IPNet) (hop string, viaCloudflare bool) {
	peer := echo.ExtractIPDirect()(r)
	if !inNets(peer, trusted) {
		return peer, false
	}
	if inNets(peer, cloudflareNets) {
		return peer, true
	}
	if edge := validIP(r.Header.Get("X-Real-IP")); edge != "" {
		return edge, inNets(edge, cloudflareNets)
	}
	return peer, true
}

// cloudflareIP resolves the client address behind Cloudflare (see
// cloudflareHop); when the request did not come through Cloudflare the
// address that reached the proxy chain is used instead.
func cloudflareIP(trusted []*net.IPNet) echo.IPExtractor {
	return func(r *http.Request) string {
		hop, viaCloudflare := cloudflareHop(r, trusted)
		if viaCloudflare {
			if ip := validIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
				return ip
			}
		}
		return hop
	}
}
