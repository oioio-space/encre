package auth

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/alexedwards/argon2id"
	"golang.org/x/sync/semaphore"
)

// PasswordParams are ENCRE_04 §7's argon2id cost for a parent's password:
// t=3 passes over m=64 MiB, hashed on p=4 lanes.
//
// OWASP's Password Storage Cheat Sheet lists five argon2id configurations as
// iso-cost alternatives, not a weak-to-strong ladder — "these configuration
// settings provide an equal level of defense, and the only difference is a
// trade off between CPU and RAM usage." ENCRE_04's 64 MiB/t=3/p=4 sits above
// all five in raw cost; it is a deliberately heavier choice for a
// single-parent login path with no throughput requirement to protect, not a
// departure from a "minimum" the source does not define.
var PasswordParams = &argon2id.Params{
	Memory:      64 * 1024, // KiB
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// PatternParams is the argon2id cost for a child's tap pattern
// (ENCRE_04 §7's replacement for a password on accounts too young to type
// one). It uses OWASP's first-listed argon2id profile (m=19 MiB, t=2, p=1) —
// the configuration OWASP leads its five iso-cost alternatives with, not a
// "floor" beneath PasswordParams (see that var's comment: none of the five
// is stronger or weaker than another). This is lighter than PasswordParams
// because a tap pattern's search space is far smaller than a password's, so
// PasswordParams' extra cost would only add latency without adding
// meaningful resistance — still salted and slow enough to make offline
// guessing expensive, but fast enough that a classroom of children logging
// in at once does not queue.
var PatternParams = &argon2id.Params{
	Memory:      19 * 1024, // KiB
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// argon2Sem caps how many argon2id calls (hash or verify, password or
// pattern) run at once across the whole process. PasswordParams alone
// allocates 64 MiB per call: with no cap, sixteen concurrent logins reserve
// a gibibyte on a VPS that also runs SQLite and Litestream, at the cost of
// twenty requests to an attacker. The cap is [runtime.NumCPU], matching the
// hardware that actually has to run these hashes.
var argon2Sem = semaphore.NewWeighted(int64(max(1, runtime.NumCPU())))

// ErrTooManyPasswordChecks is returned by [HashPassword], [VerifyPassword],
// [HashPattern] and [VerifyPattern] when argon2Sem's concurrency cap is
// already saturated. Callers should surface it as 503 Service Unavailable,
// never retry it in a loop: argon2Sem has no queue, by design — an unbounded
// queue in front of a memory-hard hash is the same denial-of-service the cap
// exists to prevent, just delayed.
var ErrTooManyPasswordChecks = errors.New("auth: too many concurrent password checks")

// acquireArgon2Slot reserves one of argon2Sem's slots and returns the
// function that releases it, or [ErrTooManyPasswordChecks] if none is free
// right now.
func acquireArgon2Slot() (release func(), err error) {
	if !argon2Sem.TryAcquire(1) {
		return nil, ErrTooManyPasswordChecks
	}
	return func() { argon2Sem.Release(1) }, nil
}

// HashPassword returns the peppered argon2id hash of password at
// [PasswordParams] (see [Pepper.PepperHash]), salted with a fresh,
// cryptographically random salt: two calls with the same password never
// return the same string. pep must not be nil — see [ErrPepperRequired] —
// so a stolen SQLite file (ENCRE_04 §12's Litestream replica, in
// particular) never carries a hash that offline guessing can test without
// also holding pep's key.
func HashPassword(password string, pep *Pepper) (string, error) {
	if pep == nil {
		return "", ErrPepperRequired
	}
	release, err := acquireArgon2Slot()
	if err != nil {
		return "", err
	}
	defer release()

	hash, err := argon2id.CreateHash(password, PasswordParams)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	peppered, err := pep.PepperHash(hash)
	if err != nil {
		return "", fmt.Errorf("peppering password hash: %w", err)
	}
	return peppered, nil
}

// VerifyPassword reports whether password matches hash — a value
// [HashPassword] returned, under the same pep — comparing in constant time.
// It returns an error only for a malformed hash, an unknown pepper key
// version, a nil pep, or a saturated [argon2Sem]
// ([ErrTooManyPasswordChecks]), never for a mismatch.
func VerifyPassword(password, hash string, pep *Pepper) (bool, error) {
	if pep == nil {
		return false, ErrPepperRequired
	}
	release, err := acquireArgon2Slot()
	if err != nil {
		return false, err
	}
	defer release()

	ok, err := pep.VerifyHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("verifying password: %w", err)
	}
	return ok, nil
}

// HashPattern returns the peppered argon2id hash of pattern at
// [PatternParams] (see [Pepper.PepperHash]), salted with a fresh,
// cryptographically random salt. pep must not be nil — see
// [ErrPepperRequired].
func HashPattern(pattern string, pep *Pepper) (string, error) {
	if pep == nil {
		return "", ErrPepperRequired
	}
	release, err := acquireArgon2Slot()
	if err != nil {
		return "", err
	}
	defer release()

	hash, err := argon2id.CreateHash(pattern, PatternParams)
	if err != nil {
		return "", fmt.Errorf("hashing pattern: %w", err)
	}
	peppered, err := pep.PepperHash(hash)
	if err != nil {
		return "", fmt.Errorf("peppering pattern hash: %w", err)
	}
	return peppered, nil
}

// VerifyPattern reports whether pattern matches hash — a value
// [HashPattern] returned, under the same pep — comparing in constant time.
// It returns an error only for a malformed hash, an unknown pepper key
// version, a nil pep, or a saturated [argon2Sem]
// ([ErrTooManyPasswordChecks]), never for a mismatch.
func VerifyPattern(pattern, hash string, pep *Pepper) (bool, error) {
	if pep == nil {
		return false, ErrPepperRequired
	}
	release, err := acquireArgon2Slot()
	if err != nil {
		return false, err
	}
	defer release()

	ok, err := pep.VerifyHash(pattern, hash)
	if err != nil {
		return false, fmt.Errorf("verifying pattern: %w", err)
	}
	return ok, nil
}
