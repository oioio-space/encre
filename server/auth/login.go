package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/oioio-space/encre/server/store"
)

// dummyPasswordHash is compared against on an unknown email, so
// [LoginParent] pays the same argon2id cost whether the email exists or not.
// A real hash's salt makes every comparison's outcome false regardless: this
// is not a secret, it exists purely to keep the two code paths' cost equal.
var dummyPasswordHash = sync.OnceValues(func() (string, error) { return HashPassword("no such account") })

// LoginParent verifies email and password against server/store, returning
// the parent's ID on success. It returns [ErrInvalidCredentials] — with
// identical text and comparable timing — whether the email is unknown or the
// password is wrong, so neither a client nor a timing side-channel can tell
// the two apart.
func LoginParent(ctx context.Context, db *store.Store, email, password string) (string, error) {
	p, err := db.ParentByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		hash, hashErr := dummyPasswordHash()
		if hashErr != nil {
			return "", fmt.Errorf("preparing dummy hash: %w", hashErr)
		}
		if _, verifyErr := VerifyPassword(password, hash); verifyErr != nil {
			return "", fmt.Errorf("verifying against dummy hash: %w", verifyErr)
		}
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}

	ok, err := VerifyPassword(password, string(p.PassHash))
	if err != nil {
		return "", fmt.Errorf("verifying parent password: %w", err)
	}
	if !ok {
		return "", ErrInvalidCredentials
	}
	return p.ID, nil
}

// LoginChild verifies pseudo and pattern against server/store, returning the
// child's ID on success.
//
// pseudo has no uniqueness constraint and is not scoped to a parent
// (ENCRE_04 §6), so more than one child can share both a pseudo and, by
// coincidence, a pattern. LoginChild checks pattern against every candidate
// with that pseudo rather than stopping at the first match, and if more than
// one candidate matches, it fails closed with [ErrInvalidCredentials] rather
// than returning whichever row the query happened to return last: an
// authentication method that cannot tell two accounts apart must refuse
// both, never guess one. (Scoping child login to a family, so two
// unrelated children can never collide in the first place, is tracked as a
// separate, structural fix.)
//
// To keep an unknown pseudo from resolving faster than a known one with a
// wrong pattern, LoginChild verifies against a dummy hash when no candidate
// exists at all.
func LoginChild(ctx context.Context, db *store.Store, pseudo, pattern string) (string, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT id, pattern_hash FROM children WHERE pseudo = ?`, pseudo)
	if err != nil {
		return "", fmt.Errorf("querying children by pseudo: %w", err)
	}
	defer func() { _ = rows.Close() }()

	found := false
	var matches []string
	for rows.Next() {
		found = true
		var id string
		var patternHash []byte
		if err := rows.Scan(&id, &patternHash); err != nil {
			return "", fmt.Errorf("scanning child: %w", err)
		}
		ok, err := VerifyPattern(pattern, string(patternHash))
		if err != nil {
			return "", fmt.Errorf("verifying child pattern: %w", err)
		}
		if ok {
			matches = append(matches, id)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterating children: %w", err)
	}

	if !found {
		hash, hashErr := dummyPatternHash()
		if hashErr != nil {
			return "", fmt.Errorf("preparing dummy hash: %w", hashErr)
		}
		if _, verifyErr := VerifyPattern(pattern, hash); verifyErr != nil {
			return "", fmt.Errorf("verifying against dummy hash: %w", verifyErr)
		}
		return "", ErrInvalidCredentials
	}
	if len(matches) != 1 {
		return "", ErrInvalidCredentials
	}
	return matches[0], nil
}

// dummyPatternHash mirrors [dummyPasswordHash] for [LoginChild].
var dummyPatternHash = sync.OnceValues(func() (string, error) { return HashPattern("0000") })
