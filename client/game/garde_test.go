package game_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

func TestGardeFanPutsTarniesFirstAndKeepsTheirRelativeOrder(t *testing.T) {
	candidates := []engine.Word{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
	}
	states := map[string]*engine.WordState{
		"b": {Tarnished: true},
		"d": {Tarnished: true},
	}

	got := game.GardeFan(candidates, states)

	var ids []string
	for _, w := range got {
		ids = append(ids, w.ID)
	}
	if want := []string{"b", "d", "a", "c"}; !slices.Equal(ids, want) {
		t.Errorf("GardeFan() = %v, want %v (ternies first, order kept within each group)", ids, want)
	}
}

func TestGardeFanWithNoTarniesLeavesOrderUntouched(t *testing.T) {
	candidates := []engine.Word{{ID: "a"}, {ID: "b"}, {ID: "c"}}

	got := game.GardeFan(candidates, nil)

	var ids []string
	for _, w := range got {
		ids = append(ids, w.ID)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(ids, want) {
		t.Errorf("GardeFan() = %v, want %v", ids, want)
	}
}

func TestGardeFanDoesNotMutateItsInput(t *testing.T) {
	candidates := []engine.Word{{ID: "a"}, {ID: "b"}}
	states := map[string]*engine.WordState{"b": {Tarnished: true}}

	game.GardeFan(candidates, states)

	if candidates[0].ID != "a" || candidates[1].ID != "b" {
		t.Errorf("GardeFan mutated its input: %v", candidates)
	}
}

func TestGardeSlotsGrowsByRankAndCapsAtFive(t *testing.T) {
	cfg := engine.DefaultConfig() // GardeSlots: 3

	tests := []struct {
		rank int
		want int
	}{
		{0, 3},
		{1, 3},
		{2, 4},
		{3, 4},
		{4, 5},
		{10, 5}, // capped
	}
	for _, tt := range tests {
		if got := game.GardeSlots(cfg, tt.rank); got != tt.want {
			t.Errorf("GardeSlots(cfg, rank=%d) = %d, want %d", tt.rank, got, tt.want)
		}
	}
}

// TestGardeSlotsMatchesWhatBuildDeckActuallyGrants cross-checks
// [game.GardeSlots] against a live [engine.BuildDeck]: this package cannot
// import engine's own unexported cap (see [game.gardeSlotsMax]'s doc), so this
// is what would notice the two drifting apart.
func TestGardeSlotsMatchesWhatBuildDeckActuallyGrants(t *testing.T) {
	cfg := engine.DefaultConfig()

	for rank := range 12 {
		gold := make([]engine.Word, 12)
		garde := make([]string, 12)
		states := map[string]*engine.WordState{}
		for i := range gold {
			id := string(rune('a' + i))
			gold[i] = engine.Word{ID: id, Letters: 3}
			garde[i] = id
			states[id] = &engine.WordState{Gold: true}
		}
		child := &engine.Child{Rank: rank}

		d := engine.BuildDeck(child, nil, gold, states, garde, 0, 1, cfg)

		if got, want := len(d.Garde), game.GardeSlots(cfg, rank); got != want {
			t.Errorf("rank %d: BuildDeck granted %d Garde slots, GardeSlots() says %d", rank, got, want)
		}
	}
}
