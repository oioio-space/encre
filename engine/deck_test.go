package engine_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
)

func words(ids ...string) []engine.Word {
	out := make([]engine.Word, 0, len(ids))
	for _, id := range ids {
		out = append(out, engine.Word{ID: id, Text: id, Letters: len(id)})
	}
	return out
}

func ids(ws []engine.Word) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.ID)
	}
	return out
}

func TestTheDeckCarriesTheWeekTheGardeTheOldAndTheCursed(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()
	week := words("un", "deux", "trois")
	known := words("or1", "or2", "vieux1", "vieux2", "vieux3", "vieux4", "vieux5", "maudit")
	states := map[string]*engine.WordState{
		"or1": {Gold: true}, "or2": {Gold: true},
		"vieux1": {Seen: true}, "vieux2": {Seen: true}, "vieux3": {Seen: true},
		"vieux4": {Seen: true}, "vieux5": {Seen: true},
		"maudit": {Seen: true, Cursed: true},
	}

	d := engine.BuildDeck(c, week, known, states, []string{"or1", "or2"}, 3, uint64(1), cfg)

	if got := ids(d.Week); !slices.Equal(got, []string{"un", "deux", "trois"}) {
		t.Errorf("Deck.Week = %v, want the week's words", got)
	}
	if got := ids(d.Garde); !slices.Equal(got, []string{"or1", "or2"}) {
		t.Errorf("Deck.Garde = %v, want the gold words asked for", got)
	}
	if got := len(d.Old); got != cfg.RecycleOld {
		t.Errorf("Deck.Old has %d words, want %d", got, cfg.RecycleOld)
	}
	if got := ids(d.Cursed); !slices.Equal(got, []string{"maudit"}) {
		t.Errorf("Deck.Cursed = %v, want the cursed word", got)
	}
}

func TestTheOldWordsBroughtBackAreNeverGoldOnes(t *testing.T) {
	// Gold is the word already remembered; spending a slot on it would take the
	// place of one that still needs the practice.
	cfg := engine.DefaultConfig()
	known := words("or1", "or2", "or3", "or4", "or5", "simple")
	states := map[string]*engine.WordState{"simple": {Seen: true}}
	for _, id := range []string{"or1", "or2", "or3", "or4", "or5"} {
		states[id] = &engine.WordState{Seen: true, Gold: true}
	}

	d := engine.BuildDeck(newChild(), words("neuf"), known, states, nil, 3, uint64(1), cfg)

	for _, w := range d.Old {
		if states[w.ID].Gold {
			t.Errorf("Deck.Old brought back the gold word %q", w.ID)
		}
	}
}

func TestTheGardeIsCappedByTheRank(t *testing.T) {
	// Three slots, one more per two ranks, never more than five (ENCRE_04 §4).
	cfg := engine.DefaultConfig()
	all := []string{"a", "b", "c", "d", "e", "f", "g"}
	known := words(all...)
	states := map[string]*engine.WordState{}
	for _, id := range all {
		states[id] = &engine.WordState{Seen: true, Gold: true}
	}

	for _, tt := range []struct{ rank, want int }{{0, 3}, {2, 4}, {4, 5}, {5, 5}} {
		c := newChild()
		c.Rank = tt.rank
		d := engine.BuildDeck(c, words("neuf"), known, states, all, 3, uint64(1), cfg)
		if got := len(d.Garde); got != tt.want {
			t.Errorf("Garde at rank %d holds %d, want %d", tt.rank, got, tt.want)
		}
	}
}

func TestATarnishedWordComesFirstInTheGarde(t *testing.T) {
	// ENCRE_01 puts the tarnished at the head of what is offered: it is the
	// word the child has stopped meeting, and the one worth pulling back.
	cfg := engine.DefaultConfig()
	all := []string{"a", "b", "terni"}
	known := words(all...)
	states := map[string]*engine.WordState{
		"a": {Seen: true, Gold: true}, "b": {Seen: true, Gold: true},
		"terni": {Seen: true, Gold: true, Tarnished: true},
	}

	d := engine.BuildDeck(newChild(), words("neuf"), known, states, all, 3, uint64(1), cfg)

	if len(d.Garde) == 0 || d.Garde[0].ID != "terni" {
		t.Errorf("Garde = %v, want the tarnished word at its head", ids(d.Garde))
	}
}

func TestTheSameSeedBuildsTheSameDeck(t *testing.T) {
	// ENCRE_04 §4: Replay is deterministic from Deck.Seed, so the server can
	// rebuild what the child played and score it again.
	cfg := engine.DefaultConfig()
	known := words("a", "b", "c", "d", "e", "f", "g", "h")
	states := map[string]*engine.WordState{}
	for _, w := range known {
		states[w.ID] = &engine.WordState{Seen: true}
	}

	first := engine.BuildDeck(newChild(), words("neuf"), known, states, nil, 3, 42, cfg)
	// Rebuilt the way the server will: from the seed the deck carries.
	again := engine.BuildDeck(newChild(), words("neuf"), known, states, nil, 3, first.Seed, cfg)

	if !slices.Equal(ids(first.Old), ids(again.Old)) {
		t.Errorf("the same seed gave %v then %v", ids(first.Old), ids(again.Old))
	}
	if first.Seed == 0 {
		t.Error("Deck.Seed is zero, so the run cannot be replayed")
	}
}

