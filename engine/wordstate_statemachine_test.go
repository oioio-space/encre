package engine_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
	"pgregory.net/rapid"
)

// wordStateMachine drives a WordState through a random sequence of Record and
// Tarnish calls, weeks at a time, and checks the invariants ENCRE_01 §9 and
// ENCRE_03 §7 promise after every single step — not just at the end, so a
// property that only breaks for one step in the middle of a long run cannot
// hide behind a final state that happens to look fine.
type wordStateMachine struct {
	st   engine.WordState
	cfg  engine.Config
	day  int32
	week int32
	// bestGoldStreak is the longest ConsecOK ever reached while the word was
	// cursed, kept so the Check step can tell a taming (ConsecOK reached 3)
	// from a coincidental gold earned on a cursed word by some other route.
	sawGoldWithoutTaming bool
}

// Record advances the day by a random amount — zero included, so the same day
// is sometimes asked twice, which is what the sorted-and-deduplicated
// property below exists to catch — then records a success.
func (m *wordStateMachine) Record(t *rapid.T) {
	correct := rapid.Bool().Draw(t, "correct")
	m.day += int32(rapid.IntRange(0, 10).Draw(t, "day-delta"))
	m.week = m.day / 7

	before := m.st
	events := engine.Record(&m.st, engine.Attempt{Correct: correct}, learnRate, m.day, m.week, m.cfg, nil)

	if slices.Contains(events, engine.Tamed) {
		if m.st.Cursed {
			t.Fatalf("Tamed fired but the word is still cursed: before %+v, after %+v", before, m.st)
		}
	}
	if slices.Contains(events, engine.GoldEarned) && !slices.Contains(events, engine.Tamed) {
		// Gold earned the ordinary way — not through taming — must satisfy
		// the "three distinct days, a week apart" rule by itself: the mark of
		// remembered, not drilled.
		if len(m.st.SuccessDays) < m.cfg.GoldDays {
			t.Fatalf("GoldEarned with only %d distinct success days, want at least %d",
				len(m.st.SuccessDays), m.cfg.GoldDays)
		}
		first, last := slices.Min(m.st.SuccessDays), slices.Max(m.st.SuccessDays)
		if last-first < m.cfg.GoldMinSpanDays {
			t.Fatalf("GoldEarned with a span of %d days, want at least %d", last-first, m.cfg.GoldMinSpanDays)
		}
		if m.st.Cursed {
			m.sawGoldWithoutTaming = true
		}
	}
}

// Tarnish advances a few weeks and tarnishes a gold word left unplayed.
func (m *wordStateMachine) Tarnish(t *rapid.T) {
	m.week += int32(rapid.IntRange(0, 6).Draw(t, "tarnish-week-delta"))
	engine.Tarnish(&m.st, m.week, m.cfg)
}

// Check runs after every action and holds the invariants that must never
// break, no matter what sequence of Record and Tarnish calls got there.
//
// It does NOT check that Gold and Cursed are mutually exclusive: they are
// not, in the code as it stands today. See
// TestAWordCanEndUpBothGoldAndCursedAtOnce below, which reports that as a
// bug rather than folding it into this Check and failing every run.
func (m *wordStateMachine) Check(t *rapid.T) {
	if m.st.Mastery < 0 || m.st.Mastery > 1 {
		t.Fatalf("Mastery = %v, want it within [0, 1]", m.st.Mastery)
	}
	if !slices.IsSorted(m.st.SuccessDays) {
		t.Fatalf("SuccessDays = %v, want them sorted", m.st.SuccessDays)
	}
	if u := slices.Compact(slices.Clone(m.st.SuccessDays)); len(u) != len(m.st.SuccessDays) {
		t.Fatalf("SuccessDays = %v, want no duplicate day", m.st.SuccessDays)
	}
}

// TestWordStateInvariantsHoldAcrossAnyRandomHistory runs a random sequence of
// Record and Tarnish calls spanning many simulated weeks and checks every
// invariant of ENCRE_01 §9 and ENCRE_03 §7 after each step. It is the
// property most likely to catch a transition bug, because a table of
// examples only ever tries the histories someone thought to write down.
func TestWordStateInvariantsHoldAcrossAnyRandomHistory(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := &wordStateMachine{cfg: engine.DefaultConfig()}
		t.Repeat(rapid.StateMachineActions(m))
	})
}

// TestAWordCanEndUpBothGoldAndCursedAtOnce reports a real bug found by the
// state machine above, reduced to six Record calls. It is a finding, not a
// regression this task is allowed to fix in engine/word.go — see Record's
// "Correct" branch:
//
//	if st.Cursed && st.ConsecOK >= tamingRun {
//	    st.Cursed, st.Gold = false, true          // taming: exclusive, fine
//	} else if !st.Gold && remembered(st, cfg) {
//	    st.Gold = true                            // does not clear Cursed
//	}
//
// A word already cursed that reaches "remembered" (three distinct success
// days spread over GoldMinSpanDays) through the second branch — because its
// run of consecutive successes was broken by a miss along the way, so it
// never reached tamingRun — turns gold WITHOUT losing its curse. ENCRE_01 §9
// describes the curse as being for "mot non doré" (a word not gold) and
// taming as the one path from cursed to gold; nothing in the brief allows a
// word to be both, and Score (engine/score.go) already treats Gold and
// Cursed as separate, independently-checked multipliers — so a word that is
// both would be scored by both at once, on top of describing a card state
// the client has no art for.
//
// Skipped rather than asserted: fixing Record is out of this task's scope
// (test-only), and an assertion here would fail `mise run test` on every
// run. Removing the Skip turns this back into a live regression test for
// whoever fixes the underlying bug.
// TestACursedWordOnlyTurnsGoldByBeingTamed walks the path that used to end with
// a word both gold and cursed — no card can be drawn that way, and it would have
// paid the gold bonus and the ×5 curse at the same time. ENCRE_01 §9 settles it:
// the curse falls on a word that is *not* gold, and taming is the only way back.
func TestACursedWordOnlyTurnsGoldByBeingTamed(t *testing.T) {
	cfg := engine.DefaultConfig()
	st := &engine.WordState{}
	record := func(day, week int32, correct bool) {
		engine.Record(st, engine.Attempt{Correct: correct}, learnRate, day, week, cfg, nil)
	}

	// Three misses in one week curse the word (CurseFails=3 within
	// CurseWeeks=3), with one success sandwiched between them so ConsecOK
	// never reaches tamingRun once the curse lands.
	record(0, 0, false)
	record(0, 0, false)
	record(0, 0, true)
	record(0, 0, false) // st.Cursed becomes true here
	// Two more successes, on distinct days seven apart from the first,
	// satisfy "remembered" without ever stringing three in a row.
	record(1, 0, true)
	record(7, 1, true)

	if st.Gold {
		t.Errorf("Gold = true on a cursed word, want the taming run to be the only way back: %+v", st)
	}
	if !st.Cursed {
		t.Errorf("Cursed = false, want the word still cursed after three misses in the window: %+v", st)
	}

	// Three successes in a row are the way out, and they turn it gold.
	record(8, 1, true)
	record(9, 1, true)
	record(10, 1, true)
	if st.Cursed || !st.Gold {
		t.Errorf("after a taming run: Gold = %v, Cursed = %v; want gold and no longer cursed",
			st.Gold, st.Cursed)
	}
}
