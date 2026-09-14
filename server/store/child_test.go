package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

// seedParent inserts and returns a parent the caller may attach children to.
func seedParent(t *testing.T, db *store.Store, id string) *store.Parent {
	t.Helper()
	p := &store.Parent{ID: id, Email: id + "@example.com", PassHash: []byte("h"), CreatedAt: time.Now()}
	if err := db.CreateParent(t.Context(), p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}
	return p
}

// wantChild is an engine.Child exercising every field this package round-trips.
func wantChild() engine.Child {
	c := engine.Child{
		Rank:           2,
		BestRank:       3,
		Prestige:       1,
		BossWinsAtRank: 4,
		WeeksAtRank:    5,
		BossFailStreak: 1,
		Kindness:       0.75,
		Skill:          0.42,
		Level:          map[engine.Color]int{engine.Muettes: 3, engine.Accordees: 1},
		Aff:            map[engine.Color]float64{engine.Muettes: 0.1, engine.Sosies: -0.2},
		LearnRate:      0.05,
		XP:             map[engine.Color]float64{engine.Muettes: 12.5},
		NextLevelW:     map[engine.Color]int32{engine.Muettes: 9},
		LastRankW:      7,
		Base:           engine.RollingRate{Wins: 8, Total: 10},
		Unlocked:       []engine.TalismanID{engine.Perroquet},
		BossWinsTotal:  6,
	}
	return c
}

func assertChildEqual(t *testing.T, got, want engine.Child) {
	t.Helper()
	if got.Rank != want.Rank || got.BestRank != want.BestRank || got.Prestige != want.Prestige {
		t.Errorf("rank fields = %+v, want %+v", got, want)
	}
	if got.BossWinsAtRank != want.BossWinsAtRank || got.WeeksAtRank != want.WeeksAtRank ||
		got.BossFailStreak != want.BossFailStreak {
		t.Errorf("boss fields = %+v, want %+v", got, want)
	}
	if got.Kindness != want.Kindness || got.Skill != want.Skill || got.LearnRate != want.LearnRate {
		t.Errorf("scalar fields = %+v, want %+v", got, want)
	}
	if got.LastRankW != want.LastRankW {
		t.Errorf("LastRankW = %d, want %d", got.LastRankW, want.LastRankW)
	}
	if got.Base != want.Base {
		t.Errorf("Base = %+v, want %+v", got.Base, want.Base)
	}
	if got.BossWinsTotal != want.BossWinsTotal {
		t.Errorf("BossWinsTotal = %d, want %d", got.BossWinsTotal, want.BossWinsTotal)
	}
	if len(got.Level) != len(want.Level) || got.Level[engine.Muettes] != want.Level[engine.Muettes] {
		t.Errorf("Level = %+v, want %+v", got.Level, want.Level)
	}
	if len(got.Aff) != len(want.Aff) || got.Aff[engine.Sosies] != want.Aff[engine.Sosies] {
		t.Errorf("Aff = %+v, want %+v", got.Aff, want.Aff)
	}
	if len(got.XP) != len(want.XP) || got.XP[engine.Muettes] != want.XP[engine.Muettes] {
		t.Errorf("XP = %+v, want %+v", got.XP, want.XP)
	}
	if len(got.NextLevelW) != len(want.NextLevelW) || got.NextLevelW[engine.Muettes] != want.NextLevelW[engine.Muettes] {
		t.Errorf("NextLevelW = %+v, want %+v", got.NextLevelW, want.NextLevelW)
	}
	if len(got.Unlocked) != 1 || got.Unlocked[0] != engine.Perroquet {
		t.Errorf("Unlocked = %+v, want [Perroquet]", got.Unlocked)
	}
}

func TestCreateAndFetchChildRoundTripsEngineChild(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "parent1")

	c := &store.Child{
		ID:          "child1",
		ParentID:    "parent1",
		Pseudo:      "Mia",
		PatternHash: []byte("pattern"),
		Avatar:      2,
		CreatedAt:   time.Unix(1_700_000_000, 0).UTC(),
	}
	c.SetEngine(wantChild())

	if err := db.CreateChild(ctx, c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}

	got, err := db.ChildByID(ctx, "child1")
	if err != nil {
		t.Fatalf("ChildByID() error = %v", err)
	}
	if got.Pseudo != "Mia" {
		t.Errorf("Pseudo = %q, want %q", got.Pseudo, "Mia")
	}
	assertChildEqual(t, got.Engine(), wantChild())
}

func TestChildrenOfParent(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "parent1")

	for _, id := range []string{"c1", "c2"} {
		c := &store.Child{ID: id, ParentID: "parent1", Pseudo: id, CreatedAt: time.Now()}
		if err := db.CreateChild(ctx, c); err != nil {
			t.Fatalf("CreateChild(%s) error = %v", id, err)
		}
	}

	children, err := db.ChildrenOfParent(ctx, "parent1")
	if err != nil {
		t.Fatalf("ChildrenOfParent() error = %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("len(children) = %d, want 2", len(children))
	}
}

