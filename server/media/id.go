package media

import "strings"

// maxIDLength bounds an ID [ValidID] accepts: every opaque ID this codebase
// assigns (server/api's newOpaqueID, server/parent's newID) is 16 bytes of
// crypto/rand, base64url-encoded — 22 characters. This leaves generous
// headroom without accepting an arbitrarily long string.
const maxIDLength = 64

// idAlphabet is exactly base64.RawURLEncoding's alphabet: every ID this
// codebase assigns is built from it (server/api.newOpaqueID,
// server/parent.newID), and it excludes every character that matters to a
// path — no "/", no "..", no NUL, no space.
const idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// ValidID reports whether id is safe to use as a path segment: non-empty, at
// most [maxIDLength] runes, and built only from [idAlphabet].
//
// This is the one gate every path this package ever builds ([Root.Path])
// goes through — a listID or itemID that fails this check is never joined
// onto a filesystem path, whatever it contains.
func ValidID(id string) bool {
	if id == "" || len(id) > maxIDLength {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool {
		return !strings.ContainsRune(idAlphabet, r)
	}) == -1
}
