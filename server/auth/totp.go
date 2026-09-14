package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"time"

	"github.com/oioio-space/encre/server/store"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// totpIssuer names ENCRE as the issuer in the otpauth:// URL, so an
// authenticator app labels the entry it creates.
const totpIssuer = "ENCRE"

// totpPeriod is the TOTP step length, RFC 6238's default and what
// [EnrollTOTP] and [ValidateTOTP] assume throughout.
const totpPeriod = 30 * time.Second

// TOTPFreshDuration is how long a validated TOTP code keeps a parent
// session's sensitive actions unlocked (ENCRE_04 §7).
const TOTPFreshDuration = time.Hour

// qrSize is the QR code's side length in pixels, large enough to scan
// comfortably from a phone screen.
const qrSize = 256

// ErrInvalidTOTPCode is returned by [VerifyParentTOTP] for a code that does
// not validate, whether because it is wrong, too old, or already used.
var ErrInvalidTOTPCode = errors.New("auth: invalid totp code")

// TOTPEnrollment is what [EnrollTOTP] hands back for a parent to finish
// enrolling a second factor.
type TOTPEnrollment struct {
	// Secret is the base32 TOTP secret. Store it (as
	// [server/store.Parent.TOTPSecret]) only after the parent has proven
	// they scanned it, by validating one code from it; never log it.
	Secret string
	// QRPNG is a PNG-encoded QR code of the otpauth:// URL, for the parent
	// to scan with an authenticator app instead of typing Secret by hand.
	QRPNG []byte
}

// EnrollTOTP generates a new TOTP secret for accountEmail and renders it as a
// scannable QR code. Nothing is persisted: the caller stores
// [TOTPEnrollment.Secret] only once the parent has proven they captured it.
func EnrollTOTP(accountEmail string) (*TOTPEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountEmail,
	})
	if err != nil {
		return nil, fmt.Errorf("generating totp key: %w", err)
	}

	img, err := key.Image(qrSize, qrSize)
	if err != nil {
		return nil, fmt.Errorf("rendering totp qr code: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encoding totp qr code: %w", err)
	}

	return &TOTPEnrollment{Secret: key.Secret(), QRPNG: buf.Bytes()}, nil
}

// ValidateTOTP reports whether code is valid for secret at time now, within
// the +/-1 period skew ENCRE_04 §7 requires. It is a pure time-window check
// with no notion of replay — see [VerifyParentTOTP] for the atomic
// replay-rejecting version this package actually uses against a stored
// secret — so the same code validates every time it is called with it,
// deliberately: replay must be enforced exactly once, by whichever single
// statement records the step as used, not by this function guessing whether
// it has been called before.
//
// On success it returns the step that matched. A malformed code (wrong
// length or non-digits) is reported as ok == false, err == nil, the same as
// a wrong code: [github.com/pquerna/otp/totp.ValidateCustom] itself returns
// an error for that input shape, which — left unhandled — would surface a
// mistyped code to a caller as a 500 rather than a rejected login.
func ValidateTOTP(code, secret string, now time.Time) (ok bool, step int64, err error) {
	current := now.Unix() / int64(totpPeriod.Seconds())
	opts := totp.ValidateOpts{
		Period:    uint(totpPeriod.Seconds()),
		Skew:      0,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}
	for _, delta := range [3]int64{0, -1, 1} {
		s := current + delta
		valid, err := totp.ValidateCustom(code, secret, time.Unix(s*int64(totpPeriod.Seconds()), 0), opts)
		switch {
		case errors.Is(err, otp.ErrValidateInputInvalidLength):
			return false, 0, nil
		case err != nil:
			return false, 0, fmt.Errorf("validating totp code: %w", err)
		case valid:
			return true, s, nil
		}
	}
	return false, 0, nil
}

