package store_test

import (
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

func seedListItem(t *testing.T, db *store.Store, listID, itemID string) {
	t.Helper()
	item := &store.Item{ID: itemID, ListID: listID, Kind: engine.KindWord, Text: "chat"}
	if err := db.SaveItem(t.Context(), item); err != nil {
		t.Fatalf("SaveItem() error = %v", err)
	}
}

func TestSaveDicteeResultAndResultsOfList(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "Semaine 1", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	seedListItem(t, db, "list1", "item1")
	seedListItem(t, db, "list1", "item2")

	entered := time.Unix(1_700_000_000, 0).UTC()
	results := []*store.DicteeResult{
		{ID: "r1", ListID: "list1", ItemID: "item1", Correct: true, EnteredAt: entered},
		{ID: "r2", ListID: "list1", ItemID: "item2", Correct: false, EnteredAt: entered},
	}
	for _, r := range results {
		if err := db.SaveDicteeResult(ctx, r); err != nil {
			t.Fatalf("SaveDicteeResult(%s) error = %v", r.ID, err)
		}
	}

	got, err := db.DicteeResultsOfList(ctx, "list1")
	if err != nil {
		t.Fatalf("DicteeResultsOfList() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(DicteeResultsOfList()) = %d, want 2", len(got))
	}

	byItem := map[string]bool{}
	for _, r := range got {
		byItem[r.ItemID] = r.Correct
		if !r.EnteredAt.Equal(entered) {
			t.Errorf("EnteredAt = %v, want %v", r.EnteredAt, entered)
		}
	}
	if !byItem["item1"] {
		t.Errorf("item1 result Correct = false, want true")
	}
	if byItem["item2"] {
		t.Errorf("item2 result Correct = true, want false")
	}
}

func TestSaveDicteeResultRejectsUnknownItem(t *testing.T) {
	db := openTestStore(t)
	r := &store.DicteeResult{ID: "r1", ListID: "list1", ItemID: "ghost", Correct: true, EnteredAt: time.Now()}
	if err := db.SaveDicteeResult(t.Context(), r); err == nil {
		t.Error("SaveDicteeResult() with unknown item: want error, got nil")
	}
}

func TestDicteeResultsOfListEmpty(t *testing.T) {
	db := openTestStore(t)
	got, err := db.DicteeResultsOfList(t.Context(), "nope")
	if err != nil {
		t.Fatalf("DicteeResultsOfList() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(DicteeResultsOfList()) = %d, want 0", len(got))
	}
}
