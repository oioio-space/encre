package gen_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oioio-space/encre/server/gen"
)

// TestAnthropicClientCompleteReturnsTheTextBlock drives [gen.AnthropicClient]
// against a local test server rather than the real API: it checks the
// request this package sends and the reply it extracts, with no network
// call and no API key.
func TestAnthropicClientCompleteReturnsTheTextBlock(t *testing.T) {
	var gotAuth, gotVersion, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"type": "text", "text": `{"mot":"chat","phrases":[]}`}},
		})
	}))
	defer srv.Close()

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: srv.URL}
	got, err := client.Complete(t.Context(), "Mots : chat")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != `{"mot":"chat","phrases":[]}` {
		t.Errorf("Complete() = %q, want the text block's content", got)
	}
	if gotAuth != "test-key" {
		t.Errorf("x-api-key header = %q, want %q", gotAuth, "test-key")
	}
	if gotVersion == "" {
		t.Error("anthropic-version header is empty")
	}
	if gotBody == "" {
		t.Error("request body is empty")
	}
}

// TestAnthropicClientCompleteReportsAPIError checks that a non-200 response
// with an Anthropic-shaped error body comes back as a Go error naming it,
// not a silent empty string.
func TestAnthropicClientCompleteReportsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"type": "authentication_error", "message": "invalid x-api-key"},
		})
	}))
	defer srv.Close()

	client := &gen.AnthropicClient{APIKey: "bad-key", BaseURL: srv.URL}
	_, err := client.Complete(t.Context(), "Mots : chat")
	if err == nil {
		t.Fatal("Complete() with a 401 response: want error, got nil")
	}
}

// TestNewAnthropicClientRequiresAPIKeyEnv checks that no key in the
// environment fails loudly rather than building a client that would fail
// silently on its first call.
func TestNewAnthropicClientRequiresAPIKeyEnv(t *testing.T) {
	t.Setenv(gen.AnthropicAPIKeyEnv, "")
	if _, err := gen.NewAnthropicClient(); err == nil {
		t.Error("NewAnthropicClient() with no API key in the environment: want error, got nil")
	}
}