// VerifyParentTOTP validates code against parentID's enrolled TOTP secret
// and atomically marks the matching step used, so the same code cannot
// validate twice even from two concurrent requests. It returns
// [ErrInvalidTOTPCode] for a wrong, expired or replayed code, and never
// includes the secret or the code in any error.
func VerifyParentTOTP(ctx context.Context, db *store.Store, parentID, code string, now time.Time) error {
	p, err := db.ParentByID(ctx, parentID)
	if err != nil {
		return err
	}
	if len(p.TOTPSecret) == 0 {
		return ErrInvalidTOTPCode
	}

	ok, step, err := ValidateTOTP(code, string(p.TOTPSecret), now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidTOTPCode
	}

	first, err := markTOTPStepUsed(ctx, db, parentID, step, now)
	if err != nil {
		return err
	}
	if !first {
		return ErrInvalidTOTPCode
	}
	return nil
}

// VerifyParentTOTPForSession validates code for the parent behind sess and,
// on success, extends sess's TOTP freshness window
// ([server/store.Session.TOTPOKUntil]) by [TOTPFreshDuration] from now — the
// step ENCRE_04 §7's sensitive parent actions require before they proceed.
// sess must be a parent session; a child session always fails with
// [ErrInvalidTOTPCode].
func VerifyParentTOTPForSession(ctx context.Context, db *store.Store, sess *store.Session, code string, now time.Time) error {
	if sess.Kind != store.SessionParent {
		return ErrInvalidTOTPCode
	}
	if err := VerifyParentTOTP(ctx, db, sess.SubjectID, code, now); err != nil {
		return err
	}
	_, err := db.DB().ExecContext(ctx,
		`UPDATE sessions SET totp_ok_until = ? WHERE token = ?`,
		now.Add(TOTPFreshDuration).Unix(), sess.Token)
	if err != nil {
		return fmt.Errorf("updating session totp freshness: %w", err)
	}
	return nil
}

// totpReplayWindow is how long a totp_used row is kept before
// [PurgeExpiredTOTPUses] deletes it: comfortably past the +/-1 period skew
// ([totpPeriod]) this package ever validates against, so a row is never
// purged while it could still matter.
const totpReplayWindow = 120 * time.Second

// markTOTPStepUsed and [PurgeExpiredTOTPUses] read and write totp_used
// (migration 0003) directly through [server/store.Store.DB]: that table
// exists solely for this package's replay check and has no reason to grow
// into a server/store method.
//
// markTOTPStepUsed reports whether this call is the first to mark
// (parentID, step) used. The single INSERT ... ON CONFLICT DO NOTHING
// statement is what makes this atomic under concurrent callers: two
// requests racing to verify the same code both reach this function, but
// SQLite's write serialization guarantees only one of their INSERTs
// actually inserts a row, and RowsAffected tells each caller which one it
// was — no SELECT-then-UPDATE window for both to slip through.
func markTOTPStepUsed(ctx context.Context, db *store.Store, parentID string, step int64, now time.Time) (first bool, err error) {
	result, err := db.DB().ExecContext(ctx,
		`INSERT INTO totp_used (parent_id, step, seen_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		parentID, step, now.Unix())
	if err != nil {
		return false, fmt.Errorf("recording totp step: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking totp step insert: %w", err)
	}
	return n == 1, nil
}

// PurgeExpiredTOTPUses deletes every totp_used row older than
// [totpReplayWindow], keeping the table from growing without bound. It does
// not run automatically on every [VerifyParentTOTP] call — that would turn
// one write into two on the hot path — so callers must run it periodically,
// the same way [server/store.Store.PurgeExpiredSessions] is.
func PurgeExpiredTOTPUses(ctx context.Context, db *store.Store, now time.Time) error {
	_, err := db.DB().ExecContext(ctx,
		`DELETE FROM totp_used WHERE seen_at < ?`, now.Add(-totpReplayWindow).Unix())
	if err != nil {
		return fmt.Errorf("purging totp_used: %w", err)
	}
	return nil
}
