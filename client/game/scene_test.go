package game_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/game"
)

func TestNewGameStartsAtItsInitialSceneAndCarriesTheJuiceGiven(t *testing.T) {
	juice := anim.DefaultJuice()
	juice.HitstopTrap = 999

	g := game.NewGame(game.Boot, juice)

	if got := g.Current(); got != game.Boot {
		t.Errorf("Current() = %v, want %v", got, game.Boot)
	}
	// Juice grew a curve type, so it is no longer comparable with ==. What the
	// test is really about is that the Game carries the timings it was handed,
	// untouched — one changed field proves that better than a whole-struct
	// comparison, and keeps proving it when new timings are added.
	if got := g.Juice.HitstopTrap; got != juice.HitstopTrap {
		t.Errorf("Juice.HitstopTrap = %v, want the value handed to NewGame, %v", got, juice.HitstopTrap)
	}
	if !reflect.DeepEqual(g.Juice, juice) {
		t.Errorf("Juice = %+v, want %+v", g.Juice, juice)
	}
}

func TestNewDirectorStartsAtItsInitialScene(t *testing.T) {
	d := game.NewDirector(game.Boot)

	if got := d.Current(); got != game.Boot {
		t.Errorf("Current() = %v, want %v", got, game.Boot)
	}
}

func TestReplayFollowsTheMapOfBriefENCRE04Section10(t *testing.T) {
	// The straight run of brief/ENCRE_04 §10, minus the loop of Salle and the
	// Boss/Revanche branch, which have their own tests below.
	steps := []game.SceneID{
		game.Atelier, game.Garde, game.Run, game.Salle, game.Boss,
		game.Enluminure, game.Cahier, game.Recap, game.Atelier,
	}

	d := game.NewDirector(game.Boot)
	for _, next := range steps {
		if err := d.Replace(next); err != nil {
			t.Fatalf("Replace(%v) from %v: %v", next, d.Current(), err)
		}
	}
}

func TestSalleMayLeadBackToRunForTheSecondManche(t *testing.T) {
	d := game.NewDirector(game.Boot)
	for _, next := range []game.SceneID{game.Atelier, game.Garde, game.Run, game.Salle, game.Run, game.Salle, game.Boss} {
		if err := d.Replace(next); err != nil {
			t.Fatalf("Replace(%v) from %v: %v", next, d.Current(), err)
		}
	}
}

func TestReplaceRefusesAnIllegalTransition(t *testing.T) {
	d := game.NewDirector(game.Boot)

	err := d.Replace(game.Boss)
	if err == nil {
		t.Fatal("Replace(Boss) from Boot = nil error, want one")
	}
	if !errors.Is(err, game.ErrIllegalTransition) {
		t.Errorf("Replace(Boss) from Boot error = %v, want it to wrap ErrIllegalTransition", err)
	}
	if got := d.Current(); got != game.Boot {
		t.Errorf("Current() after a refused Replace = %v, want it unchanged (%v)", got, game.Boot)
	}
}

func TestAtelierOpensItsFourPanelsByPush(t *testing.T) {
	for _, panel := range []game.SceneID{game.Bestiaire, game.Fioles, game.Exploits, game.FinDeTemps} {
		d := game.NewDirector(game.Atelier)

		if err := d.Push(panel); err != nil {
			t.Fatalf("Push(%v) from Atelier: %v", panel, err)
		}
		if got := d.Current(); got != panel {
			t.Errorf("Current() = %v, want %v", got, panel)
		}

		if err := d.Pop(); err != nil {
			t.Fatalf("Pop() back to Atelier: %v", err)
		}
		if got := d.Current(); got != game.Atelier {
			t.Errorf("Current() after Pop = %v, want Atelier", got)
		}
	}
}

