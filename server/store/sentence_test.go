package store_test

import (
	"errors"
	"testing"

	"github.com/oioio-space/encre/server/store"
)

func newSentence(id, itemID string) *store.Sentence {
	return &store.Sentence{ID: id, ItemID: itemID, Text: "Le ___ dort sur le lit.", TargetForm: "chat"}
}

func TestSaveAndFetchSentence(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	l := &store.WordList{ID: "list1", ChildID: childID, Label: "l"}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	seedListItem(t, db, "list1", "item1")

	s := newSentence("s1", "item1")
	if err := db.SaveSentence(ctx, s); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}

	got, err := db.SentenceByID(ctx, "s1")
	if err != nil {
		t.Fatalf("SentenceByID() error = %v", err)
	}
	if got.Text != s.Text || got.TargetForm != s.TargetForm || got.Approved {
		t.Errorf("SentenceByID() = %+v, want %+v with Approved=false", got, s)
	}
}

func TestSentenceByIDNotFound(t *testing.T) {
	db := openTestStore(t)
	if _, err := db.SentenceByID(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SentenceByID() error = %v, want ErrNotFound", err)
	}
}

func TestSaveSentenceRejectsUnknownItem(t *testing.T) {
	db := openTestStore(t)
	s := newSentence("s1", "ghost")
	if err := db.SaveSentence(t.Context(), s); err == nil {
		t.Error("SaveSentence() with unknown item: want error, got nil")
	}
}

func TestSentencesOfItem(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	l := &store.WordList{ID: "list1", ChildID: childID, Label: "l"}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	seedListItem(t, db, "list1", "item1")

	for _, id := range []string{"s1", "s2"} {
		if err := db.SaveSentence(ctx, newSentence(id, "item1")); err != nil {
			t.Fatalf("SaveSentence(%s) error = %v", id, err)
		}
	}

	got, err := db.SentencesOfItem(ctx, "item1")
	if err != nil {
		t.Fatalf("SentencesOfItem() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(SentencesOfItem()) = %d, want 2", len(got))
	}
}

func TestApproveSentence(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	l := &store.WordList{ID: "list1", ChildID: childID, Label: "l"}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	seedListItem(t, db, "list1", "item1")
	if err := db.SaveSentence(ctx, newSentence("s1", "item1")); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}

	if err := db.ApproveSentence(ctx, "s1"); err != nil {
		t.Fatalf("ApproveSentence() error = %v", err)
	}
	got, err := db.SentenceByID(ctx, "s1")
	if err != nil {
		t.Fatalf("SentenceByID() error = %v", err)
	}
	if !got.Approved {
		t.Error("Approved = false after ApproveSentence()")
	}
}

func TestApproveSentenceNotFound(t *testing.T) {
	db := openTestStore(t)
	if err := db.ApproveSentence(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ApproveSentence() error = %v, want ErrNotFound", err)
	}
}

func TestDeleteSentence(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	l := &store.WordList{ID: "list1", ChildID: childID, Label: "l"}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	seedListItem(t, db, "list1", "item1")
	if err := db.SaveSentence(ctx, newSentence("s1", "item1")); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}

	if err := db.DeleteSentence(ctx, "s1"); err != nil {
		t.Fatalf("DeleteSentence() error = %v", err)
	}
	if _, err := db.SentenceByID(ctx, "s1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SentenceByID() after delete: error = %v, want ErrNotFound", err)
	}
}

func TestDeleteSentenceNotFound(t *testing.T) {
	db := openTestStore(t)
	if err := db.DeleteSentence(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("DeleteSentence() error = %v, want ErrNotFound", err)
	}
}
