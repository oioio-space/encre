package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

func TestIsRencontre(t *testing.T) {
	tests := []struct {
		name string
		st   *engine.WordState
		want bool
	}{
		{"never recorded at all", nil, true},
		{"recorded but never seen", &engine.WordState{}, true},
		{"already seen", &engine.WordState{Seen: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := game.IsRencontre(tt.st); got != tt.want {
				t.Errorf("IsRencontre(%+v) = %v, want %v", tt.st, got, tt.want)
			}
		})
	}
}

func TestNewRencontreAttemptIsAlwaysCorrectAndMarkedAsACopy(t *testing.T) {
	a := game.NewRencontreAttempt("mot", 1, 4200)

	if !a.Correct {
		t.Error("NewRencontreAttempt().Correct = false, want true: a Rencontre admits no failure")
	}
	if !a.Copy {
		t.Error("NewRencontreAttempt().Copy = false, want true")
	}
	if a.WordID != "mot" || a.Manche != 1 || a.Millis != 4200 {
		t.Errorf("NewRencontreAttempt() = %+v, want the fields it was given carried through", a)
	}
}

// TestRencontreLeavesTheComboUntouched pins brief/ENCRE_01 §4's "combo
// inchangé": a Rencontre neither breaks the combo, the way a wrong answer
// does, nor advances it, the way a correct one otherwise would.
func TestRencontreLeavesTheComboUntouched(t *testing.T) {
	cfg := engine.DefaultConfig()
	score := game.NewRunScore(cfg)
	score.StartManche(0, engine.NoBoss, false)
	w := engine.Word{ID: "mot", Letters: 3}
	st := &engine.WordState{}

	before := score.Combo()
	score.Apply(game.NewRencontreAttempt("mot", 0, 3000), w, st, nil, nil)
	after := score.Combo()

	if after != before {
		t.Errorf("combo after a Rencontre = %v, want unchanged from %v", after, before)
	}
}
