package api

import (
	jsonv2 "encoding/json/v2"
	"net/http"
)

// maxBodyBytes bounds every request body this package reads. It is generous
// for the largest legitimate payload — a finish request carrying a full
// run's attempts — and small enough that a client cannot make a handler
// allocate an unbounded amount of memory before decoding even starts.
const maxBodyBytes = 1 << 20 // 1 MiB

// decodeJSON reads and decodes r's body into a fresh T, capped at
// maxBodyBytes, and reports whether it succeeded.
//
// It decodes with encoding/json/v2 rather than the v1 encoding/json
// server/store still uses for its own *_json columns (brief/ENCRE_04 §4's
// Go 1.27 note on this ticket): v2 rejects a duplicate object member and
// invalid UTF-8 by default, where v1 silently accepts both — the last
// duplicate key wins, and invalid bytes are replaced — which is exactly the
// ambiguity a request that decides how a run scores must not carry.
//
// On any failure — a body over maxBodyBytes, malformed JSON, a duplicate
// key, invalid UTF-8 — it writes a 400 response and returns false. The
// response never repeats anything from the decoding error: what went wrong
// is diagnostic detail for the person reading the server's own logs, not
// for whoever sent the bad request.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (v T, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := jsonv2.UnmarshalRead(r.Body, &v); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return v, false
	}
	return v, true
}

// writeJSON encodes v as the response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The status line, and possibly part of the body, are already on the
	// wire by the time Marshal could fail here; there is nothing left to do
	// but let the caller's own logging see it via the returned error being
	// swallowed only when there genuinely is no caller left to hand it to.
	_ = jsonv2.MarshalWrite(w, v)
}
