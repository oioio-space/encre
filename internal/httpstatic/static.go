// Package httpstatic serves pre-compressed static files over HTTP.
//
// ENCRE's client is tens of megabytes of WebAssembly (brief/ENCRE_04 §2):
// compressing it on every request would cost more CPU than it saves, so the
// build pipeline (`mise run wasm:build`) writes a gzip sibling once and this
// package serves that sibling instead of compressing on the fly. It is used
// by both the phone/tablet dev server (cmd/serve) and the production server
// (cmd/server) — production also sits behind Caddy's `encode zstd gzip`,
// which negotiates a better encoding when the client offers one, but the
// pre-built .gz is what a bare `go run ./cmd/serve` on a dev machine has.
package httpstatic

import (
	"mime"
	"net/http"
	"path"
	"strings"
)

// Precompressed serves "<path>.gz" from dir instead of "<path>" when the
// client's Accept-Encoding offers gzip and that sibling file exists; every
// other request falls through to next unchanged.
//
// dir is not touched for its own request: Precompressed only ever opens a
// name with ".gz" appended, so it can share a directory with any other
// handler serving the uncompressed files (typically an [http.FileServer]
// passed as next).
func Precompressed(dir http.Dir, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		f, err := dir.Open(r.URL.Path + ".gz")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer f.Close() //nolint:errcheck // read-only file, nothing to report

		// The encoding is gzip, but the TYPE is still the type of what is
		// inside — a browser told application/gzip will download the file
		// instead of instantiating or executing it.
		if ct := mime.TypeByExtension(path.Ext(r.URL.Path)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		stat, err := f.Stat()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		http.ServeContent(w, r, "", stat.ModTime(), f)
	})
}
