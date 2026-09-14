package sim_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/sim"
)

// Cohort is the hundred children of brief/ENCRE_05 ticket T08.
const Cohort = 100

// v1Talismans are the eight Talismans of V1 (ENCRE_01 §12), which is what
// TestEveryV1TalismanPaysItsWay asks the cohort about.
var v1Talismans = []engine.TalismanID{
	engine.Perroquet, engine.Chronometre, engine.Jumeau, engine.Loupe,
	engine.Gomme, engine.Collectionneur, engine.Aimant, engine.Sourd,
}

func TestACohortSurvivesTheSchoolYear(t *testing.T) {
	// The acceptance of T08, and the only test here that is about the GAME
	// rather than about the code: nine versions of the original simulation were
	// needed to get these three numbers, and the first eight killed every child
	// before week ten. If a change to the rules breaks one of them, the build
	// fails now instead of a seven-year-old discovering it in March.
	cfg := engine.DefaultConfig()

	start := time.Now()
	res := sim.Run(Cohort, 2026, cfg)
	elapsed := time.Since(start)

	t.Logf("rétention %.0f%% · M1 %.1f%% · M2 %.1f%% · boss %.1f%% · arrêts %d frustration / %d ennui · "+
		"au-dessus de Blanc %.0f%% · mots justes %.0f%% (%.0f/%.0f/%.0f) · combo max médian %v · %v",
		res.Retention(Cohort)*100, res.FailRate[0]*100, res.FailRate[1]*100, res.FailRate[2]*100,
		res.QuitFrustrated, res.QuitBored, res.AboveBlanc(Cohort)*100, res.CorrectRate*100,
		res.CorrectRateByManche[0]*100, res.CorrectRateByManche[1]*100, res.CorrectRateByManche[2]*100,
		res.ComboMaxMedian, elapsed.Round(time.Millisecond))

	if got := res.FailRate[0]; got > 0.04 {
		t.Errorf("first manche lost %.1f%% of the time, want at most 4%% — it is meant to be the one that reassures", got*100)
	}
	if got := res.FailRate[2]; got < 0.15 || got > 0.30 {
		t.Errorf("the boss was lost %.1f%% of the time, want between 15%% and 30%%", got*100)
	}
	if got := res.Retention(Cohort); got < 0.60 {
		t.Errorf("retention at week %d = %.0f%%, want at least 60%%", sim.Weeks, got*100)
	}
	if elapsed > 10*time.Second {
		t.Errorf("the cohort took %v, want under 10s — it runs on every build", elapsed)
	}

	// encre-00q.1: WeeksAtRank was never advanced anywhere in production, so a
	// hundred children out of a hundred stayed Blanc for the whole year. Half
	// the cohort climbing at least one rank by week 36 is what proves the fix
	// rather than merely the counter moving.
	if got := res.AboveBlanc(Cohort); got < 0.50 {
		t.Errorf("above Blanc at week %d = %.0f%%, want at least 50%%", sim.Weeks, got*100)
	}
}

// TestEveryV1TalismanPaysItsWay is encre-00q.2's acceptance criterion: a
// Talisman the shop sells and that never once changes a score is a Talisman
// that does nothing, whatever score.go's rules say it should do.
func TestEveryV1TalismanPaysItsWay(t *testing.T) {
	cfg := engine.DefaultConfig()
	res := sim.Run(Cohort, 2026, cfg)

	for _, id := range v1Talismans {
		carried := res.TalismanCarried[id]
		paid := res.TalismanPaid[id]
		t.Logf("%v: carried %d, paid %d", id, carried, paid)
		if carried == 0 {
			t.Errorf("%v was never carried by a run; the shop cannot be selling it", id)
			continue
		}
		if rate := float64(paid) / float64(carried); rate < 0.01 {
			t.Errorf("%v paid %d times over %d runs carrying it (%.1f%%), want at least 1%%",
				id, paid, carried, rate*100)
		}
	}
}

func TestTheSameSeedGivesTheSameYear(t *testing.T) {
	// Without this the thresholds above would be a coin toss, and a red build
	// would tell nobody anything.
	cfg := engine.DefaultConfig()

	first := sim.Run(20, 7, cfg)
	again := sim.Run(20, 7, cfg)

	if !reflect.DeepEqual(first, again) {
		t.Errorf("the same seed gave %+v then %+v", first, again)
	}
}

// TestTheShopOnlySellsV1Talismans is a light check that the simulation's own
// shop function never hands back anything but V1's eight, so
// TestEveryV1TalismanPaysItsWay above is not silently scoring a Talisman that
// was never for sale.
func TestTheShopOnlySellsV1Talismans(t *testing.T) {
	cfg := engine.DefaultConfig()
	res := sim.Run(Cohort, 2026, cfg)

	for id := range res.TalismanCarried {
		if !slices.Contains(v1Talismans, id) {
			t.Errorf("TalismanCarried names %v, which is not one of V1's eight", id)
		}
	}
}
