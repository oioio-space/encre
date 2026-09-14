package game

import (
	"fmt"
	"math"

	"github.com/oioio-space/encre/engine"
)

// RevancheEligible reports whether a manche scored score against target
// qualifies for a Revanche: short of the target, but inside cfg.
// RevancheWindow of it — the same window [engine.Replay] itself checks
// before letting a claimed Revanche stand (brief/ENCRE_01 §10: "moins de 15%
// de la cible").
//
// A manche that already met target is not eligible: there is nothing to
// replay for.
func RevancheEligible(score, target float64, cfg engine.Config) bool {
	return score < target && score >= target*cfg.RevancheWindow
}

// MissedPoints returns how far score fell short of target, rounded to the
// nearest whole point and never negative — "Il manquait N points." reads a
// count, never a fraction the child was not asked to add up, and a manche
// that met or beat its target missed nothing.
func MissedPoints(score, target float64) int {
	missed := target - score
	if missed <= 0 {
		return 0
	}
	return int(math.Round(missed))
}

// RevancheText is the line brief/ENCRE_01 §10 gives verbatim, "Il manquait N
// points.", built from [MissedPoints].
func RevancheText(score, target float64) string {
	return fmt.Sprintf("Il manquait %d points.", MissedPoints(score, target))
}