func TestSaveChildUpdatesEngineState(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "parent1")

	c := &store.Child{ID: "child1", ParentID: "parent1", Pseudo: "Mia", CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}

	c.SetEngine(wantChild())
	if err := db.SaveChild(ctx, c); err != nil {
		t.Fatalf("SaveChild() error = %v", err)
	}

	got, err := db.ChildByID(ctx, "child1")
	if err != nil {
		t.Fatalf("ChildByID() error = %v", err)
	}
	assertChildEqual(t, got.Engine(), wantChild())
}

func TestChildByIDNotFound(t *testing.T) {
	db := openTestStore(t)

	_, err := db.ChildByID(t.Context(), "nope")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ChildByID() error = %v, want ErrNotFound", err)
	}
}

func TestCreateChildRejectsUnknownParent(t *testing.T) {
	db := openTestStore(t)

	c := &store.Child{ID: "child1", ParentID: "ghost", Pseudo: "Mia", CreatedAt: time.Now()}
	if err := db.CreateChild(t.Context(), c); err == nil {
		t.Error("CreateChild() with unknown parent: want error, got nil")
	}
}

func TestDeletingParentCascadesToChildren(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "parent1")

	c := &store.Child{ID: "child1", ParentID: "parent1", Pseudo: "Mia", CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}

	if _, err := db.DB().ExecContext(ctx, `DELETE FROM parents WHERE id = ?`, "parent1"); err != nil {
		t.Fatalf("deleting parent: %v", err)
	}

	if _, err := db.ChildByID(ctx, "child1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ChildByID() after cascade delete: error = %v, want ErrNotFound", err)
	}
}

func TestCreateChildDuplicateIDFails(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "parent1")

	c := &store.Child{ID: "child1", ParentID: "parent1", Pseudo: "Mia", CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}
	if err := db.CreateChild(ctx, c); err == nil {
		t.Error("CreateChild() with duplicate ID: want error, got nil")
	}
}

func TestSaveChildNotFound(t *testing.T) {
	db := openTestStore(t)

	c := &store.Child{ID: "nope", ParentID: "ghost", Pseudo: "Mia", CreatedAt: time.Now()}
	if err := db.SaveChild(t.Context(), c); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SaveChild() on unknown child: error = %v, want ErrNotFound", err)
	}
}

func TestChildrenOfParentEmpty(t *testing.T) {
	db := openTestStore(t)
	seedParent(t, db, "parent1")

	children, err := db.ChildrenOfParent(t.Context(), "parent1")
	if err != nil {
		t.Fatalf("ChildrenOfParent() error = %v", err)
	}
	if len(children) != 0 {
		t.Errorf("len(children) = %d, want 0", len(children))
	}
}

// TestChildByIDCorruptedJSONFails checks that a *_json column that somehow
// stopped being valid JSON surfaces as a clear error from ChildByID rather
// than a panic or silently wrong data.
func TestChildByIDCorruptedJSONFails(t *testing.T) {
	columns := []string{"base_json", "level_json", "xp_json", "levelup_w_json", "unlocked_json"}
	for _, column := range columns {
		t.Run(column, func(t *testing.T) {
			db := openTestStore(t)
			ctx := t.Context()
			seedParent(t, db, "parent1")

			c := &store.Child{ID: "child1", ParentID: "parent1", Pseudo: "Mia", CreatedAt: time.Now()}
			if err := db.CreateChild(ctx, c); err != nil {
				t.Fatalf("CreateChild() error = %v", err)
			}

			_, err := db.DB().ExecContext(ctx,
				`UPDATE children SET `+column+` = 'not json' WHERE id = ?`, "child1")
			if err != nil {
				t.Fatalf("corrupting %s: %v", column, err)
			}

			if _, err := db.ChildByID(ctx, "child1"); err == nil {
				t.Errorf("ChildByID() with corrupted %s: want error, got nil", column)
			}
		})
	}
}

func TestCreateChildFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	c := &store.Child{ID: "child1", ParentID: "parent1", Pseudo: "Mia", CreatedAt: time.Now()}
	if err := db.CreateChild(t.Context(), c); err == nil {
		t.Error("CreateChild() on a closed database: want error, got nil")
	}
}

func TestChildByIDFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.ChildByID(t.Context(), "child1"); err == nil {
		t.Error("ChildByID() on a closed database: want error, got nil")
	}
}

func TestChildrenOfParentFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.ChildrenOfParent(t.Context(), "parent1"); err == nil {
		t.Error("ChildrenOfParent() on a closed database: want error, got nil")
	}
}
