package engine_test

import (
	"math"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/oioio-space/encre/engine"
	"pgregory.net/rapid"
)

// The generators below build the fixtures the properties are checked
// against. They live together because every property in this file draws from
// the same small vocabulary of words, states, Talismans and conditions.

// genTraps draws a random trap count per Couleur, many of them left at zero,
// so a trapless word is exercised as often as a heavily trapped one.
func genTraps(t *rapid.T) map[engine.Color]int {
	traps := map[engine.Color]int{}
	for _, c := range engine.Colors() {
		if n := rapid.IntRange(0, 4).Draw(t, "traps-"+c.String()); n > 0 {
			traps[c] = n
		}
	}
	return traps
}

// genWord draws a word with a random letter count and a random spread of
// traps.
func genWord(t *rapid.T) engine.Word {
	return engine.Word{
		ID:      rapid.StringN(1, 8, -1).Draw(t, "word-id"),
		Letters: rapid.IntRange(0, 20).Draw(t, "letters"),
		Traps:   genTraps(t),
	}
}

// genWordState draws a word's state, including the combinations Gold,
// Tarnished and Cursed do not usually occur together in — the properties
// below are what would catch it if the rules code ever let two of them
// coexist.
func genWordState(t *rapid.T) engine.WordState {
	return engine.WordState{
		Mastery:   rapid.Float64Range(0, 1).Draw(t, "mastery"),
		Gold:      rapid.Bool().Draw(t, "gold"),
		Tarnished: rapid.Bool().Draw(t, "tarnished"),
		Cursed:    rapid.Bool().Draw(t, "cursed"),
		Seen:      rapid.Bool().Draw(t, "seen"),
		ConsecOK:  rapid.IntRange(0, 10).Draw(t, "consec-ok"),
		Fails:     rapid.IntRange(0, 20).Draw(t, "fails"),
	}
}

// allTalismans lists every TalismanID, V1 and V2, so genTalismans can draw
// from the whole set rather than just the eight of V1.
var allTalismans = []engine.TalismanID{
	engine.Perroquet, engine.Chronometre, engine.Jumeau, engine.Loupe, engine.Gomme,
	engine.Collectionneur, engine.Aimant, engine.Sourd, engine.Fantome, engine.Horloge,
	engine.Bibliothecaire, engine.Colosse, engine.Couronne, engine.Meute, engine.Miroir,
	engine.Echo, engine.Phare, engine.Banquier, engine.Alchimiste, engine.Tambour,
}

// genTalismans draws a random subset of the Talismans a run might carry.
func genTalismans(t *rapid.T) engine.Talismans {
	owned := engine.Talismans{}
	for i, tal := range allTalismans {
		if rapid.Bool().Draw(t, "own-"+strconv.Itoa(i)) {
			owned[tal] = true
		}
	}
	return owned
}

// genLevels draws a level 1 through 10 for every Couleur.
func genLevels(t *rapid.T) map[engine.Color]int {
	levels := map[engine.Color]int{}
	for _, c := range engine.Colors() {
		levels[c] = rapid.IntRange(1, 10).Draw(t, "level-"+c.String())
	}
	return levels
}

// genCtx draws a condition an attempt might be scored or estimated under.
func genCtx(t *rapid.T) engine.Ctx {
	return engine.Ctx{
		Levels:     genLevels(t),
		Fast:       rapid.Bool().Draw(t, "fast"),
		Listens:    rapid.IntRange(1, 2).Draw(t, "listens"),
		Sentence:   rapid.Bool().Draw(t, "sentence"),
		Blind:      rapid.Bool().Draw(t, "blind"),
		Boss:       rapid.SampledFrom([]engine.Boss{engine.NoBoss, engine.Chuchoteur, engine.VoleurDAccents, engine.Brouillon, engine.Presse}).Draw(t, "boss"),
		GoldPlayed: rapid.IntRange(0, 10).Draw(t, "gold-played"),
	}
}