func TestEveryTransitionOffersTwoRoomsAndOneIsAlwaysTheÉchoppe(t *testing.T) {
	// ENCRE_01: the shop is never the thing a child gambles away, so it is on
	// the table at both transitions.
	cfg := engine.DefaultConfig()

	for seed := range uint64(50) {
		d := engine.BuildDeck(newChild(), words("neuf"), nil, nil, nil, 3, seed, cfg)
		for i, pair := range d.Rooms {
			if pair[0] == pair[1] {
				t.Fatalf("transition %d offers %q twice", i, pair[0])
			}
			if pair[0] != engine.Echoppe && pair[1] != engine.Echoppe {
				t.Fatalf("transition %d offers %v, want the Échoppe among them", i, pair)
			}
		}
	}
}

func TestDrawSpreadsTheWeekAcrossTheThreeManchesWithoutRepeatOrLoss(t *testing.T) {
	// The week's list is pedagogy, not something the sort gets to leave out:
	// a teacher's list is followed, not optimised around.
	cfg := engine.DefaultConfig()
	cfg.WordsPerManche = 10 // wide enough that every week word fits
	d := engine.Deck{Week: words("un", "deux", "trois")}
	ctx := engine.Ctx{Listens: 2, Boss: engine.NoBoss}

	var got []string
	for manche := range 3 {
		got = append(got, ids(engine.Draw(newChild(), d, nil, manche, ctx, cfg))...)
	}

	want := []string{"un", "deux", "trois"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("across the three manches, Draw gave %v, want exactly %v once each", got, want)
	}
}

func TestDrawNeverExcludesAMaudicteButNeverForcesItEither(t *testing.T) {
	// A first version seated every maudite on every manche: a child whose
	// maudites sit near 40% correct never strung together the consecutive
	// successes Record asks for to lift the curse, so the count only grew
	// (measured against the real engine building encre-00q.3). Ranked
	// alongside the Garde instead, a maudite is played when it is at a
	// chance worth playing, not on every single manche.
	cfg := engine.DefaultConfig()
	cfg.WordsPerManche = 1
	proche := engine.Word{ID: "proche", Letters: 5}
	// Heavily trapped, so its PHat lands well under the target however it is
	// mastered — a single-slot manche has no reason to spend it on the curse
	// while "proche" is sitting right on cfg.TargetPHat.
	maudit := engine.Word{ID: "maudit", Letters: 5, Traps: map[engine.Color]int{engine.Sosies: 4}}
	states := map[string]*engine.WordState{
		"proche": {Mastery: 0, Seen: true},
		"maudit": {Mastery: 0, Cursed: true},
	}
	ctx := engine.Ctx{Listens: 2, Boss: engine.NoBoss}
	// Skill 0.85, no traps, no mastery: PHat("proche") lands exactly on
	// cfg.TargetPHat's default of 0.85.
	c := &engine.Child{Skill: 0.85}

	d := engine.Deck{Garde: []engine.Word{proche}, Cursed: []engine.Word{maudit}}
	if got := ids(engine.Draw(c, d, states, 0, ctx, cfg)); slices.Contains(got, "maudit") {
		t.Errorf("Draw = %v, want the maudite left out in favour of the word nearer the target", got)
	}

	// But it is never excluded outright: without a better match to compete
	// against, the single candidate left is the one that wins the slot.
	if got := ids(engine.Draw(c, engine.Deck{Cursed: []engine.Word{maudit}}, states, 0, ctx, cfg)); !slices.Contains(got, "maudit") {
		t.Errorf("Draw = %v, want the maudite picked when nothing else is offered", got)
	}
}

func TestDrawNeverDropsAMandatoryWordForABetterMatch(t *testing.T) {
	// The sort only touches the Garde and the Old — the rest of what fills a
	// manche once the mandatory words are seated.
	cfg := engine.DefaultConfig()
	cfg.WordsPerManche = 2
	d := engine.Deck{
		Week:  words("neuf"),
		Garde: words("or"),
	}
	states := map[string]*engine.WordState{"or": {Mastery: 1}}
	ctx := engine.Ctx{Listens: 2, Boss: engine.NoBoss}

	got := ids(engine.Draw(newChild(), d, states, 0, ctx, cfg))

	if !slices.Contains(got, "neuf") {
		t.Errorf("Draw(manche 0) = %v, want the week's word \"neuf\" kept whatever its PHat", got)
	}
}

