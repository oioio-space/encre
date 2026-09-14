package engine_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
)

// runOf builds a run over one easy word per manche, with the targets given.
func runOf(targets [3]float64, attempts ...engine.Attempt) engine.Run {
	return engine.Run{
		ID:       "r1",
		Deck:     engine.Deck{Week: words("mot"), Seed: 7},
		Targets:  targets,
		Attempts: attempts,
	}
}

func answer(manche int, correct bool) engine.Attempt {
	return engine.Attempt{WordID: "mot", Manche: manche, Correct: correct}
}

func TestReplayScoresEachMancheAgainstItsOwnTarget(t *testing.T) {
	cfg := engine.DefaultConfig()
	// "mot" is three letters at a combo of one: three chips a manche.
	out, err := engine.Replay(runOf([3]float64{3, 3, 3},
		answer(0, true), answer(1, true), answer(2, true)), nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if !out.Won {
		t.Errorf("Outcome.Won = false with every manche made; scores %v", out.Scores)
	}
	if out.FailedAt != -1 {
		t.Errorf("FailedAt = %d, want -1 on a run that was won", out.FailedAt)
	}
}

func TestAMancheShortOfItsTargetEndsTheRunThere(t *testing.T) {
	cfg := engine.DefaultConfig()

	out, err := engine.Replay(runOf([3]float64{3, 1000, 3},
		answer(0, true), answer(1, true), answer(2, true)), nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if out.Won {
		t.Error("Outcome.Won = true on a run that missed a target")
	}
	if out.FailedAt != 1 {
		t.Errorf("FailedAt = %d, want 1", out.FailedAt)
	}
}

func TestTheSameRunReplaysToTheSameOutcomeEveryTime(t *testing.T) {
	// ENCRE_04 §1: the server recomputes the score and its answer is the one
	// that counts, so two replays of one run may never disagree.
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{3, 3, 3}, answer(0, true), answer(1, true), answer(2, false))

	first, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	for range 50 {
		again, err := engine.Replay(run, nil, cfg)
		if err != nil {
			t.Fatalf("Replay: %v", err)
		}
		if again.Scores != first.Scores || again.Won != first.Won || again.FailedAt != first.FailedAt {
			t.Fatalf("replay disagreed: %+v then %+v", first, again)
		}
	}
}

func TestReplayRefusesARevancheTheScoreDidNotEarn(t *testing.T) {
	// The client asks for the Revanche; the server decides. A manche under
	// RevancheWindow of its target has not earned one, and a run claiming it
	// anyway is rejected rather than quietly honoured.
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{100, 3, 3}, answer(0, true), answer(1, true), answer(2, true))
	run.Revanche = [3]bool{true, false, false}

	if _, err := engine.Replay(run, nil, cfg); err == nil {
		t.Error("Replay honoured a Revanche on a manche far under its target, want an error")
	}
}

func TestAMancheWithinTheWindowMayBeReplayedOnce(t *testing.T) {
	// ENCRE_01: "il manquait N points" — at 85% of the target the manche is
	// offered again, and the second pass carries one more multiplier.
	//
	// "mot" scores three chips at a combo of one: three, unboosted, against a
	// target of 3.49 is 86% of it — inside the window on the score as it
	// actually fell, before the Revanche's own +1 Mult is added.
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{3.49, 3, 3},
		answer(0, true), answer(1, true), answer(2, true))
	run.Revanche = [3]bool{true, false, false}

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if !out.Won {
		t.Errorf("the Revanche did not save the run: %+v", out)
	}
	if !slices.Contains(out.Events, engine.RevancheTaken) {
		t.Errorf("events = %v, want RevancheTaken among them", out.Events)
	}
}