// genAttempt draws an attempt, correct or not.
func genAttempt(t *rapid.T, correct bool) engine.Attempt {
	return engine.Attempt{
		Manche:  rapid.IntRange(0, 2).Draw(t, "manche"),
		Blind:   rapid.Bool().Draw(t, "attempt-blind"),
		Copy:    rapid.Bool().Draw(t, "copy"),
		Correct: correct,
		Millis:  rapid.IntRange(0, 20000).Draw(t, "millis"),
	}
}

// TestTargetsNeverDependOnTheTalismansProperty is the property ENCRE_04 §4
// asks for by name. Talismans reach the rest of the rules through Child, and
// the only Talisman-shaped field Child carries is Unlocked — the shop's
// record of what a child owns to equip. Two children who differ only in
// that field must be handed the very same targets, or buying a Talisman
// would quietly raise the bar the shop is supposed to let a child clear.
func TestTargetsNeverDependOnTheTalismansProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := newChild()
		c.Rank = rapid.IntRange(0, 5).Draw(t, "rank")
		n := rapid.IntRange(0, 8).Draw(t, "n-words")
		var week []engine.Word
		states := map[string]*engine.WordState{}
		for i := range n {
			w := genWord(t)
			w.ID = rapid.StringN(1, 4, -1).Draw(t, "wid") + string(rune('a'+i))
			week = append(week, w)
			st := genWordState(t)
			states[w.ID] = &st
		}
		deck := engine.Deck{Week: week}
		cfg := engine.DefaultConfig()

		c.Unlocked = nil
		bare := engine.Targets(c, deck, states, cfg)

		c.Unlocked = make([]engine.TalismanID, rapid.IntRange(0, 20).Draw(t, "n-unlocked"))
		for i := range c.Unlocked {
			c.Unlocked[i] = rapid.SampledFrom(allTalismans).Draw(t, "unlocked")
		}
		carrying := engine.Targets(c, deck, states, cfg)

		if bare != carrying {
			t.Fatalf("Targets = %v with no Talismans, %v carrying %v; want them equal", bare, carrying, c.Unlocked)
		}
	})
}

// TestScoreIsBitForBitDeterministic guards against the exact bug that once
// hit this code: a chips total accumulated by ranging over a map, whose
// iteration order Go randomises on every run, made two calls with identical
// arguments disagree. Score walks Colors() in a fixed order instead, and this
// property calls it fifty times over the same map-shaped Word.Traps and
// Ctx.Levels — whose range order Go reshuffles on every iteration even for a
// single map value — and demands the two floats returned agree down to the
// bit each time.
func TestScoreIsBitForBitDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := genAttempt(t, true)
		w := genWord(t)
		st := genWordState(t)
		owned := genTalismans(t)
		combo := rapid.Float64Range(0, 50).Draw(t, "combo")
		ctx := genCtx(t)
		cfg := engine.DefaultConfig()

		wantChips, wantMult := engine.Score(a, w, &st, owned, combo, ctx, cfg)
		for i := range 50 {
			gotChips, gotMult := engine.Score(a, w, &st, owned, combo, ctx, cfg)
			if math.Float64bits(gotChips) != math.Float64bits(wantChips) ||
				math.Float64bits(gotMult) != math.Float64bits(wantMult) {
				t.Fatalf("Score call %d = (%v, %v), want the first call's (%v, %v) bit for bit",
					i, gotChips, gotMult, wantChips, wantMult)
			}
		}
	})
}

// TestAWrongAttemptScoresNothingProperty is ENCRE_04 §4: a wrong answer
// breaks the combo, so no Talisman, Couleur or state of the word may leave it
// anything to score.
func TestAWrongAttemptScoresNothingProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := genAttempt(t, false)
		w := genWord(t)
		st := genWordState(t)
		owned := genTalismans(t)
		combo := rapid.Float64Range(0, 50).Draw(t, "combo")
		ctx := genCtx(t)

		chips, mult := engine.Score(a, w, &st, owned, combo, ctx, engine.DefaultConfig())
		if chips != 0 || mult != 0 {
			t.Fatalf("Score of a wrong answer = (%v, %v), want (0, 0)", chips, mult)
		}
	})
}

