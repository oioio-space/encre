package e2e

import (
	"net/http"
	"testing"

	"pgregory.net/rapid"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

// wordsForScoreTest are ten real French words carrying the accents and traps
// brief/ENCRE_03 §4 teaches (see cmd/client/main.go's own list, which draws
// from the same brief) — enough variety that [engine.Score] takes more than
// one code path across the attempts rapid.Check throws at it.
var wordsForScoreTest = []string{
	"école", "bébé", "mère", "forêt", "garçon",
	"français", "tête", "cœur", "flûte", "hôpital",
}

// genAttempt draws one attempt on a word actually in the deck, in manche.
// [engine.Replay] only checks that a.WordID is somewhere in the deck (see
// its byID map, built across Week, Garde, Old and Cursed) — never that it
// belongs to the manche it is attempted in — so any deck word is legal for
// any manche.
func genAttempt(t *rapid.T, wordIDs []string, manche int) engine.Attempt {
	return engine.Attempt{
		WordID:  rapid.SampledFrom(wordIDs).Draw(t, "word"),
		Manche:  manche,
		Blind:   rapid.Bool().Draw(t, "blind"),
		Copy:    rapid.IntRange(0, 5).Draw(t, "copy-roll") == 0,
		Correct: rapid.IntRange(0, 4).Draw(t, "correct-roll") != 0,
		Millis:  rapid.IntRange(0, 20_000).Draw(t, "millis"),
	}
}

// TestFinishHandlerMatchesClientScoreToTheToken is the round trip
// bead encre-qpx.1 asks for: client/game/score_test.go already proves
// [game.RunScore] agrees with [engine.Replay] in isolation
// (TestRunScoreMatchesEngineReplayToTheToken); this proves the same thing
// through the real POST /run/{id}/finish handler, against a deck the real
// handler built from real store data — the seam a unit test calling engine
// or client/game directly cannot see, because a desynchronised
// [engine.Config] between the two binaries, or a handler that quietly
// substituted its own score, would still agree with itself in either half
// taken alone.
//
// It generates 50 runs with rapid (the acceptance criterion's own number),
// each against a fresh run/start so a run applied twice is never in play
// here — TestRunFinishAppliedOnlyOnce in server/api/run_test.go already
// guards that — and checks every manche's score, float64 bit for bit.
func TestFinishHandlerMatchesClientScoreToTheToken(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedList(t, ts.DB, "child1", wordsForScoreTest...)
	loginChild(t, ts, "Mia", "1379")

	cfg := engine.DefaultConfig()
	// engine.BuildDeck picks the week's boss as bosses[weekNo%len(bosses)]
	// (engine/deck.go) — a table this package cannot import (bosses is
	// unexported) but whose order is exactly the Boss iota order
	// (engine/score.go: NoBoss, Chuchoteur, VoleurDAccents, Brouillon,
	// Presse), so weekNo%4 cast straight to engine.Boss lands on the same
	// entry. server/api/time.go's weekOf is the same division.
	const secondsPerWeek = 7 * 24 * 60 * 60
	weekNo := ts.Now().Unix() / secondsPerWeek
	boss := engine.Boss(weekNo % 4)

	runs := 0
	rapid.Check(t, func(rt *rapid.T) {
		_, run := startRun(t, ts, nil)
		if run.RunID == "" {
			rt.Fatal("run/start did not return a RunID")
		}
		wordIDs := make([]string, len(run.Deck.Week))
		for i, w := range run.Deck.Week {
			wordIDs[i] = w.ID
		}

		var attempts []engine.Attempt
		for manche := range 3 {
			for range rapid.IntRange(1, 6).Draw(rt, "attempt-count") {
				attempts = append(attempts, genAttempt(rt, wordIDs, manche))
			}
		}

		// The handler's own engine.Apply mutates each word's persisted state
		// (Gold, Seen, SuccessDays…) after every finish this loop drives, so
		// a later trial's words are no longer blank the way score_test.go's
		// randomState ones start out. Reading the store's own snapshot right
		// before finish — the same one handleRunFinish's WordStates call
		// will read — is what keeps the client side of this comparison
		// honest about what state each attempt actually landed on.
		states, err := ts.DB.WordStates(t.Context(), "child1")
		if err != nil {
			rt.Fatalf("WordStates() error = %v", err)
		}

		resp := ts.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts, [3]bool{}))
		if resp.StatusCode != http.StatusOK {
			rt.Fatalf("run/finish status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		got := decodeBody[finishRunResponse](t, resp)

		byID := map[string]engine.Word{}
		for _, w := range run.Deck.Week {
			byID[w.ID] = w
		}
		rs := game.NewRunScore(cfg)
		for manche := range 3 {
			mancheBoss := engine.NoBoss
			if manche == 2 {
				mancheBoss = boss
			}
			rs.StartManche(manche, mancheBoss, false)
			for _, a := range attempts {
				if a.Manche != manche {
					continue
				}
				w := byID[a.WordID]
				st := states[a.WordID]
				if st == nil {
					st = &engine.WordState{}
					states[a.WordID] = st
				}
				rs.Apply(a, w, st, nil, run.Levels)
			}
			// engine.Replay stops recomputing the moment a manche misses its
			// target (engine/replay.go: "out.FailedAt = manche; return out,
			// nil") rather than scoring the rest of the deck it will never
			// show the child. RunScore has no opinion on that — it only
			// tallies what Apply is called with — so this mirrors the same
			// rule to know when to stop comparing, exactly as a real scene
			// deciding whether to open the next manche would have to.
			if rs.Total(manche) < run.Targets[manche] {
				break
			}
		}

		for manche := range 3 {
			if got.Outcome.Scores[manche] != rs.Total(manche) {
				rt.Fatalf("manche %d: server Outcome.Scores = %v, client RunScore.Total = %v (run %s)",
					manche, got.Outcome.Scores[manche], rs.Total(manche), run.RunID)
			}
		}
		runs++
	})

	// rapid.Check itself halves its check count under -short (the flag
	// mise.toml's test:ci task runs with), so the acceptance criterion's
	// own "50 runs" is only enforced at full strength.
	if want := 50; !testing.Short() && runs < want {
		t.Errorf("ran %d trials, want at least %d (the acceptance criterion's own number)", runs, want)
	}
}
