package engine_test

import (
	"math"
	"testing"

	"github.com/oioio-space/encre/engine"
)

func newChild() *engine.Child {
	c := &engine.Child{Skill: 0.7, Kindness: 1, Level: map[engine.Color]int{}, Aff: map[engine.Color]float64{}}
	for _, col := range engine.Colors() {
		c.Level[col] = 1
	}
	return c
}

func TestPHatRisesWithMasteryAndFallsWithTraps(t *testing.T) {
	c := newChild()
	hard := engine.Word{Letters: 6, Traps: map[engine.Color]int{engine.Sosies: 3}}

	fresh := engine.PHat(c, hard, &engine.WordState{}, engine.Ctx{Listens: 2, Boss: engine.NoBoss})
	known := engine.PHat(c, hard, &engine.WordState{Mastery: 0.9}, engine.Ctx{Listens: 2, Boss: engine.NoBoss})
	easy := engine.PHat(c, engine.Word{Letters: 6}, &engine.WordState{}, engine.Ctx{Listens: 2, Boss: engine.NoBoss})

	if !(known > fresh) {
		t.Errorf("PHat on a mastered word = %v, want more than the fresh %v", known, fresh)
	}
	if !(easy > fresh) {
		t.Errorf("PHat on a trapless word = %v, want more than the trapped %v", easy, fresh)
	}
}

func TestEachHarderConditionLowersPHat(t *testing.T) {
	// ENCRE_03 §7. Each of these is a real handicap the child feels, so none of
	// them may leave the estimate untouched — the p̂ is what the target is
	// built from, and a condition that costs nothing would make its manche free.
	c := newChild()
	w := engine.Word{Letters: 6, Traps: map[engine.Color]int{engine.Masquees: 2}}
	st := &engine.WordState{Mastery: 0.3}
	base := engine.Ctx{Listens: 2, Boss: engine.NoBoss}
	ref := engine.PHat(c, w, st, base)

	tests := []struct {
		name string
		ctx  engine.Ctx
	}{
		{"a single listen", engine.Ctx{Listens: 1, Boss: engine.NoBoss}},
		{"the word inside a sentence", engine.Ctx{Listens: 2, Sentence: true, Boss: engine.NoBoss}},
		{"played blind", engine.Ctx{Listens: 2, Blind: true, Boss: engine.NoBoss}},
		{"the Presse", engine.Ctx{Listens: 2, Boss: engine.Presse}},
		{"the Brouillon", engine.Ctx{Listens: 2, Boss: engine.Brouillon}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engine.PHat(c, w, st, tt.ctx); got >= ref {
				t.Errorf("PHat = %v, want less than %v without the handicap", got, ref)
			}
		})
	}
}

func TestPHatNeverPromisesCertaintyNorDespair(t *testing.T) {
	// The estimate drives the dots on the card. Zero would tell a child not to
	// try; one would promise a word they can still miss.
	c := newChild()
	impossible := engine.Word{Letters: 20, Traps: map[engine.Color]int{
		engine.Sosies: 9, engine.Accordees: 9, engine.Masquees: 9,
	}}
	sure := engine.Word{Letters: 2}

	if got := engine.PHat(c, impossible, &engine.WordState{}, engine.Ctx{Listens: 1, Blind: true, Boss: engine.Brouillon}); got < 0.03 {
		t.Errorf("PHat on a hopeless word = %v, want at least 0.03", got)
	}
	if got := engine.PHat(c, sure, &engine.WordState{Mastery: 1}, engine.Ctx{Listens: 2, Boss: engine.NoBoss}); got > 0.98 {
		t.Errorf("PHat on a mastered short word = %v, want at most 0.98", got)
	}
}

func TestTargetsNeverDependOnTheTalismans(t *testing.T) {
	// The property ENCRE_04 §4 asks for by name. If the Talismans a child
	// happens to carry raised the target, buying one would make the run harder
	// — the shop would punish the child for using it.
	c := newChild()
	deck := engine.Deck{Week: []engine.Word{
		{ID: "a", Letters: 5, Traps: map[engine.Color]int{engine.Muettes: 2}},
		{ID: "b", Letters: 7, Traps: map[engine.Color]int{engine.Jumelles: 1, engine.Sosies: 2}},
	}}
	states := map[string]*engine.WordState{"a": {Mastery: 0.4}, "b": {}}

	want := engine.Targets(c, deck, states, engine.DefaultConfig())

	// Targets takes no Talismans at all, which is how the property is kept:
	// there is no argument through which they could reach it. Recomputing must
	// give the same three numbers every time.
	if got := engine.Targets(c, deck, states, engine.DefaultConfig()); got != want {
		t.Errorf("Targets is not deterministic: %v then %v", want, got)
	}
}

func TestTargetsGrowWithEachMancheAndWithTheRank(t *testing.T) {
	c := newChild()
	deck := engine.Deck{Week: []engine.Word{{ID: "a", Letters: 5, Traps: map[engine.Color]int{engine.Muettes: 2}}}}
	states := map[string]*engine.WordState{"a": {}}
	cfg := engine.DefaultConfig()

	got := engine.Targets(c, deck, states, cfg)
	if !(got[0] < got[1] && got[1] < got[2]) {
		t.Errorf("targets = %v, want each manche harder than the last", got)
	}

	c.Rank = 3
	higher := engine.Targets(c, deck, states, cfg)
	if !(higher[0] > got[0]) {
		t.Errorf("target at rank 3 = %v, want more than %v at rank 0", higher[0], got[0])
	}
}

func TestKindnessLowersTheTargetsAndStopsAtItsFloor(t *testing.T) {
	// ENCRE_01: three percent off per run lost early, no further than 0.85.
	// The simulation needed it — without it the middling child never recovers
	// from a bad fortnight and leaves.
	cfg := engine.DefaultConfig()
	c := newChild()

	for range 20 {
		c.LoseEarly(cfg)
	}
	if got := c.Kindness; got != cfg.KindnessFloor {
		t.Errorf("Kindness after twenty early losses = %v, want the floor %v", got, cfg.KindnessFloor)
	}

	c.Win()
	if got := c.Kindness; got != 1 {
		t.Errorf("Kindness after a win = %v, want 1", got)
	}
}

func TestAKinderTargetIsALowerOne(t *testing.T) {
	cfg := engine.DefaultConfig()
	deck := engine.Deck{Week: []engine.Word{{ID: "a", Letters: 5, Traps: map[engine.Color]int{engine.Muettes: 2}}}}
	states := map[string]*engine.WordState{"a": {}}

	full := engine.Targets(newChild(), deck, states, cfg)

	kind := newChild()
	kind.LoseEarly(cfg)
	lowered := engine.Targets(kind, deck, states, cfg)

	if !(lowered[0] < full[0]) {
		t.Errorf("target after an early loss = %v, want less than %v", lowered[0], full[0])
	}
	if math.Abs(lowered[0]-full[0]*(1-cfg.KindnessStep)) > 1e-9 {
		t.Errorf("target = %v, want %v — exactly one step of Bienveillance", lowered[0], full[0]*(1-cfg.KindnessStep))
	}
}