// TestTheMultiplierNeverFallsAsTheComboRises checks that, everything else
// held fixed, a longer combo never scores less: mult is combo plus the
// Talismans' bonus, and the bonus does not depend on the combo, so growing
// the combo alone can only raise or hold the multiplier — never lower it.
func TestTheMultiplierNeverFallsAsTheComboRises(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := genAttempt(t, true)
		w := genWord(t)
		st := genWordState(t)
		owned := genTalismans(t)
		ctx := genCtx(t)
		cfg := engine.DefaultConfig()

		low := rapid.Float64Range(0, 25).Draw(t, "low-combo")
		high := low + rapid.Float64Range(0, 25).Draw(t, "combo-delta")

		_, multLow := engine.Score(a, w, &st, owned, low, ctx, cfg)
		_, multHigh := engine.Score(a, w, &st, owned, high, ctx, cfg)
		if multHigh < multLow {
			t.Fatalf("mult at combo %v = %v, at combo %v = %v; want the higher combo never to score less",
				low, multLow, high, multHigh)
		}
	})
}

// TestPHatStaysWithinZeroAndOne checks the estimate ENCRE_03 §7 defines never
// leaves the range the card's dots and the target's arithmetic both assume,
// for any child, word and condition PHat can be asked about.
func TestPHatStaysWithinZeroAndOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := newChild()
		c.Skill = rapid.Float64Range(-2, 2).Draw(t, "skill")
		c.Level = genLevels(t)
		for _, col := range engine.Colors() {
			c.Aff[col] = rapid.Float64Range(-1, 1).Draw(t, "aff-"+col.String())
		}
		w := genWord(t)
		st := genWordState(t)
		ctx := genCtx(t)

		p := engine.PHat(c, w, &st, ctx)
		if p < 0 || p > 1 {
			t.Fatalf("PHat = %v, want it within [0, 1]", p)
		}
	})
}

// runAttempt is one attempt drawn for the Replay/Apply properties: always on
// the single word "mot", so every attempt names a word the deck actually
// carries.
func genRunAttempt(t *rapid.T) engine.Attempt {
	return engine.Attempt{
		WordID:  "mot",
		Manche:  rapid.IntRange(0, 2).Draw(t, "run-manche"),
		Blind:   rapid.Bool().Draw(t, "run-blind"),
		Correct: rapid.Bool().Draw(t, "run-correct"),
	}
}

// genRun draws a run over a single-word deck, with no Revanche: the point of
// the Replay/Apply properties below is reproducibility and idempotency, not
// the Revanche's own accept/reject rules, which score_test.go and
// replay_test.go already cover by example.
func genRun(t *rapid.T) engine.Run {
	n := rapid.IntRange(0, 12).Draw(t, "n-attempts")
	attempts := make([]engine.Attempt, n)
	for i := range attempts {
		attempts[i] = genRunAttempt(t)
	}
	return engine.Run{
		ID:   "run",
		Deck: engine.Deck{Week: words("mot"), Seed: rapid.Uint64().Draw(t, "seed")},
		Targets: [3]float64{
			rapid.Float64Range(0, 20).Draw(t, "target-0"),
			rapid.Float64Range(0, 20).Draw(t, "target-1"),
			rapid.Float64Range(0, 20).Draw(t, "target-2"),
		},
		Levels:   genLevels(t),
		Attempts: attempts,
	}
}

