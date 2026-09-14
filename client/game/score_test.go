package game_test

import (
	"math/rand/v2"
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

func randomWord(rng *rand.Rand, id string) engine.Word {
	traps := map[engine.Color]int{}
	for _, c := range engine.Colors() {
		if rng.IntN(3) == 0 {
			traps[c] = rng.IntN(4)
		}
	}
	return engine.Word{ID: id, Text: id, Letters: 3 + rng.IntN(8), Traps: traps}
}

func randomState(rng *rand.Rand) *engine.WordState {
	return &engine.WordState{
		Gold:      rng.IntN(4) == 0,
		Tarnished: rng.IntN(5) == 0,
		Cursed:    rng.IntN(6) == 0,
	}
}

func randomAttempt(rng *rand.Rand, deck []engine.Word, manche int) engine.Attempt {
	w := deck[rng.IntN(len(deck))]
	return engine.Attempt{
		WordID:  w.ID,
		Manche:  manche,
		Blind:   rng.IntN(4) == 0,
		Copy:    rng.IntN(6) == 0,
		Correct: rng.IntN(5) != 0,
		Millis:  rng.IntN(20000),
	}
}

// TestRunScoreMatchesEngineReplayToTheToken is the acceptance test bead
// encre-cs5 names explicitly: the client shows RunScore's own running total
// so the counter can move before the network answers, but engine.Replay is
// what the server counts (ENCRE_04 §1) — a client that reimplemented the
// rules could silently drift from it. RunScore never does: [RunScore.Apply]
// calls engine.Score for every attempt, kept in lockstep with the same combo,
// Revanche bonus, Gomme forgiveness and boss that engine.Replay's own manche
// loop carries. This generates a three-manche run's worth of random attempts,
// grouped by manche in the order they are actually played — the only order a
// live run ever hands them to a scene in — and checks RunScore's own total
// for every manche agrees with engine.Replay's, exactly, float64 bit for bit.
func TestRunScoreMatchesEngineReplayToTheToken(t *testing.T) {
	cfg := engine.DefaultConfig()
	rng := rand.New(rand.NewPCG(1, 2))

	for trial := range 200 {
		deck := make([]engine.Word, 4+rng.IntN(6))
		for i := range deck {
			deck[i] = randomWord(rng, string(rune('a'+i)))
		}
		states := map[string]*engine.WordState{}
		for _, w := range deck {
			states[w.ID] = randomState(rng)
		}
		owned := engine.Talismans{}
		for id := range 20 {
			if rng.IntN(3) == 0 {
				owned[engine.TalismanID(id)] = true
			}
		}
		levels := map[engine.Color]int{}
		for _, c := range engine.Colors() {
			levels[c] = 1 + rng.IntN(10)
		}

		var revanche [3]bool
		var attempts []engine.Attempt
		for manche := range 3 {
			revanche[manche] = rng.IntN(2) == 0
			for range 3 + rng.IntN(6) {
				attempts = append(attempts, randomAttempt(rng, deck, manche))
			}
		}

		run := engine.Run{
			ID:       "run",
			Deck:     engine.Deck{Week: deck, Boss: "Chuchoteur"},
			Levels:   levels,
			Attempts: attempts,
			Targets:  [3]float64{0, 0, 0}, // always met: every manche's own total is under test here
			Revanche: revanche,
		}
		for id := range owned {
			run.Talismans = append(run.Talismans, id)
		}

		// Replay reads states, but never writes them (only Apply does), so the
		// same map is safe to hand to both paths below.
		out, err := engine.Replay(run, states, cfg)
		if err != nil {
			t.Fatalf("trial %d: Replay: %v", trial, err)
		}

		byID := map[string]engine.Word{}
		for _, w := range deck {
			byID[w.ID] = w
		}
		rs := game.NewRunScore(cfg)
		for manche := range 3 {
			boss := engine.NoBoss
			if manche == 2 { // the last manche is the boss's, matching bossOf("Chuchoteur")
				boss = engine.Chuchoteur
			}
			rs.StartManche(manche, boss, revanche[manche])
			for _, a := range attempts {
				if a.Manche != manche {
					continue
				}
				rs.Apply(a, byID[a.WordID], states[a.WordID], owned, levels)
			}
		}

		for manche := range 3 {
			if got, want := rs.Total(manche), out.Scores[manche]; got != want {
				t.Fatalf("trial %d, manche %d: RunScore.Total() = %v, want engine.Replay's %v", trial, manche, got, want)
			}
		}
	}
}

func TestRunScoreStartsAtZero(t *testing.T) {
	rs := game.NewRunScore(engine.DefaultConfig())
	if got := rs.Total(0); got != 0 {
		t.Errorf("Total(0) on a fresh RunScore = %v, want 0", got)
	}
	if got := rs.Combo(); got != 1 {
		t.Errorf("Combo() on a fresh RunScore = %v, want 1", got)
	}
}

func TestRunScoreReportsTheChipsAndMultOfEachAttempt(t *testing.T) {
	cfg := engine.DefaultConfig()
	w := engine.Word{ID: "chat", Text: "chat", Letters: 4}
	st := &engine.WordState{}
	rs := game.NewRunScore(cfg)
	rs.StartManche(0, engine.NoBoss, false)

	chips, mult := rs.Apply(engine.Attempt{WordID: "chat", Correct: true}, w, st, nil, nil)

	if chips != 4 {
		t.Errorf("chips = %v, want 4 (the word's own letters, no traps)", chips)
	}
	if mult != 1 {
		t.Errorf("mult = %v, want 1 (the starting combo)", mult)
	}
	if rs.Total(0) != 4 {
		t.Errorf("Total(0) = %v, want 4", rs.Total(0))
	}
}

func TestRunScoreScoresNothingOnAWrongAnswerAndResetsTheCombo(t *testing.T) {
	cfg := engine.DefaultConfig()
	w := engine.Word{ID: "chat", Text: "chat", Letters: 4}
	st := &engine.WordState{}
	rs := game.NewRunScore(cfg)
	rs.StartManche(0, engine.NoBoss, false)
	rs.Apply(engine.Attempt{WordID: "chat", Correct: true}, w, st, nil, nil) // combo -> 2

	chips, mult := rs.Apply(engine.Attempt{WordID: "chat", Correct: false}, w, st, nil, nil)

	if chips != 0 || mult != 0 {
		t.Errorf("Apply on a miss = %v, %v, want 0, 0", chips, mult)
	}
	if got := rs.Combo(); got != 1 {
		t.Errorf("Combo() after a miss = %v, want 1 (a miss resets it)", got)
	}
}

func TestRunScoreCarriesTheComboAcrossManches(t *testing.T) {
	cfg := engine.DefaultConfig()
	w := engine.Word{ID: "chat", Text: "chat", Letters: 4}
	st := &engine.WordState{}
	rs := game.NewRunScore(cfg)

	rs.StartManche(0, engine.NoBoss, false)
	rs.Apply(engine.Attempt{WordID: "chat", Correct: true}, w, st, nil, nil)
	comboAfterManche0 := rs.Combo()

	rs.StartManche(1, engine.NoBoss, false)
	if got := rs.Combo(); got != comboAfterManche0 {
		t.Errorf("Combo() right after StartManche(1, …) = %v, want %v carried from manche 0", got, comboAfterManche0)
	}
}
