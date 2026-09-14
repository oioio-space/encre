package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/oioio-space/encre/server/store"
)

// dummyHash memoizes one expensive argon2id-and-pepper computation so a
// flood of failed logins against an unknown email or pseudo cannot force
// [LoginParent] or [LoginChild] to pay [PasswordParams]' or
// [PatternParams]' cost on every single request — see [onceHash] for why
// this cannot be a plain [sync.OnceValues]: the hash it caches depends on
// pep, which is a parameter here rather than a package-level constant.
type onceHash struct {
	once sync.Once
	val  string
	err  error
}

// get returns compute's result, computing it only on the first call.
func (o *onceHash) get(compute func() (string, error)) (string, error) {
	o.once.Do(func() { o.val, o.err = compute() })
	return o.val, o.err
}

// dummyPasswordHash is compared against on an unknown email, so
// [LoginParent] pays the same argon2id cost whether the email exists or not.
// A real hash's salt makes every comparison's outcome false regardless: this
// is not a secret, it exists purely to keep the two code paths' cost equal.
var dummyPasswordHash onceHash

// LoginParent verifies email and password against server/store, returning
// the parent's ID on success. It returns [ErrInvalidCredentials] — with
// identical text and comparable timing — whether the email is unknown or the
// password is wrong, so neither a client nor a timing side-channel can tell
// the two apart. pep must not be nil — see [ErrPepperRequired].
func LoginParent(ctx context.Context, db *store.Store, email, password string, pep *Pepper) (string, error) {
	if pep == nil {
		return "", ErrPepperRequired
	}
	p, err := db.ParentByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		hash, hashErr := dummyPasswordHash.get(func() (string, error) { return HashPassword("no such account", pep) })
		if hashErr != nil {
			return "", fmt.Errorf("preparing dummy hash: %w", hashErr)
		}
		if _, verifyErr := VerifyPassword(password, hash, pep); verifyErr != nil {
			return "", fmt.Errorf("verifying against dummy hash: %w", verifyErr)
		}
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}

	ok, err := VerifyPassword(password, string(p.PassHash), pep)
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
// exists at all. pep must not be nil — see [ErrPepperRequired].
func LoginChild(ctx context.Context, db *store.Store, pseudo, pattern string, pep *Pepper) (string, error) {
	if pep == nil {
		return "", ErrPepperRequired
	}
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
		ok, err := VerifyPattern(pattern, string(patternHash), pep)
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
		hash, hashErr := dummyPatternHash.get(func() (string, error) { return HashPattern("000000", pep) })
		if hashErr != nil {
			return "", fmt.Errorf("preparing dummy hash: %w", hashErr)
		}
		if _, verifyErr := VerifyPattern(pattern, hash, pep); verifyErr != nil {
			return "", fmt.Errorf("verifying against dummy hash: %w", verifyErr)
		}
		return "", ErrInvalidCredentials
	}
	if len(matches) != 1 {
		return "", ErrInvalidCredentials
	}
	return matches[0], nil
}

// dummyPatternHash mirrors [dummyPasswordHash] for [LoginChild] and
// [LoginChildInFamily].
var dummyPatternHash onceHash

// LoginChildInFamily verifies pattern against exactly one child — the one
// named by childID, and only if it belongs to the family named by
// familyCode ([server/store.Parent.FamilyCode]) — returning the child's ID
// on success. This is encre-qpx.5's structural fix for [LoginChild]'s
// global-pseudo lookup: the caller reaches this function only after the
// child has already been chosen from
// [server/store.Store.ChildrenOfParent]'s list for their own family (a
// family-scoped login page, reached by a URL or cookie carrying
// familyCode), so there is no pseudo to disambiguate and nothing to be
// ambiguous about — childID names at most one row, and this function checks
// that row alone.
//
// This also makes ENCRE_04 §7's "10 motifs/min/enfant" rate limit possible
// to enforce for the first time: a caller can key a [Limiter] on childID
// before calling this function, because childID is known up front rather
// than only after a successful match — unlike a pseudo, it is never
// attacker-chosen text that could be used to lock out an arbitrary child by
// name.
//
// It returns [ErrInvalidCredentials] — with the same generic message
// [LoginChild] uses — whether familyCode is unknown, childID does not exist,
// childID belongs to a different family, or pattern is wrong; a caller must
// never distinguish these, per ENCRE_04 §7's no-enumeration-oracle
// requirement. pep must not be nil — see [ErrPepperRequired].
func LoginChildInFamily(ctx context.Context, db *store.Store, familyCode, childID, pattern string, pep *Pepper) (string, error) {
	if pep == nil {
		return "", ErrPepperRequired
	}
	parent, err := db.ParentByFamilyCode(ctx, familyCode)
	if errors.Is(err, store.ErrNotFound) {
		return dummyChildLogin(pattern, pep)
	}
	if err != nil {
		return "", err
	}

	child, err := db.ChildByID(ctx, childID)
	if errors.Is(err, store.ErrNotFound) {
		return dummyChildLogin(pattern, pep)
	}
	if err != nil {
		return "", err
	}
	if child.ParentID != parent.ID {
		// Reported exactly like an unknown child: this family must not
		// learn that childID belongs to someone else's.
		return dummyChildLogin(pattern, pep)
	}

	ok, err := VerifyPattern(pattern, string(child.PatternHash), pep)
	if err != nil {
		return "", fmt.Errorf("verifying child pattern: %w", err)
	}
	if !ok {
		return "", ErrInvalidCredentials
	}
	return child.ID, nil
}

// dummyChildLogin pays [LoginChildInFamily]'s argon2id cost against a dummy
// hash and returns [ErrInvalidCredentials], so an unknown family code, an
// unknown or cross-family child ID, and a wrong pattern all take
// comparable time.
func dummyChildLogin(pattern string, pep *Pepper) (string, error) {
	hash, hashErr := dummyPatternHash.get(func() (string, error) { return HashPattern("000000", pep) })
	if hashErr != nil {
		return "", fmt.Errorf("preparing dummy hash: %w", hashErr)
	}
	if _, verifyErr := VerifyPattern(pattern, hash, pep); verifyErr != nil {
		return "", fmt.Errorf("verifying against dummy hash: %w", verifyErr)
	}
	return "", ErrInvalidCredentials
}