// TestRevancheEligibilityIgnoresItsOwnBonus is encre-00q.5: Replay used to
// judge a Revanche's eligibility on the score it had already boosted by the
// +1 Mult being claimed, so a manche that had actually fallen short of the
// 85% window could buy its own way in. Eligibility must be judged on the
// score as it fell; the +1 Mult belongs only to the replay that follows.
func TestRevancheEligibilityIgnoresItsOwnBonus(t *testing.T) {
	// "mot" scores three chips at a combo of one, unboosted. The two targets
	// below put that raw score at 82% and 86% of the target respectively —
	// under and over the 85% window — while the boosted score (mult 2) would
	// clear the 85% window either way, which is exactly the bug: judging
	// eligibility on the boosted number would call both of these eligible.
	tests := map[string]struct {
		target  float64
		wantWon bool
		wantErr bool
	}{
		"82 percent unboosted, not eligible": {target: 3.66, wantErr: true},
		"86 percent unboosted, eligible":     {target: 3.49, wantWon: true},
	}
	cfg := engine.DefaultConfig()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			run := runOf([3]float64{tc.target, 3, 3}, answer(0, true), answer(1, true), answer(2, true))
			run.Revanche = [3]bool{true, false, false}

			out, err := engine.Replay(run, nil, cfg)
			if tc.wantErr {
				if err == nil {
					t.Error("Replay honoured a Revanche below the 85% window, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			if out.Won != tc.wantWon {
				t.Errorf("Won = %v, want %v: %+v", out.Won, tc.wantWon, out)
			}
		})
	}
}

// TestReplayCountsTheGoldWordsAlreadyPlayedForTheCollectionneur exercises the
// one line score_test.go's Collectionneur case cannot reach: Replay's own
// goldPlayed counter, incremented as each attempt is walked so that a manche
// with two gold words pays the second one more than the first. Scoring the
// Talisman's bonus in isolation, as score_test.go does with a hand-picked
// Ctx.GoldPlayed, never runs the counter itself — only replaying a manche
// with a gold word ahead of another one does.
func TestReplayCountsTheGoldWordsAlreadyPlayedForTheCollectionneur(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := engine.Run{
		ID:        "r1",
		Deck:      engine.Deck{Week: words("un", "deux"), Seed: 7},
		Targets:   [3]float64{0, 0, 0},
		Talismans: []engine.TalismanID{engine.Collectionneur},
		Attempts: []engine.Attempt{
			{WordID: "un", Manche: 0, Correct: true},
			{WordID: "deux", Manche: 0, Correct: true},
		},
	}

	withGold := map[string]*engine.WordState{"un": {Gold: true}}
	out, err := engine.Replay(run, withGold, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	withoutGold := map[string]*engine.WordState{"un": {}}
	bare, err := engine.Replay(run, withoutGold, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if !(out.Scores[0] > bare.Scores[0]) {
		t.Errorf("manche score with the first word gold = %v, want more than %v without it",
			out.Scores[0], bare.Scores[0])
	}
}

func TestReplayScoresTrapsAtTheLevelsTheRunWasPlayedAt(t *testing.T) {
	// Levelling up between a run and its replay must not rewrite the score the
	// child earned, so the run carries the levels it was played at.
	cfg := engine.DefaultConfig()
	trapped := engine.Word{
		ID: "mot", Text: "mot", Letters: 3,
		Traps: map[engine.Color]int{engine.Muettes: 1},
	}

	run := engine.Run{
		ID: "r1", Deck: engine.Deck{Week: []engine.Word{trapped}, Seed: 7},
		Targets:  [3]float64{1, 1, 1},
		Levels:   map[engine.Color]int{engine.Muettes: 2},
		Attempts: []engine.Attempt{answer(0, true), answer(1, true), answer(2, true)},
	}

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	// Three letters plus one trap at level two: 3 + 1x10x2 = 23, at a combo of one.
	if want := 23.0; out.Scores[0] != want {
		t.Errorf("first manche scored %v, want %v", out.Scores[0], want)
	}
}

func TestReplayRefusesAWordThatIsNotInTheDeck(t *testing.T) {
	// A run naming a word it was never dealt is either a bug or a forgery; the
	// server is the authority and says no either way.
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{3, 3, 3}, engine.Attempt{WordID: "ailleurs", Correct: true})

	if _, err := engine.Replay(run, nil, cfg); err == nil {
		t.Error("Replay accepted an attempt on a word outside the deck, want an error")
	}
}

func TestReplayRefusesAMancheOutsideTheThree(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{3, 3, 3}, engine.Attempt{WordID: "mot", Manche: 7, Correct: true})

	if _, err := engine.Replay(run, nil, cfg); err == nil {
		t.Error("Replay accepted a fourth manche, want an error")
	}
}

