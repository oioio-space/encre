package auth_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oioio-space/encre/server/auth"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q) error = %v", s, err)
	}
	return n
}

// TestClientIPIgnoresForgedHeaderFromUntrustedPeer is the regression test
// for the bug an independent audit found: behind a reverse proxy,
// r.RemoteAddr is the proxy's own address for every request, so a naive
// handler reads X-Forwarded-For instead — but that header is attacker
// input up to the point a *trusted* proxy overwrites it. Without checking
// who the immediate peer is, an attacker can send a fresh forged IP on
// every request and empty rate-limit key by key.
func TestClientIPIgnoresForgedHeaderFromUntrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:54321" // not in trustedProxies below
	req.Header.Set("X-Forwarded-For", "6.6.6.6")

	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.1/32")}
	got := auth.ClientIP(req, trusted)
	if got != "203.0.113.9" {
		t.Errorf("ClientIP() = %q, want %q (RemoteAddr, header from an untrusted peer must be ignored)", got, "203.0.113.9")
	}
}

func TestClientIPTrustsHeaderFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.1/32")}
	got := auth.ClientIP(req, trusted)
	if got != "198.51.100.7" {
		t.Errorf("ClientIP() = %q, want %q (header from a trusted proxy)", got, "198.51.100.7")
	}
}

// TestClientIPSkipsTrustedHopsInChain checks the "last untrusted hop" rule:
// with two trusted proxies chained, the header carries [client, proxy1] by
// the time it reaches proxy2 (this server), and the real client is the
// rightmost entry that is NOT itself a trusted proxy.
func TestClientIPSkipsTrustedHopsInChain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.2:1"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 127.0.0.1")

	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.0/8")}
	got := auth.ClientIP(req, trusted)
	if got != "198.51.100.7" {
		t.Errorf("ClientIP() = %q, want %q (skip the trusted hop, use the client's)", got, "198.51.100.7")
	}
}

func TestClientIPFallsBackWhenEveryHopIsTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.2:1"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")

	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.0/8")}
	got := auth.ClientIP(req, trusted)
	if got != "127.0.0.2" {
		t.Errorf("ClientIP() = %q, want %q (RemoteAddr, no untrusted hop found)", got, "127.0.0.2")
	}
}

func TestClientIPNormalizesIPv6ToSlash64(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[2001:db8:1234:5678:aaaa:bbbb:cccc:dddd]:1"

	got := auth.ClientIP(req, nil)
	if got != "2001:db8:1234:5678::" {
		t.Errorf("ClientIP() = %q, want %q (masked to /64)", got, "2001:db8:1234:5678::")
	}
}
