package store_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

func newRun(id, childID string) *store.Run {
	return &store.Run{
		ID:        id,
		ChildID:   childID,
		StartedAt: time.Unix(1_700_000_000, 0).UTC(),
		Rank:      1,
		Deck:      engine.Deck{Seed: 42, Boss: "chuchoteur"},
		Targets:   [3]float64{1, 2, 3},
		Talismans: []engine.TalismanID{engine.Perroquet},
		Rooms:     [2]engine.RoomID{"repos", "encrier"},
		FailedAt:  -1,
	}
}

func TestCreateAndFetchRun(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	got, err := db.RunByID(ctx, "run1")
	if err != nil {
		t.Fatalf("RunByID() error = %v", err)
	}
	if got.ChildID != childID || got.Rank != 1 || got.Deck.Seed != 42 {
		t.Errorf("RunByID() = %+v", got)
	}
	if len(got.Talismans) != 1 || got.Talismans[0] != engine.Perroquet {
		t.Errorf("Talismans = %+v", got.Talismans)
	}
	if got.Rooms != r.Rooms {
		t.Errorf("Rooms = %+v, want %+v", got.Rooms, r.Rooms)
	}
	if got.Applied {
		t.Error("Applied = true, want false for a fresh run")
	}
}

func TestRunByIDNotFound(t *testing.T) {
	db := openTestStore(t)
	if _, err := db.RunByID(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RunByID() error = %v, want ErrNotFound", err)
	}
}

func TestMarkAppliedIsIdempotent(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	if err := db.MarkApplied(ctx, "run1"); err != nil {
		t.Fatalf("first MarkApplied() error = %v", err)
	}

	err := db.MarkApplied(ctx, "run1")
	if !errors.Is(err, engine.ErrAlreadyApplied) {
		t.Errorf("second MarkApplied() error = %v, want engine.ErrAlreadyApplied", err)
	}

	got, err := db.RunByID(ctx, "run1")
	if err != nil {
		t.Fatalf("RunByID() error = %v", err)
	}
	if !got.Applied {
		t.Error("Applied = false, want true after MarkApplied")
	}
}

func TestMarkAppliedNotFound(t *testing.T) {
	db := openTestStore(t)
	if err := db.MarkApplied(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("MarkApplied() error = %v, want ErrNotFound", err)
	}
}