func TestTheCahierStoresItsBonusForTheNextRun(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{3, 3, 3}, answer(0, true), answer(1, true), answer(2, true))
	run.Cahier = true

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if out.CahierBonus != cfg.CahierBonus {
		t.Errorf("CahierBonus = %v, want %v", out.CahierBonus, cfg.CahierBonus)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	// ENCRE_04 §4 asks for it by name. A phone that loses the network and
	// resends its run must not gild the same word twice.
	cfg := engine.DefaultConfig()
	c := newChild()
	run := runOf([3]float64{3, 3, 3}, answer(0, true), answer(1, true), answer(2, true))
	states := map[string]*engine.WordState{}

	out, err := engine.Replay(run, states, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if _, err := engine.Apply(c, run, out, states, 0, 0, cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	firstMastery := states["mot"].Mastery
	firstWins := c.BossWinsTotal

	events, err := engine.Apply(c, run, out, states, 0, 0, cfg)
	if err != nil {
		t.Fatalf("Apply the second time: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("the second Apply raised %v, want nothing", events)
	}
	if states["mot"].Mastery != firstMastery || c.BossWinsTotal != firstWins {
		t.Errorf("the second Apply changed the state: mastery %v then %v",
			firstMastery, states["mot"].Mastery)
	}
}

func TestApplyRecordsTheAttemptsAndTheBoss(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()
	run := runOf([3]float64{3, 3, 3}, answer(0, true), answer(1, true), answer(2, true))
	states := map[string]*engine.WordState{}

	out, _ := engine.Replay(run, states, cfg)
	if _, err := engine.Apply(c, run, out, states, 0, 0, cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	switch {
	case states["mot"] == nil || !states["mot"].Seen:
		t.Error("Apply did not record the word as seen")
	case c.BossWinsTotal != 1:
		t.Errorf("BossWinsTotal = %d, want 1 after a run won", c.BossWinsTotal)
	case c.Kindness != 1:
		t.Errorf("Kindness = %v, want 1 after a win", c.Kindness)
	}
}

func TestARunLostEarlyLowersTheTargetsOfTheNext(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()
	run := runOf([3]float64{1000, 3, 3}, answer(0, true))
	states := map[string]*engine.WordState{}

	out, _ := engine.Replay(run, states, cfg)
	if _, err := engine.Apply(c, run, out, states, 0, 0, cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if c.Kindness >= 1 {
		t.Errorf("Kindness = %v after a run lost in the first manche, want it lowered", c.Kindness)
	}
}

// TestReplayPlaysTheWeekSBossOnlyOnTheThirdManche is encre-00q.1: Replay used
// to hard-code Boss: NoBoss for every manche, so the Voleur d'accents never
// touched a score. The boss belongs to the third manche only (ENCRE_01 §3);
// an accented word in the first two must still pay in full.
func TestReplayPlaysTheWeekSBossOnlyOnTheThirdManche(t *testing.T) {
	cfg := engine.DefaultConfig()
	accented := engine.Word{
		ID: "école", Text: "école", Letters: 5,
		Traps: map[engine.Color]int{engine.Accentuees: 1},
	}
	run := engine.Run{
		ID:      "r1",
		Deck:    engine.Deck{Week: []engine.Word{accented}, Seed: 7, Boss: "Voleur d'accents"},
		Targets: [3]float64{0, 0, 0},
		Levels:  map[engine.Color]int{engine.Accentuees: 2},
		Attempts: []engine.Attempt{
			{WordID: "école", Manche: 0, Correct: true},
			{WordID: "école", Manche: 2, Correct: true},
		},
	}

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if out.Scores[0] <= out.Scores[2] {
		t.Errorf("manche 1 scored %v and the boss manche %v, want the boss manche lower — "+
			"the Voleur d'accents takes the Accentuées to nothing there", out.Scores[0], out.Scores[2])
	}
}

// TestTheChronometreRewardsAnAnswerInsideItsWindow is encre-00q.1: Score reads
// Ctx.Fast, but Replay never derived it from Attempt.Millis, so the
// Chronomètre never paid.
func TestTheChronometreRewardsAnAnswerInsideItsWindow(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := engine.Run{
		ID:        "r1",
		Deck:      engine.Deck{Week: words("mot"), Seed: 7},
		Targets:   [3]float64{0, 0, 0},
		Talismans: []engine.TalismanID{engine.Chronometre},
		Attempts: []engine.Attempt{
			{WordID: "mot", Manche: 0, Correct: true, Millis: int(cfg.ChronoSeconds*1000) - 1},
		},
	}
	slow := run
	slow.Attempts = []engine.Attempt{
		{WordID: "mot", Manche: 0, Correct: true, Millis: int(cfg.ChronoSeconds*1000) + 1000},
	}

	fast, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	slowOut, err := engine.Replay(slow, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if fast.Scores[0] <= slowOut.Scores[0] {
		t.Errorf("fast answer scored %v, slow one %v, want the fast one higher — the Chronomètre pays speed",
			fast.Scores[0], slowOut.Scores[0])
	}
}

// TestAWrongAnswerRetombeTheComboToOne is ENCRE_01 §6: a fault resets the
// combo to one. Replay never did this at all until encre-00q.1.
func TestAWrongAnswerRetombeTheComboToOne(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{0, 0, 0},
		answer(0, true), answer(0, false), answer(0, true))

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	// "mot" is three letters: the first word scores at combo 1 (3), the
	// second is wrong (0), the third is back at combo 1 too (3) — not 3 as it
	// would be at a combo of two-then-broken. Total: 3 + 0 + 3 = 6.
	if want := 6.0; out.Scores[0] != want {
		t.Errorf("manche score = %v, want %v — the fault must reset the combo", out.Scores[0], want)
	}
}

// TestTheGommeForgivesTheFirstFaultOfAManche is ENCRE_01 §12: "une faute
// pardonnée par manche". The first wrong answer of a manche, with the Gomme
// carried, leaves the combo untouched — the word itself still scores zero.
func TestTheGommeForgivesTheFirstFaultOfAManche(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := engine.Run{
		ID:        "r1",
		Deck:      engine.Deck{Week: words("mot"), Seed: 7},
		Targets:   [3]float64{0, 0, 0},
		Talismans: []engine.TalismanID{engine.Gomme},
		Attempts: []engine.Attempt{
			{WordID: "mot", Manche: 0, Correct: true},
			{WordID: "mot", Manche: 0, Correct: false},
			{WordID: "mot", Manche: 0, Correct: true},
		},
	}

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	// Combo 1, then the fault forgiven — the combo holds at 2 rather than
	// falling back to 1 — then a third word played at that same combo of 2:
	// 3 + 0 + 6 = 9, against 6 without the Gomme (a reset to 1 either side).
	if want := 9.0; out.Scores[0] != want {
		t.Errorf("manche score with the Gomme = %v, want %v", out.Scores[0], want)
	}
}

// TestTheGommeOnlyForgivesOneFaultPerManche checks the second fault of the
// same manche still breaks the combo even with the Gomme carried.
func TestTheGommeOnlyForgivesOneFaultPerManche(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := engine.Run{
		ID:        "r1",
		Deck:      engine.Deck{Week: words("mot"), Seed: 7},
		Targets:   [3]float64{0, 0, 0},
		Talismans: []engine.TalismanID{engine.Gomme},
		Attempts: []engine.Attempt{
			{WordID: "mot", Manche: 0, Correct: false},
			{WordID: "mot", Manche: 0, Correct: false},
			{WordID: "mot", Manche: 0, Correct: true},
		},
	}

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	// First fault forgiven (combo stays 1), second fault is not (combo resets
	// to 1 again), third word scores at combo 1: 0 + 0 + 3 = 3.
	if want := 3.0; out.Scores[0] != want {
		t.Errorf("manche score = %v, want %v", out.Scores[0], want)
	}
}

// TestTheComboPersistsBetweenManches is ENCRE_01 §6: the combo carries from
// one manche into the next, so the boss is naturally played at a high
// multiplier. Replay used to reset it to one at the top of every manche.
func TestTheComboPersistsBetweenManches(t *testing.T) {
	cfg := engine.DefaultConfig()
	run := runOf([3]float64{0, 0, 0}, answer(0, true), answer(1, true))

	out, err := engine.Replay(run, nil, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	// Manche 0 plays at combo 1 (3 chips), manche 1 opens at combo 2 (6 chips)
	// rather than resetting to combo 1 (which would also score 3).
	if out.Scores[1] <= out.Scores[0] {
		t.Errorf("manche 0 scored %v, manche 1 scored %v, want manche 1 higher — the combo carries over",
			out.Scores[0], out.Scores[1])
	}
}
