package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

// TestDeleteListCascadesToItemsAndSentences is encre-6z2's DB-side half of
// "supprimé avec la liste": deleting the list must take its items and their
// sentences with it, via migration 0001's ON DELETE CASCADE foreign keys.
func TestDeleteListCascadesToItemsAndSentences(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "l", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	item := &store.Item{ID: "item1", ListID: "list1", Kind: engine.KindWord, Text: "chat"}
	if err := db.SaveItem(ctx, item); err != nil {
		t.Fatalf("SaveItem() error = %v", err)
	}
	sentence := &store.Sentence{ID: "s1", ItemID: "item1", Text: "Le chat dort.", TargetForm: "chat"}
	if err := db.SaveSentence(ctx, sentence); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}

	if err := db.DeleteList(ctx, "list1"); err != nil {
		t.Fatalf("DeleteList() error = %v", err)
	}

	if _, err := db.ListByID(ctx, "list1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ListByID() after DeleteList(): error = %v, want ErrNotFound", err)
	}
	if _, err := db.ItemByID(ctx, "item1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ItemByID() after DeleteList(): error = %v, want ErrNotFound (cascade)", err)
	}
	if _, err := db.SentenceByID(ctx, "s1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SentenceByID() after DeleteList(): error = %v, want ErrNotFound (cascade)", err)
	}
}

func TestDeleteListNotFound(t *testing.T) {
	db := openTestStore(t)
	if err := db.DeleteList(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("DeleteList() error = %v, want ErrNotFound", err)
	}
}