func TestDrawEvaluatesPHatUnderTheManchesRealContext(t *testing.T) {
	// The trap measured against the real engine: ranking the pool outside the
	// boss context it will actually be played under sent the boss failure
	// rate to 40.6% (encre-00q.3). A word near the target at an ordinary
	// manche can be far from it once the Brouillon is the one asking.
	cfg := engine.DefaultConfig()
	cfg.WordsPerManche = 1
	c := &engine.Child{Skill: 0.85}
	d := engine.Deck{Garde: words("frais", "acquis")}
	states := map[string]*engine.WordState{
		// PHat("frais", NoBoss) lands exactly on the 0.85 target, and only the
		// Brouillon's penalty — scaled by 1-Mastery — moves it.
		"frais": {Mastery: 0},
		// Mastery 1 makes PHat 0.93 under any Boss: the Brouillon's own term is
		// scaled by 1-Mastery and vanishes.
		"acquis": {Mastery: 1},
	}

	ordinary := engine.Ctx{Listens: 2, Boss: engine.NoBoss}
	if got := ids(engine.Draw(c, d, states, 0, ordinary, cfg)); !slices.Equal(got, []string{"frais"}) {
		t.Errorf("Draw under an ordinary manche = %v, want [frais], the one sitting on the target", got)
	}

	boss := engine.Ctx{Listens: 2, Boss: engine.Brouillon}
	if got := ids(engine.Draw(c, d, states, 0, boss, cfg)); !slices.Equal(got, []string{"acquis"}) {
		t.Errorf("Draw under the Brouillon = %v, want [acquis] — \"frais\" only looked right outside the boss", got)
	}
}

func TestDrawToursTheSortedPoolRatherThanHandingTheBestToOneManche(t *testing.T) {
	cfg := engine.DefaultConfig()
	cfg.WordsPerManche = 1
	d := engine.Deck{Garde: words("a", "b", "c")}
	states := map[string]*engine.WordState{
		"a": {Mastery: 0}, "b": {Mastery: 0.5}, "c": {Mastery: 1},
	}
	ctx := engine.Ctx{Listens: 2, Boss: engine.NoBoss}
	c := newChild()

	var got []string
	for manche := range 3 {
		got = append(got, ids(engine.Draw(c, d, states, manche, ctx, cfg))...)
	}

	want := []string{"a", "b", "c"}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("across the three manches the tourniquet gave %v, want each of %v exactly once", got, want)
	}
}

func TestDrawToppsUpFromTheSamePoolWhenItIsTooThinToFillAManche(t *testing.T) {
	cfg := engine.DefaultConfig()
	cfg.WordsPerManche = 3
	d := engine.Deck{Garde: words("a", "b")}
	ctx := engine.Ctx{Listens: 2, Boss: engine.NoBoss}

	got := engine.Draw(newChild(), d, nil, 0, ctx, cfg)

	if len(got) != cfg.WordsPerManche {
		t.Errorf("Draw with a pool of 2 for a manche of %d gave %d words, want the manche filled", cfg.WordsPerManche, len(got))
	}
}

func TestDrawIsDeterministic(t *testing.T) {
	cfg := engine.DefaultConfig()
	d := engine.Deck{
		Week:  words("un", "deux"),
		Garde: words("or1", "or2", "or3", "or4"),
		Old:   words("vieux1", "vieux2", "vieux3"),
	}
	states := map[string]*engine.WordState{
		"or1": {Mastery: 0.9}, "or2": {Mastery: 0.2}, "or3": {Mastery: 0.5}, "or4": {Mastery: 0.7},
		"vieux1": {Mastery: 0.6}, "vieux2": {Mastery: 0.1}, "vieux3": {Mastery: 0.4},
	}
	ctx := engine.Ctx{Listens: 2, Boss: engine.NoBoss}

	first := ids(engine.Draw(newChild(), d, states, 1, ctx, cfg))
	again := ids(engine.Draw(newChild(), d, states, 1, ctx, cfg))

	if !slices.Equal(first, again) {
		t.Errorf("Draw is not deterministic: %v then %v", first, again)
	}
}

func TestTheBossOfAWeekIsTheSameEveryTime(t *testing.T) {
	// The boss is drawn from the week, not from the seed: the child and the
	// parent should be able to know on Monday what Friday holds.
	cfg := engine.DefaultConfig()

	for week := range int32(12) {
		first := engine.BuildDeck(newChild(), words("neuf"), nil, nil, nil, week, uint64(1), cfg)
		again := engine.BuildDeck(newChild(), words("neuf"), nil, nil, nil, week, uint64(99), cfg)
		if first.Boss != again.Boss {
			t.Errorf("week %d gave boss %q then %q", week, first.Boss, again.Boss)
		}
	}

	a := engine.BuildDeck(newChild(), words("neuf"), nil, nil, nil, 0, uint64(1), cfg)
	b := engine.BuildDeck(newChild(), words("neuf"), nil, nil, nil, 1, uint64(1), cfg)
	if a.Boss == b.Boss {
		t.Errorf("two consecutive weeks share the boss %q", a.Boss)
	}
}
