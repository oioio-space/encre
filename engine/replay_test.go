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
	cfg := engine.DefaultConfig()
	// Three chips at a multiplier of two — one for the combo, one the Revanche
	// adds — make six against a target of seven: short, but 86% of it, which is
	// inside the window.
	run := runOf([3]float64{7, 3, 3},
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
