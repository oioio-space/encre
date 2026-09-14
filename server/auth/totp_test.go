package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
	"github.com/pquerna/otp/totp"
)

func TestEnrollTOTPReturnsSecretAndQR(t *testing.T) {
	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	if enroll.Secret == "" {
		t.Error("EnrollTOTP().Secret is empty")
	}
	// PNG signature.
	if len(enroll.QRPNG) < 8 || string(enroll.QRPNG[1:4]) != "PNG" {
		t.Error("EnrollTOTP().QRPNG does not look like a PNG file")
	}
}

func codeAt(t *testing.T, secret string, when time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, when)
	if err != nil {
		t.Fatalf("totp.GenerateCode() error = %v", err)
	}
	return code
}

// TestValidateTOTPWindow checks the +/-1 period skew ENCRE_04 §7 requires.
// ValidateTOTP itself has no notion of replay (see its doc comment): that is
// [VerifyParentTOTP]'s job, and is tested separately, atomically, against a
// real store.
func TestValidateTOTPWindow(t *testing.T) {
	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	secret := enroll.Secret

	now := time.Unix(1_700_000_000, 0).UTC()
	// Align to a period boundary so "previous"/"next" are unambiguous.
	now = time.Unix(now.Unix()/30*30, 0).UTC()

	prevCode := codeAt(t, secret, now.Add(-30*time.Second))
	curCode := codeAt(t, secret, now)
	nextCode := codeAt(t, secret, now.Add(30*time.Second))
	tooOldCode := codeAt(t, secret, now.Add(-time.Hour))

	ok, _, err := auth.ValidateTOTP(prevCode, secret, now)
	if err != nil {
		t.Fatalf("ValidateTOTP(prev) error = %v", err)
	}
	if !ok {
		t.Error("ValidateTOTP() with the previous period's code = false, want true (±1 skew)")
	}

	ok, _, err = auth.ValidateTOTP(curCode, secret, now)
	if err != nil {
		t.Fatalf("ValidateTOTP(cur) error = %v", err)
	}
	if !ok {
		t.Error("ValidateTOTP() with the current period's code = false, want true")
	}

	ok, _, err = auth.ValidateTOTP(nextCode, secret, now)
	if err != nil {
		t.Fatalf("ValidateTOTP(next) error = %v", err)
	}
	if !ok {
		t.Error("ValidateTOTP() with the next period's code = false, want true (±1 skew)")
	}

	ok, _, err = auth.ValidateTOTP(tooOldCode, secret, now)
	if err != nil {
		t.Fatalf("ValidateTOTP(too old) error = %v", err)
	}
	if ok {
		t.Error("ValidateTOTP() with a code from an hour ago = true, want false")
	}
}

func TestValidateTOTPWrongCode(t *testing.T) {
	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	ok, _, err := auth.ValidateTOTP("000000", enroll.Secret, time.Now())
	if err != nil {
		t.Fatalf("ValidateTOTP() error = %v", err)
	}
	if ok {
		t.Error("ValidateTOTP() with a made-up code = true, want false")
	}
}

// TestValidateTOTPMalformedCodeIsRejectedNotErrored is the regression test
// for a proven 500: pquerna/otp's ValidateCustom rejects a code of the wrong
// length before comparing anything and returns
// otp.ErrValidateInputInvalidLength, which an earlier version of this
// package wrapped as an opaque error instead of treating as "wrong code". A
// parent who mistypes a digit must see a rejected login, not a crash.
func TestValidateTOTPMalformedCodeIsRejectedNotErrored(t *testing.T) {
	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}

	for _, code := range []string{"", "12345", "1234567", "abcdef"} {
		ok, _, err := auth.ValidateTOTP(code, enroll.Secret, time.Now())
		if err != nil {
			t.Errorf("ValidateTOTP(%q) error = %v, want nil", code, err)
		}
		if ok {
			t.Errorf("ValidateTOTP(%q) = true, want false", code)
		}
	}
}

func TestVerifyParentTOTPPersistsFreshnessAndRejectsReplay(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()

	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	p := &store.Parent{
		ID: "p1", Email: "parent@example.com",
		PassHash: []byte("x"), TOTPSecret: []byte(enroll.Secret), CreatedAt: time.Now(),
	}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	now := time.Unix(1_700_000_000, 0).UTC()
	code := codeAt(t, enroll.Secret, now)

	if err := auth.VerifyParentTOTP(ctx, db, "p1", code, now); err != nil {
		t.Fatalf("VerifyParentTOTP() error = %v", err)
	}

	if err := auth.VerifyParentTOTP(ctx, db, "p1", code, now); !errors.Is(err, auth.ErrInvalidTOTPCode) {
		t.Errorf("VerifyParentTOTP() replay: error = %v, want ErrInvalidTOTPCode", err)
	}
}

func TestVerifyParentTOTPWrongCode(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()

	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	p := &store.Parent{
		ID: "p1", Email: "parent@example.com",
		PassHash: []byte("x"), TOTPSecret: []byte(enroll.Secret), CreatedAt: time.Now(),
	}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	err = auth.VerifyParentTOTP(ctx, db, "p1", "000000", time.Now())
	if !errors.Is(err, auth.ErrInvalidTOTPCode) {
		t.Errorf("VerifyParentTOTP() with a wrong code: error = %v, want ErrInvalidTOTPCode", err)
	}
}

func TestVerifyParentTOTPForSessionSetsFreshness(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()

	enroll, err := auth.EnrollTOTP("parent@example.com")
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	p := &store.Parent{
		ID: "p1", Email: "parent@example.com",
		PassHash: []byte("x"), TOTPSecret: []byte(enroll.Secret), CreatedAt: time.Now(),
	}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	now := time.Unix(1_700_000_000, 0).UTC()
	token, err := auth.CreateSession(ctx, db, store.SessionParent, "p1", now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sess, err := auth.LookupSession(ctx, db, token, store.SessionParent, now)
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}
	if auth.TOTPFresh(sess, now) {
		t.Fatal("TOTPFresh() on a brand-new session = true, want false before any code is verified")
	}

	code := codeAt(t, enroll.Secret, now)
	if err := auth.VerifyParentTOTPForSession(ctx, db, sess, code, now); err != nil {
		t.Fatalf("VerifyParentTOTPForSession() error = %v", err)
	}

	sess, err = auth.LookupSession(ctx, db, token, store.SessionParent, now)
	if err != nil {
		t.Fatalf("LookupSession() after verify: error = %v", err)
	}
	if !auth.TOTPFresh(sess, now) {
		t.Error("TOTPFresh() right after a successful verify = false, want true")
	}
	if auth.TOTPFresh(sess, now.Add(auth.TOTPFreshDuration+time.Second)) {
		t.Error("TOTPFresh() past TOTPFreshDuration = true, want false")
	}
}

func TestVerifyParentTOTPForSessionRejectsChildSession(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sess, err := auth.LookupSession(ctx, db, token, store.SessionChild, now)
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}

	err = auth.VerifyParentTOTPForSession(ctx, db, sess, "000000", now)
	if !errors.Is(err, auth.ErrInvalidTOTPCode) {
		t.Errorf("VerifyParentTOTPForSession() on a child session: error = %v, want ErrInvalidTOTPCode", err)
	}
}
