package httpstatic_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/oioio-space/encre/internal/httpstatic"
)

// notReached fails the test if it is ever invoked — it stands in for "the
// fallback handler must not run" in the cases that expect the compressed
// sibling to be served instead.
func notReached(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("Precompressed() called the fallback handler, want the .gz sibling served")
	})
}

func TestPrecompressedServesGzipSiblingWhenAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "encre.wasm.gz"), []byte("gz-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	h := httpstatic.Precompressed(http.Dir(dir), notReached(t))
	req := httptest.NewRequest(http.MethodGet, "/encre.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got, want := rec.Body.String(), "gz-bytes"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Content-Encoding"), "gzip"; got != want {
		t.Errorf("Content-Encoding = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/wasm"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Vary"), "Accept-Encoding"; got != want {
		t.Errorf("Vary = %q, want %q", got, want)
	}
}

func TestPrecompressedFallsBackWithoutAcceptEncoding(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "encre.wasm.gz"), []byte("gz-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	var called bool
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := httpstatic.Precompressed(http.Dir(dir), fallback)
	req := httptest.NewRequest(http.MethodGet, "/encre.wasm", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Error("Precompressed() did not fall back when Accept-Encoding lacked gzip")
	}
}

func TestPrecompressedFallsBackWhenNoGzipSibling(t *testing.T) {
	dir := t.TempDir()

	var called bool
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := httpstatic.Precompressed(http.Dir(dir), fallback)
	req := httptest.NewRequest(http.MethodGet, "/missing.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Error("Precompressed() did not fall back when no .gz sibling exists")
	}
}
