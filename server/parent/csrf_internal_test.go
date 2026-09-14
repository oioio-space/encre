package parent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFSignerTokenIsBoundToSessionToken(t *testing.T) {
	signer, err := newCSRFSigner()
	if err != nil {
		t.Fatalf("newCSRFSigner: %v", err)
	}

	tokenA := signer.Token("session-a")
	tokenB := signer.Token("session-b")
	if tokenA == tokenB {
		t.Errorf("Token: two different session tokens produced the same CSRF token")
	}
	if !signer.Valid("session-a", tokenA) {
		t.Errorf("Valid: token computed for session-a did not validate against session-a")
	}
	if signer.Valid("session-b", tokenA) {
		t.Errorf("Valid: token computed for session-a validated against session-b")
	}
	if signer.Valid("session-a", "") {
		t.Errorf("Valid: an empty submitted token validated")
	}
}

func TestRequireSameOriginSafeMethodsAlwaysPass(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://example.test/parent/children", nil)
	if !requireSameOrigin(r) {
		t.Errorf("requireSameOrigin: a GET was rejected, want always allowed")
	}
}

func TestRequireSameOriginSecFetchSite(t *testing.T) {
	tests := []struct {
		name string
		site string
		want bool
	}{
		{"same-origin allowed", "same-origin", true},
		{"none allowed (typed URL / bookmark)", "none", true},
		{"cross-site rejected", "cross-site", false},
		{"same-site rejected", "same-site", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "https://example.test/parent/children", nil)
			r.Header.Set("Sec-Fetch-Site", tt.site)
			if got := requireSameOrigin(r); got != tt.want {
				t.Errorf("requireSameOrigin with Sec-Fetch-Site=%s: got %v, want %v", tt.site, got, tt.want)
			}
		})
	}
}

func TestRequireSameOriginFallsBackToOrigin(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://example.test/parent/children", nil)
	r.Header.Set("Origin", "http://example.test")
	if !requireSameOrigin(r) {
		t.Errorf("requireSameOrigin: matching Origin with no Sec-Fetch-Site was rejected")
	}

	r2 := httptest.NewRequest(http.MethodPost, "http://example.test/parent/children", nil)
	r2.Header.Set("Origin", "https://evil.test")
	if requireSameOrigin(r2) {
		t.Errorf("requireSameOrigin: mismatched Origin was allowed")
	}
}

func TestRequireSameOriginRejectsWhenBothHeadersMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "https://example.test/parent/children", nil)
	if requireSameOrigin(r) {
		t.Errorf("requireSameOrigin: a mutating request with neither header was allowed, want rejected by default")
	}
}
