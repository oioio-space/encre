package engine

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
)

// RevancheTaken is raised when a manche was replayed under the Revanche.
const RevancheTaken Event = "Revanche"

// manches is how many a run holds.
const manches = 3

// ErrAlreadyApplied is returned by Apply for a run whose effects are already
// recorded. It is not a failure: a phone that lost the network and resent its
// run gets this, and the right thing to do is nothing.
var ErrAlreadyApplied = errors.New("engine: this run was already applied")

// Run is everything the client reports about one run (ENCRE_04 §4).
type Run struct {
	ID        string
	ChildID   string
	Deck      Deck
	Rank      int
	Targets   [3]float64
	Talismans []TalismanID
	Rooms     [2]RoomID
	// Levels is the child's level in each Couleur AT THE TIME OF THE RUN.
	// Without it a replay would score the traps at today's levels, so levelling
	// up would silently rewrite a score earned last week — and ENCRE_04 §1
	// makes this recomputation the one that counts.
	Levels   map[Color]int
	Attempts []Attempt
	// Revanche says which manches the child replayed. The client asks; Replay
	// decides whether the score had earned it.
	Revanche [3]bool
	Cahier   bool
}

// Outcome is what a run came to, as the server computes it.
type Outcome struct {
	Scores [3]float64
	// FailedAt is the manche the run ended in, or -1 when it was won.
	FailedAt int
	Won      bool
	Money    int
	// CahierBonus is the chips carried into the next run.
	CahierBonus float64
	Events      []Event
}

// Replay recomputes a run from its attempts, and is the score that counts.
//
// The client scores as it plays so the counter can move, but ENCRE_04 §1 makes
// the server the authority: this recomputes everything from the deck seed and
// the attempts, and disagreeing with the client is an answer, not an error.
// It is deterministic — the same run replays to the same outcome every time.
//
// It returns an error for a run that could not have been played: a word outside
// the deck, a fourth manche, or a Revanche the score had not earned.
func Replay(run Run, states map[string]*WordState, cfg Config) (Outcome, error) {
	out := Outcome{FailedAt: -1, Money: cfg.RunMoney}

	byID := map[string]Word{}
	for _, w := range slices.Concat(run.Deck.Week, run.Deck.Garde, run.Deck.Old, run.Deck.Cursed) {
		byID[w.ID] = w
	}
	owned := Talismans{}
	for _, id := range run.Talismans {
		owned[id] = true
	}

	// Words are scored against a copy of the state, so Replay never changes
	// anything: Apply is the only thing allowed to.
	scratch := map[string]*WordState{}
	state := func(id string) *WordState {
		if st, ok := scratch[id]; ok {
			return st
		}
		st := &WordState{}
		if live := states[id]; live != nil {
			copied := *live
			st = &copied
		}
		scratch[id] = st
		return st
	}

	for manche := range manches {
		combo := 1.0
		// A Revanche replays the manche with one more multiplier on everything.
		if run.Revanche[manche] {
			combo++
		}
		goldPlayed := 0

		for _, a := range run.Attempts {
			if a.Manche < 0 || a.Manche >= manches {
				return Outcome{}, fmt.Errorf("engine: attempt on manche %d, which is outside the three", a.Manche)
			}
			if a.Manche != manche {
				continue
			}
			w, ok := byID[a.WordID]
			if !ok {
				return Outcome{}, fmt.Errorf("engine: attempt on word %q, which is not in the deck", a.WordID)
			}
			st := state(a.WordID)
			ctx := Ctx{Levels: run.Levels, Boss: NoBoss, GoldPlayed: goldPlayed}
			if st.Gold {
				goldPlayed++
			}
			chips, mult := Score(a, w, st, owned, combo, ctx, cfg)
			out.Scores[manche] += chips * mult
			if a.Correct && !a.Copy {
				combo++
			}
		}

		target := run.Targets[manche]
		if out.Scores[manche] >= target {
			// Four for the manche, and up to three more for beating it well.
			out.Money += cfg.MancheMoney + min(3, int(out.Scores[manche]/max(target, 1)))
			continue
		}
		if run.Revanche[manche] {
			// The client claimed a Revanche; it is only legal from inside the
			// window, and the server checks rather than trusts.
			if out.Scores[manche] < target*cfg.RevancheWindow {
				return Outcome{}, fmt.Errorf(
					"engine: manche %d claimed a Revanche at %.1f of a target of %.1f, under the %.0f%% window",
					manche, out.Scores[manche], target, cfg.RevancheWindow*100,
				)
			}
			out.Events = append(out.Events, RevancheTaken)
			out.Money += cfg.MancheMoney
			continue
		}
		out.FailedAt = manche
		return out, nil
	}

	out.Won = true
	if run.Cahier {
		out.CahierBonus = cfg.CahierBonus
	}
	return out, nil
}

// Apply writes a run's outcome into the child and the words, once.
//
// A phone that loses the network resends its run, so this records what it has
// already applied and returns [ErrAlreadyApplied] the second time rather than
// gilding the same word twice (ENCRE_04 §4).
func Apply(c *Child, run Run, out Outcome, states map[string]*WordState, day, week int32, cfg Config) ([]Event, error) {
	if c.AppliedRuns == nil {
		c.AppliedRuns = map[string]bool{}
	}
	if c.AppliedRuns[run.ID] {
		return nil, nil
	}
	c.AppliedRuns[run.ID] = true

	byID := map[string]Word{}
	for _, w := range slices.Concat(run.Deck.Week, run.Deck.Garde, run.Deck.Old, run.Deck.Cursed) {
		byID[w.ID] = w
	}
	// Shine is drawn from the deck's own seed, so the server draws what the
	// child saw rather than something new.
	// #nosec G404 -- reproducing the child's draw is the requirement here.
	draw := rand.New(rand.NewPCG(run.Deck.Seed, 0x5417))

	var events []Event
	for _, a := range run.Attempts {
		w, ok := byID[a.WordID]
		if !ok {
			return nil, fmt.Errorf("engine: attempt on word %q, which is not in the deck", a.WordID)
		}
		st := states[a.WordID]
		if st == nil {
			st = &WordState{}
			states[a.WordID] = st
		}
		events = append(events, Record(st, a, c.LearnRate, day, week, cfg, draw)...)
		events = append(events, c.GainXP(w, a, Ctx{Boss: NoBoss}, week, cfg)...)
	}

	if out.Won {
		c.Win()
		events = append(events, c.WinBoss(week, cfg)...)
	} else {
		// Only a run lost early buys Bienveillance: going down in the third
		// manche means the targets were nearly right.
		if out.FailedAt < manches-1 {
			c.LoseEarly(cfg)
		}
		events = append(events, c.LoseBoss(cfg)...)
	}
	return append(events, out.Events...), nil
}
