package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// newRunID returns a fresh run identifier: 16 bytes of crypto/rand,
// base64url-encoded. The same recipe as [auth.NewSessionToken], sized down
// because a run ID is not a secret — it appears in the run's own URL — only
// unguessable enough that one child cannot enumerate another's.
func newRunID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating run id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// newDeckSeed draws the seed a fresh run's deck is built from.
//
// It comes from crypto/rand, not math/rand/v2, because nothing about this
// value should ever be predictable from the outside — a child who could
// guess the next seed could see next week's boss before Monday. Nothing
// about determinism suffers: once drawn, the seed is recorded in Deck.Seed,
// and everything downstream ([engine.BuildDeck]'s own draw, [engine.Apply]'s
// shine draw) rebuilds from that recorded value, never from crypto/rand
// again.
func newDeckSeed() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("generating deck seed: %w", err)
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}
