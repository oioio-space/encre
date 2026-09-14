package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

func TestRevancheEligible(t *testing.T) {
	cfg := engine.DefaultConfig() // RevancheWindow: 0.85

	tests := []struct {
		name          string
		score, target float64
		want          bool
	}{
		{"met the target", 100, 100, false},
		{"beat the target", 120, 100, false},
		{"exactly at the window", 85, 100, true},
		{"inside the window", 90, 100, true},
		{"just under the window", 84.9, 100, false},
		{"far short", 10, 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := game.RevancheEligible(tt.score, tt.target, cfg); got != tt.want {
				t.Errorf("RevancheEligible(%v, %v, cfg) = %v, want %v", tt.score, tt.target, got, tt.want)
			}
		})
	}
}

func TestMissedPoints(t *testing.T) {
	tests := []struct {
		name          string
		score, target float64
		want          int
	}{
		{"the brief's own example", 100 - 87, 100, 87},
		{"met the target", 100, 100, 0},
		{"beat the target", 130, 100, 0},
		{"rounds to nearest", 90.6, 100, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := game.MissedPoints(tt.score, tt.target); got != tt.want {
				t.Errorf("MissedPoints(%v, %v) = %v, want %v", tt.score, tt.target, got, tt.want)
			}
		})
	}
}

func TestRevancheText(t *testing.T) {
	if got, want := game.RevancheText(13, 100), "Il manquait 87 points."; got != want {
		t.Errorf("RevancheText(13, 100) = %q, want %q", got, want)
	}
}

// TestRevancheEligibleImpliesEngineReplayAcceptsTheClaim pins one direction
// of RevancheEligible against [engine.Replay]'s own check: when this reports
// a manche eligible, claiming the Revanche on it must not be refused.
//
// It is one direction only, not a full agreement: [engine.Replay] checks the
// window against the score already carrying the Revanche's own +1 Mult
// (brief/ENCRE_01 §10), which only ever raises a score, never lowers it — so
// a manche eligible on its original score stays eligible once boosted, but a
// manche this reports ineligible might still cross the window once boosted,
// and Replay is the one entitled to decide that, not this test.
func TestRevancheEligibleImpliesEngineReplayAcceptsTheClaim(t *testing.T) {
	cfg := engine.DefaultConfig()
	w := engine.Word{ID: "mot", Letters: 10}
	deck := engine.Deck{Week: []engine.Word{w}}

	for _, letters := range []int{50, 84, 85, 90, 99} {
		target := 100.0
		w.Letters = letters
		deck.Week[0] = w
		attempt := engine.Attempt{WordID: "mot", Manche: 0, Correct: true}
		// Only manche 0 carries an attempt; the other two must not also fail
		// and mask what manche 0's own Revanche claim decided, so their
		// targets are zero — met by a manche that scores nothing.
		baseRun := engine.Run{
			Deck: deck, Targets: [3]float64{target, 0, 0},
			Levels: map[engine.Color]int{}, Attempts: []engine.Attempt{attempt},
		}

		// The score as it failed, with no Revanche bonus yet — what a screen
		// deciding whether to offer the Revanche button would see.
		asFailed, err := engine.Replay(baseRun, map[string]*engine.WordState{}, cfg)
		if err != nil {
			t.Fatalf("letters %d: Replay without a Revanche claim: %v", letters, err)
		}
		if !game.RevancheEligible(asFailed.Scores[0], target, cfg) {
			continue // nothing this test can assert either way; see the doc comment.
		}

		claimed := baseRun
		claimed.Revanche = [3]bool{true}
		if _, err := engine.Replay(claimed, map[string]*engine.WordState{}, cfg); err != nil {
			t.Errorf(
				"letters %d: score %.1f was RevancheEligible, but engine.Replay refused the claim: %v",
				letters, asFailed.Scores[0], err,
			)
		}
	}
}
