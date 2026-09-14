package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
	"github.com/oioio-space/encre/server/store"
)

// seedChild creates a parent and a child under it, returning the child's ID.
func seedChild(t *testing.T, db *store.Store, id string) string {
	t.Helper()
	seedParent(t, db, id+"-parent")
	c := &store.Child{ID: id, ParentID: id + "-parent", Pseudo: id, CreatedAt: time.Now()}
	if err := db.CreateChild(t.Context(), c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}
	return id
}

func TestCreateAndFetchList(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{
		ID:        "list1",
		ChildID:   childID,
		Label:     "Semaine 1",
		ShareCode: "ABC123",
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}

	got, err := db.ListByID(ctx, "list1")
	if err != nil {
		t.Fatalf("ListByID() error = %v", err)
	}
	if got.Label != "Semaine 1" || got.Validated {
		t.Errorf("ListByID() = %+v, want Label=Semaine 1, Validated=false", got)
	}
}

func TestListByIDNotFound(t *testing.T) {
	db := openTestStore(t)
	if _, err := db.ListByID(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ListByID() error = %v, want ErrNotFound", err)
	}
}

func TestValidateList(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "Semaine 1", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}

	if err := db.ValidateList(ctx, "list1"); err != nil {
		t.Fatalf("ValidateList() error = %v", err)
	}

	got, err := db.ListByID(ctx, "list1")
	if err != nil {
		t.Fatalf("ListByID() error = %v", err)
	}
	if !got.Validated {
		t.Error("ListByID().Validated = false, want true")
	}
}

func TestValidateListNotFound(t *testing.T) {
	db := openTestStore(t)
	if err := db.ValidateList(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ValidateList() error = %v, want ErrNotFound", err)
	}
}

func TestSaveItemAndItemsOfList(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "Semaine 1", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}

	item := &store.Item{
		ID:         "item1",
		ListID:     "list1",
		Kind:       engine.KindWord,
		Text:       "chat",
		Targets:    []lexique.Span{{Start: 0, End: 4}},
		Colors:     map[engine.Color]int{engine.Muettes: 1},
		Rules:      []engine.Rule{"t_muet"},
		Family:     "chaton",
		AudioPath:  "media/list1/item1.ogg",
		Source:     1,
		Confidence: 0.92,
		Enabled:    true,
	}
	if err := db.SaveItem(ctx, item); err != nil {
		t.Fatalf("SaveItem() create error = %v", err)
	}

	item.Confidence = 0.5
	item.Enabled = false
	if err := db.SaveItem(ctx, item); err != nil {
		t.Fatalf("SaveItem() update error = %v", err)
	}

	items, err := db.ItemsOfList(ctx, "list1")
	if err != nil {
		t.Fatalf("ItemsOfList() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	got := items[0]
	if got.Confidence != 0.5 || got.Enabled {
		t.Errorf("saved item = %+v, want Confidence=0.5, Enabled=false", got)
	}
	if len(got.Targets) != 1 || got.Targets[0] != (lexique.Span{Start: 0, End: 4}) {
		t.Errorf("Targets = %+v, want [{0 4}]", got.Targets)
	}
	if got.Colors[engine.Muettes] != 1 {
		t.Errorf("Colors[Muettes] = %d, want 1", got.Colors[engine.Muettes])
	}
	if len(got.Rules) != 1 || got.Rules[0] != "t_muet" {
		t.Errorf("Rules = %+v, want [t_muet]", got.Rules)
	}
}

func TestCreateListRejectsUnknownChild(t *testing.T) {
	db := openTestStore(t)
	l := &store.WordList{ID: "list1", ChildID: "ghost", Label: "x", CreatedAt: time.Now()}
	if err := db.CreateList(t.Context(), l); err == nil {
		t.Error("CreateList() with unknown child: want error, got nil")
	}
}

func TestCreateListDuplicateIDFails(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "x", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	if err := db.CreateList(ctx, l); err == nil {
		t.Error("CreateList() with duplicate ID: want error, got nil")
	}
}

func TestListByIDWithDueDate(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	due := time.Unix(1_700_100_000, 0).UTC()
	l := &store.WordList{ID: "list1", ChildID: childID, Label: "x", DueDate: due, CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}

	got, err := db.ListByID(ctx, "list1")
	if err != nil {
		t.Fatalf("ListByID() error = %v", err)
	}
	if !got.DueDate.Equal(due) {
		t.Errorf("DueDate = %v, want %v", got.DueDate, due)
	}
}

func TestValidateListDuplicateIsIdempotent(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "x", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	if err := db.ValidateList(ctx, "list1"); err != nil {
		t.Fatalf("first ValidateList() error = %v", err)
	}
	if err := db.ValidateList(ctx, "list1"); err != nil {
		t.Errorf("second ValidateList() error = %v, want nil", err)
	}
}

func TestItemsOfListEmpty(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	l := &store.WordList{ID: "list1", ChildID: childID, Label: "x", CreatedAt: time.Now()}
	if err := db.CreateList(ctx, l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}

	items, err := db.ItemsOfList(ctx, "list1")
	if err != nil {
		t.Fatalf("ItemsOfList() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}

// TestItemsOfListCorruptedJSONFails checks that a *_json column that somehow
// stopped being valid JSON surfaces as a clear error from ItemsOfList rather
// than a panic or silently wrong data.
func TestItemsOfListCorruptedJSONFails(t *testing.T) {
	columns := []string{"targets_json", "colors_json", "rules_json"}
	for _, column := range columns {
		t.Run(column, func(t *testing.T) {
			db := openTestStore(t)
			ctx := t.Context()
			childID := seedChild(t, db, "child1")

			l := &store.WordList{ID: "list1", ChildID: childID, Label: "x", CreatedAt: time.Now()}
			if err := db.CreateList(ctx, l); err != nil {
				t.Fatalf("CreateList() error = %v", err)
			}
			item := &store.Item{ID: "item1", ListID: "list1", Kind: engine.KindWord, Text: "chat"}
			if err := db.SaveItem(ctx, item); err != nil {
				t.Fatalf("SaveItem() error = %v", err)
			}

			_, err := db.DB().ExecContext(ctx,
				`UPDATE items SET `+column+` = 'not json' WHERE id = ?`, "item1")
			if err != nil {
				t.Fatalf("corrupting %s: %v", column, err)
			}

			if _, err := db.ItemsOfList(ctx, "list1"); err == nil {
				t.Errorf("ItemsOfList() with corrupted %s: want error, got nil", column)
			}
		})
	}
}

func TestCreateListFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	l := &store.WordList{ID: "list1", ChildID: "child1", Label: "x", CreatedAt: time.Now()}
	if err := db.CreateList(t.Context(), l); err == nil {
		t.Error("CreateList() on a closed database: want error, got nil")
	}
}

func TestListByIDFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.ListByID(t.Context(), "list1"); err == nil {
		t.Error("ListByID() on a closed database: want error, got nil")
	}
}

func TestValidateListFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.ValidateList(t.Context(), "list1"); err == nil {
		t.Error("ValidateList() on a closed database: want error, got nil")
	}
}

func TestSaveItemFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	item := &store.Item{ID: "item1", ListID: "list1", Kind: engine.KindWord, Text: "chat"}
	if err := db.SaveItem(t.Context(), item); err == nil {
		t.Error("SaveItem() on a closed database: want error, got nil")
	}
}

func TestItemsOfListFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.ItemsOfList(t.Context(), "list1"); err == nil {
		t.Error("ItemsOfList() on a closed database: want error, got nil")
	}
}
