package gen_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// TestAnthropicClientCompleteReportsNon200WithoutErrorBody checks the
// branch [AnthropicClient.Complete] falls into when a non-200 response
// carries no Anthropic-shaped error field at all (a bare 500 from a proxy
// in front of the real API, say): the caller still gets an error naming the
// HTTP status, not a silently empty reply.
func TestAnthropicClientCompleteReportsNon200WithoutErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: srv.URL}
	_, err := client.Complete(t.Context(), "Mots : chat")
	if err == nil {
		t.Fatal("Complete() with a 500 response and no error body: want error, got nil")
	}
}

// TestAnthropicClientCompleteReportsRateLimit checks the 429 path
// specifically: a rate-limited caller must see the same wrapped error shape
// as any other API error, not a status this client treats as retryable on
// its own (it never retries — that decision belongs to the caller).
func TestAnthropicClientCompleteReportsRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"type": "rate_limit_error", "message": "too many requests"},
		})
	}))
	defer srv.Close()

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: srv.URL}
	_, err := client.Complete(t.Context(), "Mots : chat")
	if err == nil {
		t.Fatal("Complete() with a 429 response: want error, got nil")
	}
}

// TestAnthropicClientCompleteReportsNoTextBlock checks a 200 response whose
// content carries no "text" block (an empty content array, or only
// non-text blocks) fails with an explicit error rather than returning an
// empty string [GenerateSentences] would then try to parse as JSON.
func TestAnthropicClientCompleteReportsNoTextBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"content": []map[string]string{}})
	}))
	defer srv.Close()

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: srv.URL}
	_, err := client.Complete(t.Context(), "Mots : chat")
	if err == nil {
		t.Fatal("Complete() with a content array carrying no text block: want error, got nil")
	}
}

// TestAnthropicClientCompleteReportsMalformedResponseBody checks a 200
// response whose body is not valid JSON at all (a truncated reply, or a
// proxy that answers HTML) fails cleanly instead of panicking on decode.
func TestAnthropicClientCompleteReportsMalformedResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content": [`))
	}))
	defer srv.Close()

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: srv.URL}
	_, err := client.Complete(t.Context(), "Mots : chat")
	if err == nil {
		t.Fatal("Complete() with a truncated JSON body: want error, got nil")
	}
}

// TestAnthropicClientCompleteRespectsContextDeadline checks that a request
// past its context's deadline fails with an error rather than hanging —
// the network-timeout half of "l'API peut échouer" this client must answer
// for, exercised against a server that deliberately never responds in time.
func TestAnthropicClientCompleteRespectsContextDeadline(t *testing.T) {
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-unblock
	}))
	defer func() {
		close(unblock)
		srv.Close()
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: srv.URL}
	if _, err := client.Complete(ctx, "Mots : chat"); err == nil {
		t.Error("Complete() past its context deadline: want error, got nil")
	}
}

// TestAnthropicClientCompleteReportsNetworkFailure points BaseURL at a
// closed local port, so the request fails at the transport level rather
// than getting any HTTP response at all — [client.Do]'s own error branch,
// distinct from every other test in this file which reaches a real (if
// unusual) response.
func TestAnthropicClientCompleteReportsNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close() // nothing is listening here anymore

	client := &gen.AnthropicClient{APIKey: "test-key", BaseURL: closedURL}
	if _, err := client.Complete(t.Context(), "Mots : chat"); err == nil {
		t.Error("Complete() against a closed port: want error, got nil")
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
