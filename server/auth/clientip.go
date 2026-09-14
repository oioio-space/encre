package auth

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the address a rate limiter should key on from r,
// suitable as the key passed to [Limiter.Allow] for ENCRE_04 §7's
// 5-logins-per-minute-per-IP limit.
//
// It trusts the X-Forwarded-For header only when r.RemoteAddr itself is one
// of trustedProxies (ENCRE's deployment puts Caddy in front of the server on
// the same host or a private network, per ENCRE_04 §1) — otherwise
// RemoteAddr is used as-is. This matters because r.RemoteAddr behind any
// reverse proxy is the proxy's own address for every request, which is
// useless as a rate-limit key; the header is meant to carry the real one.
// But the header is also entirely attacker-controlled up to the point a
// trusted proxy overwrites or appends to it — reading it unconditionally
// lets an attacker present a fresh, forged key on every request and defeat
// the limit entirely, which is exactly the bug this function exists to
// close.
//
// X-Forwarded-For is a comma-separated list, oldest hop first, newest
// (closest to this server) last. ClientIP walks it from the right and
// returns the first entry that is not itself inside trustedProxies — a hop
// any of our own trusted proxies added, chained or not, is skipped, and the
// first hop an attacker could have written by hand is the one used. If
// every hop turns out to be trusted, or the header is absent or unparsable,
// RemoteAddr is used.
//
// The result is normalized so one client cannot spread its requests across
// many limiter keys for free: an IPv4 address is returned as-is, an IPv6
// address is masked to its /64 — the block a residential ISP typically
// hands one customer, so a rotating address within it still hits the same
// limiter key.
func ClientIP(r *http.Request, trustedProxies []*net.IPNet) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	remoteIP := net.ParseIP(remoteHost)
	if remoteIP == nil {
		return remoteHost
	}
	if !isTrustedProxy(remoteIP, trustedProxies) {
		return normalizeIP(remoteIP)
	}

	for hop := range lastToFirst(r.Header.Get("X-Forwarded-For")) {
		ip := net.ParseIP(strings.TrimSpace(hop))
		if ip == nil || isTrustedProxy(ip, trustedProxies) {
			continue
		}
		return normalizeIP(ip)
	}
	return normalizeIP(remoteIP)
}

// lastToFirst yields the comma-separated entries of s from last to first.
func lastToFirst(s string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		if s == "" {
			return
		}
		hops := strings.Split(s, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			if !yield(hops[i]) {
				return
			}
		}
	}
}

func isTrustedProxy(ip net.IP, proxies []*net.IPNet) bool {
	for _, p := range proxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// normalizeIP returns ip in the form used as a rate-limiter key: an IPv4
// address unchanged, an IPv6 address masked to its /64.
func normalizeIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String()
}