// TestReplayIsReproducible is ENCRE_04 §1: the server's recomputation is the
// one that counts, so replaying the same run must never disagree with
// itself, field by field, across many draws of a run's attempts and targets.
func TestReplayIsReproducible(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		run := genRun(t)
		cfg := engine.DefaultConfig()

		first, err := engine.Replay(run, nil, cfg)
		if err != nil {
			t.Fatalf("Replay: %v", err)
		}
		for i := range 10 {
			again, err := engine.Replay(run, nil, cfg)
			if err != nil {
				t.Fatalf("Replay on pass %d: %v", i, err)
			}
			if again.Scores != first.Scores || again.FailedAt != first.FailedAt ||
				again.Won != first.Won || again.Money != first.Money ||
				again.CahierBonus != first.CahierBonus || !slices.Equal(again.Events, first.Events) {
				t.Fatalf("replay %d disagreed: %+v then %+v", i, first, again)
			}
		}
	})
}

// snapshotChild copies the fields Apply can change, so
// TestApplyIsIdempotentProperty can tell a second Apply changed nothing
// without reaching into every field by hand.
type childSnapshot struct {
	kindness                      float64
	rank, bestRank                int
	bossWinsAtRank, weeksAtRank   int
	bossFailStreak, bossWinsTotal int
}

func snapshotChild(c *engine.Child) childSnapshot {
	return childSnapshot{
		kindness: c.Kindness, rank: c.Rank, bestRank: c.BestRank,
		bossWinsAtRank: c.BossWinsAtRank, weeksAtRank: c.WeeksAtRank,
		bossFailStreak: c.BossFailStreak, bossWinsTotal: c.BossWinsTotal,
	}
}

// TestApplyIsIdempotentProperty is ENCRE_04 §4 by name: a run already
// recorded must leave the child and the word exactly where the first Apply
// left them, across many random runs — this is the transition a resent run
// from a phone that lost the network relies on.
func TestApplyIsIdempotentProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		run := genRun(t)
		cfg := engine.DefaultConfig()
		c := newChild()
		states := map[string]*engine.WordState{}

		out, err := engine.Replay(run, states, cfg)
		if err != nil {
			t.Fatalf("Replay: %v", err)
		}
		if _, err := engine.Apply(c, run, out, states, 0, 0, cfg); err != nil {
			t.Fatalf("first Apply: %v", err)
		}
		before := snapshotChild(c)
		// The run may hold no attempts at all, in which case "mot" was never
		// touched and states["mot"] stays nil — a valid state to compare too.
		var beforeState engine.WordState
		if st := states["mot"]; st != nil {
			beforeState = *st
		}

		events, err := engine.Apply(c, run, out, states, 0, 0, cfg)
		if err != nil {
			t.Fatalf("second Apply: %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("second Apply raised %v, want nothing", events)
		}
		if snapshotChild(c) != before {
			t.Fatalf("second Apply changed the child: %+v then %+v", before, snapshotChild(c))
		}
		var afterState engine.WordState
		if st := states["mot"]; st != nil {
			afterState = *st
		}
		if !reflect.DeepEqual(afterState, beforeState) {
			t.Fatalf("second Apply changed the word: %+v then %+v", beforeState, afterState)
		}
	})
}

// genKnownWords draws n distinct known words together with a state for each,
// so BuildDeck's properties can exercise a realistic mix of gold, seen,
// cursed and never-played words.
func genKnownWords(t *rapid.T, n int) ([]engine.Word, map[string]*engine.WordState) {
	known := make([]engine.Word, n)
	states := map[string]*engine.WordState{}
	for i := range n {
		id := "k" + rapid.StringN(1, 4, -1).Draw(t, "kid") + string(rune('a'+i))
		known[i] = engine.Word{ID: id, Text: id, Letters: 4}
		st := genWordState(t)
		states[id] = &st
	}
	return known, states
}