func TestBossOpensRevancheByPushAndReturnsByPop(t *testing.T) {
	d := game.NewDirector(game.Boss)

	if err := d.Push(game.Revanche); err != nil {
		t.Fatalf("Push(Revanche): %v", err)
	}
	if got := d.Current(); got != game.Revanche {
		t.Errorf("Current() = %v, want Revanche", got)
	}

	if err := d.Pop(); err != nil {
		t.Fatalf("Pop() back to Boss: %v", err)
	}
	if got := d.Current(); got != game.Boss {
		t.Errorf("Current() after Pop = %v, want Boss", got)
	}
}

func TestPushRefusesAnIllegalTransitionAndLeavesTheStackUnchanged(t *testing.T) {
	d := game.NewDirector(game.Boot)

	if err := d.Push(game.Cahier); err == nil {
		t.Fatal("Push(Cahier) from Boot = nil error, want one")
	} else if !errors.Is(err, game.ErrIllegalTransition) {
		t.Errorf("Push(Cahier) from Boot error = %v, want it to wrap ErrIllegalTransition", err)
	}

	if err := d.Pop(); err == nil {
		t.Fatal("Pop() on a single-scene stack = nil error, want one")
	} else if !errors.Is(err, game.ErrIllegalTransition) {
		t.Errorf("Pop() on a single-scene stack error = %v, want it to wrap ErrIllegalTransition", err)
	}
	if got := d.Current(); got != game.Boot {
		t.Errorf("Current() after refused transitions = %v, want Boot", got)
	}
}

func TestSceneIDStringNamesAnUnknownSceneByItsNumber(t *testing.T) {
	if got, want := game.SceneID(99).String(), "SceneID(99)"; got != want {
		t.Errorf("SceneID(99).String() = %q, want %q", got, want)
	}
}

func TestSceneIDStringNamesEveryScene(t *testing.T) {
	scenes := []game.SceneID{
		game.Boot, game.Atelier, game.Garde, game.Run, game.Salle, game.Boss,
		game.Revanche, game.Enluminure, game.Cahier, game.Recap,
		game.Bestiaire, game.Fioles, game.Exploits, game.FinDeTemps,
	}
	seen := map[string]bool{}
	for _, s := range scenes {
		name := s.String()
		if name == "" {
			t.Errorf("SceneID(%d).String() is empty", s)
		}
		if seen[name] {
			t.Errorf("SceneID(%d).String() = %q, already used by another scene", s, name)
		}
		seen[name] = true
	}
}

// TestALostRunReachesTheRecap covers the path ENCRE_04 §10's diagram leaves out
// and ENCRE_01 §3 spells out: « cible non atteinte = fin de run […] fin de run →
// récap ». A child who misses a target has to land somewhere warm, and the only
// somewhere is the récap. Without these edges the game can draw a win and not a
// loss, which is the half that happens more often.
func TestALostRunReachesTheRecap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from game.SceneID
	}{
		{name: "manche perdue", from: game.Run},
		{name: "boss perdu", from: game.Boss},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := game.NewDirector(tc.from)
			if err := d.Replace(game.Recap); err != nil {
				t.Fatalf("Replace(%v -> Recap) = %v, want a legal end of run", tc.from, err)
			}
			if got := d.Current(); got != game.Recap {
				t.Errorf("Current() = %v, want Recap", got)
			}
			if err := d.Replace(game.Atelier); err != nil {
				t.Errorf("Replace(Recap -> Atelier) = %v, want the way home", err)
			}
		})
	}
}

// TestTheRecapIsAlwaysReachable is the invariant behind the test above, stated
// once: from every scene a run can be playing in, the récap is one move away.
// Anything else would strand a child mid-run with no way out.
func TestTheRecapIsAlwaysReachable(t *testing.T) {
	t.Parallel()

	for _, from := range []game.SceneID{game.Run, game.Salle, game.Boss} {
		d := game.NewDirector(from)
		if err := d.Replace(game.Recap); err != nil {
			t.Errorf("from %v: Replace(Recap) = %v, want the récap always within reach", from, err)
		}
	}
}
