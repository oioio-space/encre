package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/content"
	"github.com/oioio-space/encre/engine"
)

func TestListensAllowed(t *testing.T) {
	if got := game.ListensAllowed(engine.Chuchoteur); got != 1 {
		t.Errorf("ListensAllowed(Chuchoteur) = %d, want 1", got)
	}
	for _, boss := range []engine.Boss{engine.NoBoss, engine.Brouillon, engine.Presse, engine.VoleurDAccents} {
		if got := game.ListensAllowed(boss); got != 2 {
			t.Errorf("ListensAllowed(%v) = %d, want 2", boss, got)
		}
	}
}

func TestBossLineArrival(t *testing.T) {
	pack := content.Embedded()

	got, ok := game.BossLine(pack, engine.Chuchoteur, game.BossArrival, false)
	if !ok {
		t.Fatal("BossLine(Chuchoteur, BossArrival, false): not found")
	}
	if got == "" {
		t.Error("BossLine(Chuchoteur, BossArrival, false) = \"\", want the arrival line")
	}
}

// TestBossLineEndPicksTheFieldNamedTheOppositeWayItReads pins content.Boss's
// inverted naming (see [content.Boss]'s own doc): Defeat is what the boss says
// when the CHILD wins, Victory is what it says when the child loses.
func TestBossLineEndPicksTheFieldNamedTheOppositeWayItReads(t *testing.T) {
	pack := content.Embedded()
	boss, ok := pack.Boss(engine.Chuchoteur)
	if !ok {
		t.Fatal("pack.Boss(Chuchoteur): not found")
	}

	got, ok := game.BossLine(pack, engine.Chuchoteur, game.BossEnd, true)
	if !ok || got != boss.Defeat {
		t.Errorf("BossLine(Chuchoteur, BossEnd, childWon=true) = %q, %v, want %q, true", got, ok, boss.Defeat)
	}

	got, ok = game.BossLine(pack, engine.Chuchoteur, game.BossEnd, false)
	if !ok || got != boss.Victory {
		t.Errorf("BossLine(Chuchoteur, BossEnd, childWon=false) = %q, %v, want %q, true", got, ok, boss.Victory)
	}
}

func TestBossLineReportsNotFoundForNoBoss(t *testing.T) {
	pack := content.Embedded()
	if _, ok := game.BossLine(pack, engine.NoBoss, game.BossArrival, false); ok {
		t.Error("BossLine(NoBoss, BossArrival, false) = ok, want not ok: an ordinary manche has no boss to speak")
	}
}
