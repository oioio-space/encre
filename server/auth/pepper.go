package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alexedwards/argon2id"
	"golang.org/x/crypto/argon2"
)

// PepperKeyEnv names the environment variable [LoadPepper] reads the current
// pepper key from: "<version>:<base64-encoded 32 bytes>", for example
// "v1:3q2+7w...==". PepperKeyRetiredEnv optionally names a comma-separated
// list of the same "<version>:<base64>" pairs for keys that are no longer
// used to hash or encrypt anything new, but are still needed to verify or
// decrypt rows written under them until they are rotated forward — see
// [LoadPepper]'s doc comment for the rotation sequence this supports.
//
// Neither variable is read from anywhere inside the SQLite database
// Litestream replicates (encre-qpx.4): a leaked backup of the .db file alone
// never carries either key.
const (
	PepperKeyEnv        = "ENCRE_PEPPER_KEY"
	PepperKeyRetiredEnv = "ENCRE_PEPPER_KEY_RETIRED"
)

// pepperKeyBytes is the required length of every pepper key: 256 bits, long
// enough for both HMAC-SHA256 (any length works, but this is generous) and
// AES-256-GCM (which requires exactly this many).
const pepperKeyBytes = 32

// ErrPepperRequired is returned by [HashPassword], [VerifyPassword],
// [HashPattern], [VerifyPattern], [LoginParent], [LoginChild],
// [VerifyParentTOTP] and [VerifyParentTOTPForSession] when called with a nil
// [Pepper]. It exists to fail closed: a nil pepper would otherwise be
// indistinguishable from "peppering intentionally skipped," and the whole
// point of encre-qpx.4 is that nothing in this package ever stores or
// compares an un-peppered secret.
var ErrPepperRequired = errors.New("auth: pepper key required")

// ErrUnknownPepperVersion is returned by [Pepper.VerifyHash] and
// [Pepper.Decrypt] when the stored value names a key version [Pepper] was
// not built with — the version was retired for real (its entry dropped from
// [PepperKeyRetiredEnv]) rather than merely rotated out of current use.
var ErrUnknownPepperVersion = errors.New("auth: unknown pepper key version")

// errMalformedPeppered is returned for a stored hash or ciphertext that does
// not parse as this package's own format — never reachable from ordinary
// operation, since every value in that shape was written by this package,
// but a defensive check against a corrupted or hand-edited row.
var errMalformedPeppered = errors.New("auth: malformed peppered value")

// pepperKey is one version's worth of key material.
type pepperKey struct {
	version string
	key     []byte // always pepperKeyBytes long
}

// Pepper holds the out-of-database key material encre-qpx.4 requires:
// [HashPassword] and [HashPattern] apply it to an argon2id digest before the
// result is ever written to [server/store.Store], and the parent's TOTP
// secret is encrypted with it before [server/store.Store.CreateParent] or
// the totp_pending row [BeginTOTPEnrollment] writes ever sees it. A stolen
// SQLite file — the whole database, including every salt and every
// ciphertext — is therefore not enough to test a single guess against a
// child's pattern or a parent's password offline: the guess must also be
// peppered with a key that was never in the file and, per ENCRE_04 §12, is
// never part of what Litestream replicates.
//
// Build one with [NewPepper] or [LoadPepper]. Its zero value is not usable.
type Pepper struct {
	current pepperKey
	all     map[string]pepperKey
}

// NewPepper builds a Pepper whose current (write) key is current and whose
// verify/decrypt path also accepts any key in retired — for the rotation
// window between deploying a new [PepperKeyEnv] value and every row written
// under the old one having been rehashed or re-encrypted. Every key,
// current or retired, must be exactly 32 bytes and have a version distinct
// from every other key's.
func NewPepper(currentVersion string, currentKey []byte, retired ...struct {
	Version string
	Key     []byte
},
) (*Pepper, error) {
	cur, err := newPepperKey(currentVersion, currentKey)
	if err != nil {
		return nil, err
	}
	p := &Pepper{current: cur, all: map[string]pepperKey{cur.version: cur}}
	for _, r := range retired {
		k, err := newPepperKey(r.Version, r.Key)
		if err != nil {
			return nil, err
		}
		if _, dup := p.all[k.version]; dup {
			return nil, fmt.Errorf("auth: duplicate pepper key version %q", k.version)
		}
		p.all[k.version] = k
	}
	return p, nil
}

func newPepperKey(version string, key []byte) (pepperKey, error) {
	if version == "" {
		return pepperKey{}, errors.New("auth: pepper key version must not be empty")
	}
	if strings.Contains(version, ":") {
		return pepperKey{}, errors.New("auth: pepper key version must not contain ':'")
	}
	if len(version) > 255 {
		return pepperKey{}, errors.New("auth: pepper key version too long")
	}
	if len(key) != pepperKeyBytes {
		return pepperKey{}, fmt.Errorf("auth: pepper key must be %d bytes, got %d", pepperKeyBytes, len(key))
	}
	return pepperKey{version: version, key: key}, nil
}