// TestMarkAppliedIsAtomicUnderConcurrency races two callers marking the same
// run applied: exactly one succeeds, whatever the interleaving, because the
// decision is made by a single UPDATE ... WHERE applied = 0.
func TestMarkAppliedIsAtomicUnderConcurrency(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		succeeds int
	)
	for range 8 {
		wg.Go(func() {
			if err := db.MarkApplied(ctx, "run1"); err == nil {
				mu.Lock()
				succeeds++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if succeeds != 1 {
		t.Errorf("successful MarkApplied() calls = %d, want 1", succeeds)
	}
}

func TestSaveAttempts(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	seedItem(t, db, childID, "item1")
	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	attempts := []*store.Attempt{
		{RunID: "run1", Idx: 0, ItemID: "item1", Manche: 0, Correct: true, Typed: "chat", Millis: 1200, Chips: 10, Mult: 1.5},
		{RunID: "run1", Idx: 1, ItemID: "item1", Manche: 0, Blind: true, Correct: false, Typed: "cha", Millis: 900},
	}
	if err := db.SaveAttempts(ctx, attempts); err != nil {
		t.Fatalf("SaveAttempts() error = %v", err)
	}

	var count int
	if err := db.DB().QueryRowContext(ctx, `SELECT count(*) FROM attempts WHERE run_id = ?`, "run1").Scan(&count); err != nil {
		t.Fatalf("counting attempts: %v", err)
	}
	if count != 2 {
		t.Errorf("attempts count = %d, want 2", count)
	}
}

func TestCreateRunDuplicateIDFails(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := db.CreateRun(ctx, r); err == nil {
		t.Error("CreateRun() with duplicate ID: want error, got nil")
	}
}

func TestCreateRunRejectsUnknownChild(t *testing.T) {
	db := openTestStore(t)
	r := newRun("run1", "ghost")
	if err := db.CreateRun(t.Context(), r); err == nil {
		t.Error("CreateRun() with unknown child: want error, got nil")
	}
}

func TestRunByIDWithFinishedAtAndFailedAt(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	r := newRun("run1", childID)
	r.FinishedAt = time.Unix(1_700_003_600, 0).UTC()
	r.FailedAt = 1
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	got, err := db.RunByID(ctx, "run1")
	if err != nil {
		t.Fatalf("RunByID() error = %v", err)
	}
	if !got.FinishedAt.Equal(r.FinishedAt) {
		t.Errorf("FinishedAt = %v, want %v", got.FinishedAt, r.FinishedAt)
	}
	if got.FailedAt != 1 {
		t.Errorf("FailedAt = %d, want 1", got.FailedAt)
	}
}

// TestRunByIDCorruptedJSONFails checks that a *_json column that somehow
// stopped being valid JSON surfaces as a clear error from RunByID rather
// than a panic or silently wrong data.
func TestRunByIDCorruptedJSONFails(t *testing.T) {
	columns := []string{"deck_json", "targets_json", "talismans_json", "rooms_json"}
	for _, column := range columns {
		t.Run(column, func(t *testing.T) {
			db := openTestStore(t)
			ctx := t.Context()
			childID := seedChild(t, db, "child1")

			r := newRun("run1", childID)
			if err := db.CreateRun(ctx, r); err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}

			_, err := db.DB().ExecContext(ctx,
				`UPDATE runs SET `+column+` = 'not json' WHERE id = ?`, "run1")
			if err != nil {
				t.Fatalf("corrupting %s: %v", column, err)
			}

			if _, err := db.RunByID(ctx, "run1"); err == nil {
				t.Errorf("RunByID() with corrupted %s: want error, got nil", column)
			}
		})
	}
}

func TestSaveAttemptsConflictUpdatesExisting(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	seedItem(t, db, childID, "item1")
	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	first := []*store.Attempt{{RunID: "run1", Idx: 0, ItemID: "item1", Correct: false, Typed: "cha"}}
	if err := db.SaveAttempts(ctx, first); err != nil {
		t.Fatalf("first SaveAttempts() error = %v", err)
	}
	second := []*store.Attempt{{RunID: "run1", Idx: 0, ItemID: "item1", Correct: true, Typed: "chat"}}
	if err := db.SaveAttempts(ctx, second); err != nil {
		t.Fatalf("second SaveAttempts() error = %v", err)
	}

	var correct bool
	err := db.DB().QueryRowContext(ctx, `SELECT correct FROM attempts WHERE run_id = ? AND idx = 0`, "run1").Scan(&correct)
	if err != nil {
		t.Fatalf("querying attempt: %v", err)
	}
	if !correct {
		t.Error("correct = false, want true after the conflicting save")
	}
}

func TestDeletingChildCascadesToRuns(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	r := newRun("run1", childID)
	if err := db.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	if _, err := db.DB().ExecContext(ctx, `DELETE FROM children WHERE id = ?`, childID); err != nil {
		t.Fatalf("deleting child: %v", err)
	}

	if _, err := db.RunByID(ctx, "run1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RunByID() after cascade delete: error = %v, want ErrNotFound", err)
	}
}

func TestCreateRunFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	r := newRun("run1", "child1")
	if err := db.CreateRun(t.Context(), r); err == nil {
		t.Error("CreateRun() on a closed database: want error, got nil")
	}
}

func TestRunByIDFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.RunByID(t.Context(), "run1"); err == nil {
		t.Error("RunByID() on a closed database: want error, got nil")
	}
}

func TestMarkAppliedFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.MarkApplied(t.Context(), "run1"); err == nil {
		t.Error("MarkApplied() on a closed database: want error, got nil")
	}
}

func TestSaveAttemptsFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	attempts := []*store.Attempt{{RunID: "run1", Idx: 0, ItemID: "item1"}}
	if err := db.SaveAttempts(t.Context(), attempts); err == nil {
		t.Error("SaveAttempts() on a closed database: want error, got nil")
	}
}
