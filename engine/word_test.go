package engine_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
)

const learnRate = 0.25

func ok(st *engine.WordState, day int32, cfg engine.Config) []engine.Event {
	return engine.Record(st, engine.Attempt{Correct: true}, learnRate, day, day/7, cfg, nil)
}

func miss(st *engine.WordState, day int32, cfg engine.Config) []engine.Event {
	return engine.Record(st, engine.Attempt{Correct: false}, learnRate, day, day/7, cfg, nil)
}

func TestAWordTurnsGoldOnThreeDaysSpreadOverAWeek(t *testing.T) {
	// ENCRE_01: gold means remembered, not drilled. Three distinct days AND a
	// span of at least a week, so an afternoon of repetition cannot buy it.
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}

	ok(st, 0, cfg)
	ok(st, 3, cfg)
	events := ok(st, 8, cfg)

	if !st.Gold {
		t.Errorf("after days 0, 3 and 8 the word is not gold: %+v", st)
	}
	if !slices.Contains(events, engine.GoldEarned) {
		t.Errorf("events = %v, want GoldEarned among them", events)
	}
}

func TestThreeSuccessesInOneDayDoNotBuyGold(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}

	for range 3 {
		ok(st, 0, cfg)
	}

	if st.Gold {
		t.Error("three successes on one day turned the word gold, want them not to")
	}
}

func TestThreeDaysTooCloseTogetherDoNotBuyGold(t *testing.T) {
	// Three distinct days inside six is still a cram.
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}

	ok(st, 0, cfg)
	ok(st, 2, cfg)
	ok(st, 5, cfg)

	if st.Gold {
		t.Error("three days inside a week turned the word gold, want them not to")
	}
}

func TestAMissLosesTheGold(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{Gold: true}

	events := miss(st, 20, cfg)

	if st.Gold {
		t.Error("the word is still gold after a miss")
	}
	if !slices.Contains(events, engine.GoldLost) {
		t.Errorf("events = %v, want GoldLost among them", events)
	}
}

func TestAGoldWordLeftAloneForFourWeeksTarnishes(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{Gold: true, LastPlayedW: 2}

	if got := engine.Tarnish(st, 2+int32(cfg.TarnishWeeks)-1, cfg); got {
		t.Error("the word tarnished a week early")
	}
	if got := engine.Tarnish(st, 2+int32(cfg.TarnishWeeks), cfg); !got {
		t.Errorf("the word did not tarnish after %d weeks", cfg.TarnishWeeks)
	}
	if !st.Tarnished {
		t.Error("Tarnish reported the change but did not make it")
	}
}

func TestSpellingATarnishedWordRestoresIt(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{Gold: true, Tarnished: true}

	events := ok(st, 30, cfg)

	if st.Tarnished {
		t.Error("the word is still tarnished after being spelled right")
	}
	if !slices.Contains(events, engine.Restored) {
		t.Errorf("events = %v, want Restored among them", events)
	}
}

func TestThreeMissesInThreeWeeksCurseTheWord(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}

	miss(st, 0, cfg)
	miss(st, 7, cfg)
	events := miss(st, 14, cfg)

	if !st.Cursed {
		t.Errorf("three misses in three weeks left the word uncursed: %+v", st)
	}
	if !slices.Contains(events, engine.Cursed) {
		t.Errorf("events = %v, want Cursed among them", events)
	}
}

func TestMissesSpreadWiderThanTheWindowDoNotCurse(t *testing.T) {
	// The curse is for a word fought with now, not one missed once a term.
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}

	miss(st, 0, cfg)
	miss(st, 70, cfg)
	miss(st, 140, cfg)

	if st.Cursed {
		t.Error("three misses months apart cursed the word, want them not to")
	}
}

