package store_test

import (
	"testing"
)

func TestAddAndFetchPlayTime(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	if err := db.AddPlayTime(ctx, childID, 19_000, 120); err != nil {
		t.Fatalf("AddPlayTime() error = %v", err)
	}
	if err := db.AddPlayTime(ctx, childID, 19_000, 60); err != nil {
		t.Fatalf("second AddPlayTime() error = %v", err)
	}

	seconds, bonus, err := db.PlayTime(ctx, childID, 19_000)
	if err != nil {
		t.Fatalf("PlayTime() error = %v", err)
	}
	if seconds != 180 {
		t.Errorf("seconds = %d, want 180", seconds)
	}
	if bonus != 0 {
		t.Errorf("bonus = %d, want 0", bonus)
	}
}

func TestPlayTimeUnknownDayIsZero(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	seconds, bonus, err := db.PlayTime(ctx, childID, 19_000)
	if err != nil {
		t.Fatalf("PlayTime() error = %v", err)
	}
	if seconds != 0 || bonus != 0 {
		t.Errorf("PlayTime() = (%d, %d), want (0, 0)", seconds, bonus)
	}
}

func TestAddPlayTimeFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.AddPlayTime(t.Context(), "child1", 1, 60); err == nil {
		t.Error("AddPlayTime() on a closed database: want error, got nil")
	}
}

func TestAddBonusSecondsAccumulatesWithoutTouchingPlayedSeconds(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	if err := db.AddPlayTime(ctx, childID, 19_000, 300); err != nil {
		t.Fatalf("AddPlayTime() error = %v", err)
	}
	if err := db.AddBonusSeconds(ctx, childID, 19_000, 120); err != nil {
		t.Fatalf("AddBonusSeconds() error = %v", err)
	}
	if err := db.AddBonusSeconds(ctx, childID, 19_000, 60); err != nil {
		t.Fatalf("second AddBonusSeconds() error = %v", err)
	}

	seconds, bonus, err := db.PlayTime(ctx, childID, 19_000)
	if err != nil {
		t.Fatalf("PlayTime() error = %v", err)
	}
	if seconds != 300 {
		t.Errorf("seconds = %d, want 300 (untouched by AddBonusSeconds)", seconds)
	}
	if bonus != 180 {
		t.Errorf("bonus = %d, want 180", bonus)
	}
}

func TestAddBonusSecondsCreatesRowWhenNoneExists(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	if err := db.AddBonusSeconds(ctx, childID, 19_001, 90); err != nil {
		t.Fatalf("AddBonusSeconds() error = %v", err)
	}
	seconds, bonus, err := db.PlayTime(ctx, childID, 19_001)
	if err != nil {
		t.Fatalf("PlayTime() error = %v", err)
	}
	if seconds != 0 || bonus != 90 {
		t.Errorf("PlayTime() = (%d, %d), want (0, 90)", seconds, bonus)
	}
}

func TestAddBonusSecondsFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.AddBonusSeconds(t.Context(), "child1", 1, 60); err == nil {
		t.Error("AddBonusSeconds() on a closed database: want error, got nil")
	}
}

func TestPlayTimeFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, _, err := db.PlayTime(t.Context(), "child1", 1); err == nil {
		t.Error("PlayTime() on a closed database: want error, got nil")
	}
}