// TestBuildDeckIsReproducibleFromItsSeed is ENCRE_04 §4: the server rebuilds
// the deck the child actually played from Deck.Seed alone, so two calls with
// the same seed and the same inputs must draw the very same Old and Rooms.
func TestBuildDeckIsReproducibleFromItsSeed(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 10).Draw(t, "n-known")
		known, states := genKnownWords(t, n)
		c := newChild()
		c.Rank = rapid.IntRange(0, 10).Draw(t, "rank")
		seed := rapid.Uint64().Draw(t, "seed")
		week := rapid.IntRange(0, 30).Draw(t, "week")
		cfg := engine.DefaultConfig()

		first := engine.BuildDeck(c, words("neuf"), known, states, nil, int32(week), seed, cfg)
		again := engine.BuildDeck(c, words("neuf"), known, states, nil, int32(week), seed, cfg)

		if !slices.Equal(ids(first.Old), ids(again.Old)) || first.Rooms != again.Rooms || first.Boss != again.Boss {
			t.Fatalf("the same seed %d built a different deck: Old %v/%v Rooms %v/%v",
				seed, ids(first.Old), ids(again.Old), first.Rooms, again.Rooms)
		}
	})
}

// TestBuildDeckGardeIsCappedContainsOnlyTheRequestedWordsAndHasNoDuplicates
// checks the three shapes of ENCRE_04 §4 the Garde must always have: never
// past five slots, never a word beyond what was asked for, never the same
// word twice.
func TestBuildDeckGardeIsCappedContainsOnlyTheRequestedWordsAndHasNoDuplicates(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 12).Draw(t, "n-known")
		known, states := genKnownWords(t, n)
		for _, w := range known {
			// Every candidate is gold and seen, so the Garde has as many
			// choices as possible to pick — and to overflow — from.
			states[w.ID].Gold, states[w.ID].Seen = true, true
		}
		garde := make([]string, 0, len(known))
		for _, w := range known {
			if rapid.Bool().Draw(t, "in-garde-"+w.ID) {
				garde = append(garde, w.ID)
			}
		}
		c := newChild()
		c.Rank = rapid.IntRange(0, 10).Draw(t, "rank")
		cfg := engine.DefaultConfig()

		d := engine.BuildDeck(c, words("neuf"), known, states, garde, 3, rapid.Uint64().Draw(t, "seed"), cfg)

		if len(d.Garde) > 5 {
			t.Fatalf("Garde has %d slots, want at most 5", len(d.Garde))
		}
		seen := map[string]bool{}
		for _, w := range d.Garde {
			if !slices.Contains(garde, w.ID) {
				t.Fatalf("Garde contains %q, which was never asked for; asked for %v", w.ID, garde)
			}
			if seen[w.ID] {
				t.Fatalf("Garde contains %q twice", w.ID)
			}
			seen[w.ID] = true
		}
	})
}

// TestRankStaysBoundedAndBestRankNeverFallsBehind drives a long random
// sequence of boss wins and losses and checks, after every one of them, the
// two invariants a rank can never break: it stays inside Blanc through
// Diamant, and BestRank — what furnishes the atelier — never falls below the
// rank actually held nor below what it once was.
func TestRankStaysBoundedAndBestRankNeverFallsBehind(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := newChild()
		cfg := engine.DefaultConfig()
		week := int32(0)
		bestSeen := 0

		steps := rapid.IntRange(1, 60).Draw(t, "steps")
		for i := range steps {
			week += int32(rapid.IntRange(0, 3).Draw(t, "week-delta"))
			c.WeeksAtRank++

			if rapid.Bool().Draw(t, "win") {
				c.WinBoss(week, cfg)
			} else {
				c.LoseBoss(cfg)
			}

			if c.Rank < 0 || c.Rank > 5 {
				t.Fatalf("step %d: Rank = %d, want it within [0, 5]", i, c.Rank)
			}
			if c.BestRank < c.Rank {
				t.Fatalf("step %d: BestRank = %d, less than the current Rank %d", i, c.BestRank, c.Rank)
			}
			bestSeen = max(bestSeen, c.Rank)
			if c.BestRank < bestSeen {
				t.Fatalf("step %d: BestRank = %d, less than the %d once reached", i, c.BestRank, bestSeen)
			}
		}
	})
}