// LoadPepper builds a Pepper from [PepperKeyEnv] (required) and
// [PepperKeyRetiredEnv] (optional). A deployment rotates its pepper key by:
// moving the current ENCRE_PEPPER_KEY value into PEPPER_KEY_RETIRED,
// setting a freshly generated ENCRE_PEPPER_KEY with a new version, and —
// only once every row is confirmed rehashed or re-encrypted under the new
// key — dropping the old entry from PEPPER_KEY_RETIRED.
func LoadPepper() (*Pepper, error) {
	current, ok := os.LookupEnv(PepperKeyEnv)
	if !ok || current == "" {
		return nil, fmt.Errorf("auth: %s is not set", PepperKeyEnv)
	}
	curVersion, curKey, err := parsePepperKeyEnv(current)
	if err != nil {
		return nil, fmt.Errorf("auth: parsing %s: %w", PepperKeyEnv, err)
	}

	var retired []struct {
		Version string
		Key     []byte
	}
	if raw := os.Getenv(PepperKeyRetiredEnv); raw != "" {
		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			version, key, err := parsePepperKeyEnv(entry)
			if err != nil {
				return nil, fmt.Errorf("auth: parsing %s: %w", PepperKeyRetiredEnv, err)
			}
			retired = append(retired, struct {
				Version string
				Key     []byte
			}{version, key})
		}
	}
	return NewPepper(curVersion, curKey, retired...)
}

// parsePepperKeyEnv splits and decodes one "<version>:<base64 key>" entry.
func parsePepperKeyEnv(entry string) (version string, key []byte, err error) {
	version, b64, ok := strings.Cut(entry, ":")
	if !ok {
		return "", nil, errors.New(`expected "<version>:<base64 key>"`)
	}
	key, err = base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", nil, fmt.Errorf("decoding key: %w", err)
	}
	return version, key, nil
}

// PepperHash takes an argon2id-PHC-encoded hash (as [argon2id.CreateHash]
// returns) and returns the peppered form this package actually stores:
// HMAC-SHA256(p.current key, raw digest) in place of the raw digest, with
// the salt and cost parameters left in plain sight — they are not secret —
// and p.current's version prefixed so [Pepper.VerifyHash] knows which key to
// recompute against, even after rotation moves p.current to a new version.
func (p *Pepper) PepperHash(hash string) (string, error) {
	params, salt, digest, err := argon2id.DecodeHash(hash)
	if err != nil {
		return "", fmt.Errorf("auth: decoding hash to pepper it: %w", err)
	}
	peppered := hmacDigest(p.current.key, digest)
	encoded := encodeArgon2idHash(params, salt, peppered)
	return p.current.version + ":" + encoded, nil
}

// VerifyHash reports whether candidate — put through the same argon2id cost
// parameters and salt stored's PHC portion carries — peppers to the digest
// stored carries, comparing in constant time. stored must be in the form
// [Pepper.PepperHash] returns; a value from any Pepper built with the
// matching key version verifies it, current or retired.
func (p *Pepper) VerifyHash(candidate, stored string) (bool, error) {
	version, encoded, ok := strings.Cut(stored, ":")
	if !ok {
		return false, errMalformedPeppered
	}
	k, ok := p.all[version]
	if !ok {
		return false, ErrUnknownPepperVersion
	}
	params, salt, storedDigest, err := argon2id.DecodeHash(encoded)
	if err != nil {
		return false, fmt.Errorf("auth: decoding peppered hash: %w", err)
	}
	candidateDigest := argon2.IDKey([]byte(candidate), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	peppered := hmacDigest(k.key, candidateDigest)
	return hmac.Equal(peppered, storedDigest), nil
}

// hmacDigest returns HMAC-SHA256(key, digest).
func hmacDigest(key, digest []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(digest)
	return mac.Sum(nil)
}

// encodeArgon2idHash re-serializes params, salt and digest into the same PHC
// shape [argon2id.CreateHash] produces, so [argon2id.DecodeHash] can read a
// peppered hash back exactly as it reads an unpeppered one — the peppered
// digest happens to be 32 bytes too (HMAC-SHA256's output size), so it
// round-trips through the same base64 field [Pepper.VerifyHash] decodes.
func encodeArgon2idHash(params *argon2id.Params, salt, digest []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.Memory, params.Iterations, params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest))
}

// gcmNonceSize is the standard AES-GCM nonce length [Pepper.Encrypt] and
// [Pepper.Decrypt] use.
const gcmNonceSize = 12

// Encrypt seals plaintext (a parent's TOTP secret) with AES-256-GCM under
// p.current's key, prefixed with that key's version and a fresh random
// nonce, so [Pepper.Decrypt] can find the right key and nonce again. The
// version is length-prefixed rather than delimited, because the nonce and
// ciphertext that follow it are arbitrary bytes that could otherwise contain
// a delimiter.
func (p *Pepper) Encrypt(plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(p.current.key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("auth: generating gcm nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	version := p.current.version
	if len(version) > 255 {
		return nil, errors.New("auth: pepper key version too long to encode")
	}
	out := make([]byte, 0, 1+len(version)+len(nonce)+len(ciphertext))
	// #nosec G115 -- len(version) was just checked above to be <= 255, so
	// this conversion never truncates.
	out = append(out, byte(len(version)))
	out = append(out, version...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt reverses [Pepper.Encrypt], using whichever key (current or
// retired) matches the version data was encrypted under.
func (p *Pepper) Decrypt(data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, errMalformedPeppered
	}
	vlen := int(data[0])
	if len(data) < 1+vlen+gcmNonceSize {
		return nil, errMalformedPeppered
	}
	rest := data[1+vlen:]
	nonce, ciphertext := rest[:gcmNonceSize], rest[gcmNonceSize:]

	k, ok := p.all[string(data[1:1+vlen])]
	if !ok {
		return nil, ErrUnknownPepperVersion
	}
	gcm, err := newGCM(k.key)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: decrypting: %w", err)
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("auth: building aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("auth: building gcm: %w", err)
	}
	return gcm, nil
}
