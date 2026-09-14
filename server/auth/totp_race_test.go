package auth_test

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// TestVerifyParentTOTPRaceIsAtomic is the regression test for the race an
// independent audit found: server/store.OpenMemory (used by every other test
// in this package) is capped at a single connection, which makes a
// SELECT-then-UPDATE race structurally unobservable there — the single
// connection serializes everything by accident. This test opens a real,
// file-backed [server/store.Store] (multiple connections, ENCRE's actual
// deployment shape) and fires the same TOTP code at VerifyParentTOTP from
// many goroutines at once. Run with -race; exactly one goroutine must
// succeed.
func TestVerifyParentTOTPRaceIsAtomic(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "encre.sqlite")
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

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

	const concurrent = 20
	var successes atomic.Int64
	var wg sync.WaitGroup
	for range concurrent {
		wg.Go(func() {
			if err := auth.VerifyParentTOTP(ctx, db, "p1", code, now); err == nil {
				successes.Add(1)
			}
		})
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Errorf("concurrent VerifyParentTOTP() with the same code: %d succeeded, want exactly 1", got)
	}
}
