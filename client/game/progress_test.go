package game_test

import (
	"errors"
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/net"
	"github.com/oioio-space/encre/engine"
)

// fakeStore is an in-memory net.Store for game's tests: no browser, no
// filesystem, no server.
type fakeStore struct {
	data map[string][]byte
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: map[string][]byte{}}
}

func (f *fakeStore) Load(key string) ([]byte, error) {
	data, ok := f.data[key]
	if !ok {
		return nil, net.ErrNotFound
	}
	return data, nil
}

func (f *fakeStore) Save(key string, data []byte) error {
	f.data[key] = data
	return nil
}

func (f *fakeStore) Clear(key string) error {
	delete(f.data, key)
	return nil
}

func TestRecordAttemptSavesAfterEveryWordAndResumeSeesThem(t *testing.T) {
	store := newFakeStore()
	session := game.NewRunSession(store, engine.Run{ID: "run-1", ChildID: "child-1"})

	attempts := []engine.Attempt{
		{WordID: "chat", Correct: true},
		{WordID: "école", Correct: false, Typed: "ecole"},
		{WordID: "forêt", Correct: true, Blind: true},
	}
	for i, a := range attempts {
		if err := session.RecordAttempt(a); err != nil {
			t.Fatalf("RecordAttempt(word %d): %v", i, err)
		}

		// A tab closing right after this word must resume with exactly the
		// words recorded so far — not fewer, not the whole run pre-filled.
		resumed, ok := game.ResumeRunSession(store)
		if !ok {
			t.Fatalf("ResumeRunSession after word %d: ok = false", i)
		}
		if got, want := len(resumed.Run().Attempts), i+1; got != want {
			t.Fatalf("after word %d: resumed with %d attempts, want %d", i, got, want)
		}
	}

	if got := session.Run().Attempts; len(got) != len(attempts) {
		t.Errorf("session.Run().Attempts has %d entries, want %d", len(got), len(attempts))
	}
}

func TestResumeRunSessionOfAFreshStoreHasNothingToResume(t *testing.T) {
	if _, ok := game.ResumeRunSession(newFakeStore()); ok {
		t.Error("ResumeRunSession of a fresh store: ok = true, want false")
	}
}

func TestResumeRunSessionOfCorruptedDataStartsCleanRatherThanPanicking(t *testing.T) {
	store := newFakeStore()
	store.data["run"] = []byte("not json at all {{{")

	if _, ok := game.ResumeRunSession(store); ok {
		t.Error("ResumeRunSession of corrupted data: ok = true, want false")
	}
}

func TestFinishQueuesTheJobAndClearsTheRunState(t *testing.T) {
	store := newFakeStore()
	session := game.NewRunSession(store, engine.Run{ID: "run-1"})
	if err := session.RecordAttempt(engine.Attempt{WordID: "chat", Correct: true}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}

	queue := net.NewQueue(store)
	talismans := []engine.TalismanID{engine.Perroquet}
	if err := session.Finish(queue, talismans, [3]bool{true, false, false}, true); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	jobs, err := queue.Jobs()
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(Jobs()) = %d, want 1", len(jobs))
	}
	job := jobs[0]
	if job.RunID != "run-1" {
		t.Errorf("job.RunID = %q, want %q", job.RunID, "run-1")
	}
	if len(job.Attempts) != 1 {
		t.Errorf("len(job.Attempts) = %d, want 1", len(job.Attempts))
	}
	if !job.Cahier {
		t.Error("job.Cahier = false, want true")
	}

	if _, ok := game.ResumeRunSession(store); ok {
		t.Error("ResumeRunSession after Finish: ok = true, want false (nothing left to resume)")
	}
}

var errBoom = errors.New("boom")

// failingStore fails every Save, the way a full localStorage does, so
// RecordAttempt's error path is exercised without a browser.
type failingStore struct{ *fakeStore }

func (f failingStore) Save(string, []byte) error { return errBoom }

func TestRecordAttemptReportsAStorageFailureRatherThanLosingItSilently(t *testing.T) {
	store := failingStore{newFakeStore()}
	session := game.NewRunSession(store, engine.Run{ID: "run-1"})

	err := session.RecordAttempt(engine.Attempt{WordID: "chat", Correct: true})
	if !errors.Is(err, errBoom) {
		t.Errorf("RecordAttempt error = %v, want it to wrap %v", err, errBoom)
	}
}
