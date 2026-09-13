package sim_test

import (
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/sim"
)

// Cohort is the hundred children of brief/ENCRE_05 ticket T08.
const Cohort = 100

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

	t.Logf("rétention %.0f%% · M1 %.1f%% · M2 %.1f%% · boss %.1f%% · arrêts %d frustration / %d ennui · %v",
		res.Retention(Cohort)*100, res.FailRate[0]*100, res.FailRate[1]*100,
		res.FailRate[2]*100, res.QuitFrustrated, res.QuitBored, elapsed.Round(time.Millisecond))

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
}

func TestTheSameSeedGivesTheSameYear(t *testing.T) {
	// Without this the thresholds above would be a coin toss, and a red build
	// would tell nobody anything.
	cfg := engine.DefaultConfig()

	first := sim.Run(20, 7, cfg)
	again := sim.Run(20, 7, cfg)

	if first != again {
		t.Errorf("the same seed gave %+v then %+v", first, again)
	}
}
