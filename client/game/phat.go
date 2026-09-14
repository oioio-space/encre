package game

import "math"

// pHatDots is the number of points p̂ is drawn as (brief/ENCRE_06 §5).
const pHatDots = 10

// PHatPoints converts p, the chance of spelling a word right that
// engine.PHat estimates, into filled and total points, never digits:
// brief/ENCRE_02 §12's rule is "pas de chiffres là où des points suffisent",
// and this is the one place p̂ reaches the screen.
//
// p is clamped to [0,1] before rounding to the nearest tenth, so a value
// outside engine.PHat's own [0.03, 0.98] range — which nothing here assumes —
// still returns a sane count rather than an out-of-range one.
func PHatPoints(p float64) (filled, total int) {
	p = min(max(p, 0), 1)
	return int(math.Round(p * pHatDots)), pHatDots
}
