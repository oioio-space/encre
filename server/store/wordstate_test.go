package store_test

import (
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

func seedItem(t *testing.T, db *store.Store, childID, itemID string) {
	t.Helper()
	l := &store.WordList{ID: itemID + "-list", ChildID: childID, Label: "l", CreatedAt: time.Now()}
	if err := db.CreateList(t.Context(), l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	item := &store.Item{ID: itemID, ListID: l.ID, Kind: engine.KindWord, Text: "chat"}
	if err := db.SaveItem(t.Context(), item); err != nil {
		t.Fatalf("SaveItem() error = %v", err)
	}
}

func TestSaveAndFetchWordStates(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	seedItem(t, db, childID, "item1")
	seedItem(t, db, childID, "item2")

	states := map[string]*engine.WordState{
		"item1": {Mastery: 0.8, SuccessDays: []int32{1, 3}, Gold: true, Shine: engine.ShineHolo},
		"item2": {Mastery: 0.1, Fails: 2, FailWeeks: []int32{5}, Cursed: true},
	}
	if err := db.SaveWordStates(ctx, childID, states); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}

	got, err := db.WordStates(ctx, childID)
	if err != nil {
		t.Fatalf("WordStates() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got["item1"].Mastery != 0.8 || !got["item1"].Gold || got["item1"].Shine != engine.ShineHolo {
		t.Errorf("got[item1] = %+v", got["item1"])
	}
	if len(got["item1"].SuccessDays) != 2 || got["item1"].SuccessDays[1] != 3 {
		t.Errorf("got[item1].SuccessDays = %+v", got["item1"].SuccessDays)
	}
	if got["item2"].Fails != 2 || !got["item2"].Cursed || len(got["item2"].FailWeeks) != 1 {
		t.Errorf("got[item2] = %+v", got["item2"])
	}
}

func TestSaveWordStatesOverwritesExisting(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	seedItem(t, db, childID, "item1")

	first := map[string]*engine.WordState{"item1": {Mastery: 0.2}}
	if err := db.SaveWordStates(ctx, childID, first); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}
	second := map[string]*engine.WordState{"item1": {Mastery: 0.9}}
	if err := db.SaveWordStates(ctx, childID, second); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}

	got, err := db.WordStates(ctx, childID)
	if err != nil {
		t.Fatalf("WordStates() error = %v", err)
	}
	if got["item1"].Mastery != 0.9 {
		t.Errorf("Mastery = %v, want 0.9", got["item1"].Mastery)
	}
}

func TestWordStatesCorruptedJSONFails(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	seedItem(t, db, childID, "item1")

	if err := db.SaveWordStates(ctx, childID, map[string]*engine.WordState{"item1": {Mastery: 0.5}}); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}
	_, err := db.DB().ExecContext(ctx,
		`UPDATE word_states SET state_json = 'not json' WHERE child_id = ? AND item_id = ?`, childID, "item1")
	if err != nil {
		t.Fatalf("corrupting state_json: %v", err)
	}

	if _, err := db.WordStates(ctx, childID); err == nil {
		t.Error("WordStates() with corrupted state_json: want error, got nil")
	}
}

func TestWordStatesEmptyForUnknownChild(t *testing.T) {
	db := openTestStore(t)

	got, err := db.WordStates(t.Context(), "nope")
	if err != nil {
		t.Fatalf("WordStates() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestWordStatesFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.WordStates(t.Context(), "child1"); err == nil {
		t.Error("WordStates() on a closed database: want error, got nil")
	}
}

func TestSaveWordStatesFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	states := map[string]*engine.WordState{"item1": {Mastery: 0.5}}
	if err := db.SaveWordStates(t.Context(), "child1", states); err == nil {
		t.Error("SaveWordStates() on a closed database: want error, got nil")
	}
}
