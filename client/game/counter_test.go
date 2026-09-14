package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/game"
)

func TestCounterDurationIsBoundedAndGrowsWithLogOfTheScore(t *testing.T) {
	j := anim.DefaultJuice()

	if got := game.CounterDuration(0, 1000, j); got != j.CounterMinDuration.Duration() {
		t.Errorf("CounterDuration(0, …) = %v, want the floor %v", got, j.CounterMinDuration.Duration())
	}
	if got := game.CounterDuration(1, 1000, j); got != j.CounterMinDuration.Duration() {
		t.Errorf("CounterDuration(1, …) = %v, want the floor %v (log(1) = 0)", got, j.CounterMinDuration.Duration())
	}
	if got := game.CounterDuration(1000, 1000, j); got != j.CounterMaxDuration.Duration() {
		t.Errorf("CounterDuration(ceiling, ceiling) = %v, want the cap %v", got, j.CounterMaxDuration.Duration())
	}
	if got := game.CounterDuration(1_000_000, 1000, j); got != j.CounterMaxDuration.Duration() {
		t.Errorf("CounterDuration above the ceiling = %v, want it still capped at %v", got, j.CounterMaxDuration.Duration())
	}

	small := game.CounterDuration(10, 1000, j)
	big := game.CounterDuration(500, 1000, j)
	if small >= big {
		t.Errorf("CounterDuration(10) = %v, CounterDuration(500) = %v, want the second strictly longer", small, big)
	}
}
