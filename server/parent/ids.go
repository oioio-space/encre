package parent

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// idBytes is how much entropy [newID] reads from crypto/rand for each
// opaque row identifier this package assigns — 128 bits, far past any
// collision concern for the number of parents and children ENCRE ever
// creates.
const idBytes = 16

// newID returns a fresh, opaque identifier suitable for
// [github.com/oioio-space/encre/server/store.Parent.ID] or
// [github.com/oioio-space/encre/server/store.Child.ID]: idBytes of
// [crypto/rand], base64url-encoded. It is never math/rand — a predictable
// identifier is guessable, and a guessable child ID would let one family
// enumerate another's.
func newID() (string, error) {
	b := make([]byte, idBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
