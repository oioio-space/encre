package net_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/oioio-space/encre/client/net"
	"github.com/oioio-space/encre/engine"
)

// genAttempt draws one recorded attempt, varied enough (typed text, whether
// it was correct, blind, a Rencontre copy) to exercise every field
// [net.SaveRunState] and [net.LoadRunState] round-trip.
func genAttempt(t *rapid.T) engine.Attempt {
	return engine.Attempt{
		WordID:  rapid.StringN(1, 8, -1).Draw(t, "word-id"),
		Manche:  rapid.IntRange(0, 2).Draw(t, "manche"),
		Blind:   rapid.Bool().Draw(t, "blind"),
		Copy:    rapid.Bool().Draw(t, "copy"),
		Correct: rapid.Bool().Draw(t, "correct"),
		Typed:   rapid.String().Draw(t, "typed"),
		Millis:  rapid.IntRange(0, 60_000).Draw(t, "millis"),
	}
}

// genRun draws a run carrying a random number of attempts, standing in for
// however far into a run the child has played.
func genRun(t *rapid.T) engine.Run {
	return engine.Run{
		ID:      rapid.StringN(1, 12, -1).Draw(t, "run-id"),
		ChildID: rapid.StringN(1, 12, -1).Draw(t, "child-id"),
		Rank:    rapid.IntRange(0, 5).Draw(t, "rank"),
		Targets: [3]float64{
			rapid.Float64Range(0, 500).Draw(t, "target-0"),
			rapid.Float64Range(0, 500).Draw(t, "target-1"),
			rapid.Float64Range(0, 500).Draw(t, "target-2"),
		},
		Attempts: rapid.SliceOf(rapid.Custom(genAttempt)).Draw(t, "attempts"),
		Revanche: [3]bool{
			rapid.Bool().Draw(t, "revanche-0"),
			rapid.Bool().Draw(t, "revanche-1"),
			rapid.Bool().Draw(t, "revanche-2"),
		},
		Cahier: rapid.Bool().Draw(t, "cahier"),
	}
}

// TestSaveThenLoadRunStateRoundTrips is the property brief T27 asks for:
// for any run, however many attempts it has accumulated, saving and reloading
// it returns exactly the same state — never a panic, never a silent drop of a
// word the child already typed.
func TestSaveThenLoadRunStateRoundTrips(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := newMemStore()
		run := genRun(t)
		now := time.Now().UTC().Round(time.Second)

		if err := net.SaveRunState(store, run, now); err != nil {
			t.Fatalf("SaveRunState: %v", err)
		}

		got, ok := net.LoadRunState(store)
		if !ok {
			t.Fatal("LoadRunState: ok = false after a successful save")
		}
		want := net.RunState{Version: net.RunStateVersion, SavedAt: now, Run: run}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("LoadRunState() mismatch (-want +got):\n%s", diff)
		}
	})
}

// TestSaveRunStateAppendedAfterEveryWordStillRoundTrips is the concrete
// shape of "saved after every word": each call appends one more attempt
// before saving, as the game does mid-run, and every intermediate save must
// still read back correctly — not just the final, complete run.
func TestSaveRunStateAppendedAfterEveryWordStillRoundTrips(t *testing.T) {
	store := newMemStore()
	run := engine.Run{ID: "run-1", ChildID: "child-1"}
	words := []engine.Attempt{
		{WordID: "chat", Correct: true},
		{WordID: "école", Correct: false, Typed: "ecole"},
		{WordID: "forêt", Correct: true, Blind: true},
	}

	for i, a := range words {
		run.Attempts = append(run.Attempts, a)
		now := time.Now()
		if err := net.SaveRunState(store, run, now); err != nil {
			t.Fatalf("SaveRunState after word %d: %v", i, err)
		}

		got, ok := net.LoadRunState(store)
		if !ok {
			t.Fatalf("LoadRunState after word %d: ok = false", i)
		}
		if len(got.Run.Attempts) != i+1 {
			t.Fatalf("after word %d: %d attempts saved, want %d", i, len(got.Run.Attempts), i+1)
		}
		if diff := cmp.Diff(run.Attempts, got.Run.Attempts); diff != "" {
			t.Errorf("after word %d, attempts mismatch (-want +got):\n%s", i, diff)
		}
	}
}

func TestLoadRunStateOfAnEmptyStoreIsNotAResumableRun(t *testing.T) {
	store := newMemStore()

	if _, ok := net.LoadRunState(store); ok {
		t.Error("LoadRunState of an empty store: ok = true, want false")
	}
}

func TestLoadRunStateOfCorruptedDataIsNotAResumableRunAndDoesNotPanic(t *testing.T) {
	store := newMemStore()
	store.data["run"] = []byte("{not valid json")

	if _, ok := net.LoadRunState(store); ok {
		t.Error("LoadRunState of corrupted data: ok = true, want false")
	}
}

func TestLoadRunStateOfAnOlderVersionIsNotResumed(t *testing.T) {
	store := newMemStore()
	store.data["run"] = []byte(`{"Version":0,"Run":{"ID":"old-run"}}`)

	if _, ok := net.LoadRunState(store); ok {
		t.Error("LoadRunState of an older schema version: ok = true, want false")
	}
}

func TestClearRunStateRemovesTheSave(t *testing.T) {
	store := newMemStore()
	run := engine.Run{ID: "run-1"}
	if err := net.SaveRunState(store, run, time.Now()); err != nil {
		t.Fatalf("SaveRunState: %v", err)
	}

	if err := net.ClearRunState(store); err != nil {
		t.Fatalf("ClearRunState: %v", err)
	}
	if _, ok := net.LoadRunState(store); ok {
		t.Error("LoadRunState after ClearRunState: ok = true, want false")
	}
}
