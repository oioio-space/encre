package game

import (
	"math"
	"time"

	"github.com/oioio-space/encre/client/anim"
)

// CounterDuration is how long the score counter takes to count up chips
// tokens, following log(chips) between j.CounterMinDuration and
// j.CounterMaxDuration (brief/ENCRE_02 §12, restated by ENCRE_07 §4.1's
// correction over the fixed 1.8 s of ENCRE_06 §6's own storyboard demo).
//
// ceiling is the score that saturates the duration at its cap — the caller's
// own reference for "a big jump", such as the manche's target — so the
// formula carries no score magnitude of its own: every number in it comes
// from juice.json or from what the caller already knows about the run.
//
// chips or ceiling at or under one returns the floor: log(1) is zero, and
// nothing here divides by log(1).
func CounterDuration(chips, ceiling float64, j anim.Juice) time.Duration {
	lo, hi := j.CounterMinDuration.Duration(), j.CounterMaxDuration.Duration()
	if chips <= 1 || ceiling <= 1 {
		return lo
	}
	t := math.Log(chips) / math.Log(ceiling)
	t = min(max(t, 0), 1)
	return lo + time.Duration(t*float64(hi-lo))
}