func TestThreeSuccessesInARowTameACursedWordIntoGold(t *testing.T) {
	// ENCRE_01: taming a curse is the way back, and it lands on gold rather
	// than on plain — the reward for facing the word that beat you.
	cfg := engine.DefaultConfig()
	st := &engine.WordState{Cursed: true}

	ok(st, 0, cfg)
	ok(st, 1, cfg)
	events := ok(st, 2, cfg)

	switch {
	case st.Cursed:
		t.Error("the word is still cursed after three successes in a row")
	case !st.Gold:
		t.Error("a tamed word did not land on gold")
	case !slices.Contains(events, engine.Tamed):
		t.Errorf("events = %v, want Tamed among them", events)
	}
}

func TestAMissBreaksTheRunTowardsTaming(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{Cursed: true}

	ok(st, 0, cfg)
	ok(st, 1, cfg)
	miss(st, 2, cfg)
	ok(st, 3, cfg)

	if !st.Cursed {
		t.Error("the curse lifted on a broken run of successes")
	}
}

func TestARencontreMarksTheWordSeenWithoutCountingTowardsGold(t *testing.T) {
	// ENCRE_01: a Rencontre shows the word rather than asking for it. It
	// teaches — mastery moves — but it is not an answer, so it may not feed the
	// combo nor the three days that make gold.
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}

	for _, day := range []int32{0, 4, 9} {
		engine.Record(st, engine.Attempt{Correct: true, Copy: true}, learnRate, day, day/7, cfg, nil)
	}

	switch {
	case !st.Seen:
		t.Error("a Rencontre did not mark the word seen")
	case st.Gold:
		t.Error("three Rencontres turned the word gold, want them not to")
	case len(st.SuccessDays) != 0:
		t.Errorf("SuccessDays = %v, want a Rencontre to add none", st.SuccessDays)
	}
	if want := 3 * 0.12; st.Mastery < want-1e-9 || st.Mastery > want+1e-9 {
		t.Errorf("Mastery after three Rencontres = %v, want %v", st.Mastery, want)
	}
}

func TestMasteryRisesFasterOnASuccessThanOnAMiss(t *testing.T) {
	// A miss still teaches — the correction is shown — but only half as much.
	cfg := engine.DefaultConfig()
	won, lost := &engine.WordState{}, &engine.WordState{}

	ok(won, 0, cfg)
	miss(lost, 0, cfg)

	if !(won.Mastery > lost.Mastery && lost.Mastery > 0) {
		t.Errorf("mastery after a success = %v, after a miss = %v; want the first larger and the second above zero",
			won.Mastery, lost.Mastery)
	}
}

func TestShineIsDrawnOnceAtTheRatesOfTheConfig(t *testing.T) {
	// One word in eight comes back holographic and one in forty polychrome,
	// drawn from the run's own seed so the server can replay it.
	cfg := engine.DefaultConfig()
	counts := map[engine.Shine]int{}
	const draws = 20000

	for i := range draws {
		rng := rand.New(rand.NewPCG(uint64(i), 7))
		st := &engine.WordState{}
		engine.Record(st, engine.Attempt{Correct: true}, learnRate, 0, 0, cfg, rng)
		counts[st.Shine]++
	}

	holo := float64(counts[engine.ShineHolo]) / draws
	poly := float64(counts[engine.ShinePoly]) / draws
	if holo < cfg.HoloOdds*0.85 || holo > cfg.HoloOdds*1.15 {
		t.Errorf("holo rate = %.3f, want about %.3f", holo, cfg.HoloOdds)
	}
	if poly < cfg.PolyOdds*0.7 || poly > cfg.PolyOdds*1.3 {
		t.Errorf("poly rate = %.3f, want about %.3f", poly, cfg.PolyOdds)
	}
}

func TestAWordAlreadyShiningIsNotDrawnAgain(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{Shine: engine.ShinePoly, Seen: true}

	engine.Record(st, engine.Attempt{Correct: true}, learnRate, 0, 0, cfg, rand.New(rand.NewPCG(1, 1)))

	if st.Shine != engine.ShinePoly {
		t.Errorf("Shine = %v, want the polychrome it already had", st.Shine)
	}
}
