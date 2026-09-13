package engine

import (
	"math/rand/v2"
	"slices"
)

// Event is something worth telling the child about (ENCRE_04 §4). The client
// turns each into an animation, so they are named for what happened to the
// word, not for the field that changed.
type Event string

// The events a word's life can raise.
const (
	GoldEarned Event = "GoldEarned"
	GoldLost   Event = "GoldLost"
	Tarnished  Event = "Tarnished"
	Restored   Event = "Restored"
	Cursed     Event = "Cursed"
	Tamed      Event = "Tamed"
	Shined     Event = "Shine"
)

// rencontreMastery is what seeing a word in a Rencontre teaches. It is a
// fraction of what answering teaches, because being shown a word is not the
// same as having written it.
const rencontreMastery = 0.12

// tamingRun is the successes in a row that lift a curse (ENCRE_01).
const tamingRun = 3

// Record applies one attempt to the word's state and returns what changed.
//
// rng may be nil, in which case no shine is drawn — which is what callers that
// only want the transitions do. When it is given it must come from the run's
// own seed, so the server replaying the run draws the same shine the child saw.
func Record(st *WordState, a Attempt, learnRate float64, day, week int32, cfg Config, rng *rand.Rand) []Event {
	var events []Event
	st.Seen = true
	st.LastPlayedW = week

	// A Rencontre shows the word instead of asking for it. It teaches, so
	// mastery moves, but it is not an answer: it feeds neither the combo nor
	// the days that make gold, or a child could gild a deck by being shown it.
	if a.Copy {
		st.Mastery += rencontreMastery
		return events
	}

	if a.Correct {
		st.Mastery += learnRate * (1 - st.Mastery)
		st.ConsecOK++
		if !slices.Contains(st.SuccessDays, day) {
			st.SuccessDays = append(st.SuccessDays, day)
		}
		if st.FirstSuccess == 0 && len(st.SuccessDays) == 1 {
			st.FirstSuccess = day
		}
		if st.Tarnished {
			st.Tarnished = false
			events = append(events, Restored)
		}
		if st.Cursed && st.ConsecOK >= tamingRun {
			// A tamed word lands on gold rather than on plain: it is the
			// reward for going back to the word that beat you.
			st.Cursed, st.Gold = false, true
			events = append(events, Tamed, GoldEarned)
		} else if !st.Gold && remembered(st, cfg) {
			st.Gold = true
			events = append(events, GoldEarned)
		}
	} else {
		// A miss still teaches, because the correction is shown — but half as
		// much as writing the word right.
		st.Mastery += 0.5 * learnRate * (1 - st.Mastery)
		st.Fails++
		st.ConsecOK = 0
		st.FailWeeks = append(st.FailWeeks, week)
		if st.Gold {
			st.Gold, st.Tarnished = false, false
			events = append(events, GoldLost)
		}
		if !st.Cursed && recentFails(st, week, cfg) >= cfg.CurseFails {
			st.Cursed = true
			events = append(events, Cursed)
		}
	}

	if rng != nil && st.Shine == ShineNone {
		if s := drawShine(rng, cfg); s != ShineNone {
			st.Shine = s
			events = append(events, Shined)
		}
	}
	return events
}

// remembered reports whether the word has been spelled right on enough distinct
// days, spread widely enough, to be gold.
//
// Both halves matter: the count alone would let an afternoon of repetition buy
// gold, and gold is meant to say remembered.
func remembered(st *WordState, cfg Config) bool {
	if len(st.SuccessDays) < cfg.GoldDays {
		return false
	}
	first, last := slices.Min(st.SuccessDays), slices.Max(st.SuccessDays)
	return last-first >= cfg.GoldMinSpanDays
}

// recentFails counts the misses inside the curse's window, so a word missed
// once a term is never cursed — the curse is for a word being fought with now.
func recentFails(st *WordState, week int32, cfg Config) int {
	n := 0
	for _, w := range st.FailWeeks {
		if week-w < cfg.CurseWeeks {
			n++
		}
	}
	return n
}

// drawShine rolls for the finish a word comes back wearing. Polychrome is drawn
// first because it is the rarer of the two.
func drawShine(rng *rand.Rand, cfg Config) Shine {
	switch r := rng.Float64(); {
	case r < cfg.PolyOdds:
		return ShinePoly
	case r < cfg.PolyOdds+cfg.HoloOdds:
		return ShineHolo
	default:
		return ShineNone
	}
}

// Tarnish marks a gold word left unplayed too long, and reports whether it just
// changed. A tarnished word pays double, which is how the game pulls a child
// back to something they have stopped meeting.
func Tarnish(st *WordState, week int32, cfg Config) bool {
	if !st.Gold || st.Tarnished || week-st.LastPlayedW < cfg.TarnishWeeks {
		return false
	}
	st.Tarnished = true
	return true
}
