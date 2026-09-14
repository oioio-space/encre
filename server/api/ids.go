package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// opaqueIDBytes is how much crypto/rand entropy [newOpaqueID] reads: the
// same 16 bytes [newRunID] always drew, before it and every sibling ID
// function this package needs ([newDicteeResultID], [newItemID]) shared the
// recipe.
const opaqueIDBytes = 16

// newOpaqueID returns a fresh, unguessable identifier: opaqueIDBytes of
// crypto/rand, base64url-encoded. what names the caller in a wrapped error.
//
// It is never math/rand/v2: none of these IDs are secrets — every one of
// them appears in a URL or a response body — but each still has to be
// unguessable enough that one child or parent can never enumerate another's.
func newOpaqueID(what string) (string, error) {
	b := make([]byte, opaqueIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating %s: %w", what, err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// newRunID returns a fresh run identifier.
func newRunID() (string, error) { return newOpaqueID("run id") }

// newDicteeResultID returns a fresh dictée-result identifier
// ([store.DicteeResult.ID]).
func newDicteeResultID() (string, error) { return newOpaqueID("dictee result id") }

// newItemID returns a fresh item identifier ([store.Item.ID]), for an item
// this package creates directly — [handleQuickWord]'s "mes mots" deck —
// rather than one built by server/parent's list analysis.
func newItemID() (string, error) { return newOpaqueID("item id") }

// newListID returns a fresh word-list identifier ([store.WordList.ID]).
func newListID() (string, error) { return newOpaqueID("list id") }

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
