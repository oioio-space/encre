package engine_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
)

// winBosses beats n bosses, one per week starting at the given week.
func winBosses(c *engine.Child, n int, from int32, cfg engine.Config) []engine.Event {
	var events []engine.Event
	for i := range n {
		c.WeeksAtRank++
		events = append(events, c.WinBoss(from+int32(i), cfg)...)
	}
	return events
}

func TestARankNeedsBothEnoughWinsAndEnoughWeeks(t *testing.T) {
	// ENCRE_01 asks for both, and the second is what stops a good fortnight
	// carrying a child past what they can actually spell.
	cfg := engine.DefaultConfig()

	quick := newChild()
	quick.WeeksAtRank = 0
	for i := range cfg.RankUpWins {
		quick.WinBoss(int32(i), cfg) // every win in its own week, but WeeksAtRank stays low
	}
	if quick.Rank != 0 {
		t.Errorf("rank after %d wins in too few weeks = %d, want 0", cfg.RankUpWins, quick.Rank)
	}

	patient := newChild()
	events := winBosses(patient, cfg.RankUpWins, 0, cfg)
	if patient.Rank != 1 {
		t.Errorf("rank after %d wins over as many weeks = %d, want 1", cfg.RankUpWins, patient.Rank)
	}
	if !slices.Contains(events, engine.RankUp) {
		t.Errorf("events = %v, want RankUp among them", events)
	}
}

func TestOnlyOneRankIsWonPerWeek(t *testing.T) {
	// Two runs in one evening must not move the rank twice.
	cfg := engine.DefaultConfig()
	c := newChild()
	winBosses(c, cfg.RankUpWins, 0, cfg)

	before := c.Rank
	c.WeeksAtRank = 99
	for range cfg.RankUpWins {
		c.WinBoss(int32(cfg.RankUpWins)-1, cfg) // all in the week of the last rank-up
	}

	if c.Rank != before {
		t.Errorf("rank moved to %d inside the week it was won, want it to stay at %d", c.Rank, before)
	}
}

func TestThreeFailedBossesDropARank(t *testing.T) {
	// The descent is what the simulation needed: without it everyone plateaus
	// and leaves out of boredom.
	cfg := engine.DefaultConfig()
	c := newChild()
	c.Rank = 3

	var events []engine.Event
	for range cfg.RankDownFails {
		events = append(events, c.LoseBoss(cfg)...)
	}

	if c.Rank != 2 {
		t.Errorf("rank after %d failed bosses = %d, want 2", cfg.RankDownFails, c.Rank)
	}
	if !slices.Contains(events, engine.RankDown) {
		t.Errorf("events = %v, want RankDown among them", events)
	}
}

func TestAWinClearsTheRunOfFailures(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()
	c.Rank = 2

	c.LoseBoss(cfg)
	c.LoseBoss(cfg)
	c.WinBoss(0, cfg)
	c.LoseBoss(cfg)
	c.LoseBoss(cfg)

	if c.Rank != 2 {
		t.Errorf("rank = %d, want 2 — the win broke the run of failures", c.Rank)
	}
}

func TestTheRankNeverFallsBelowBlanc(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()

	for range 10 {
		c.LoseBoss(cfg)
	}

	if c.Rank != 0 {
		t.Errorf("rank = %d, want 0 — Blanc is the floor", c.Rank)
	}
}

func TestBestRankRemembersTheHighestReached(t *testing.T) {
	// The atelier is furnished by the best rank ever held, so losing one does
	// not empty the room.
	cfg := engine.DefaultConfig()
	c := newChild()
	winBosses(c, cfg.RankUpWins, 0, cfg)
	for range cfg.RankDownFails {
		c.LoseBoss(cfg)
	}

	if c.BestRank != 1 {
		t.Errorf("BestRank = %d, want 1", c.BestRank)
	}
}

func TestOnlyBlindWordsAndBossManchesEarnXP(t *testing.T) {
	// ENCRE_01: levelling comes from the harder ways of playing, not from
	// repeating an easy manche.
	cfg := engine.DefaultConfig()
	w := engine.Word{Traps: map[engine.Color]int{engine.Muettes: 1}}

	tests := []struct {
		name string
		a    engine.Attempt
		ctx  engine.Ctx
		want float64
	}{
		{name: "an ordinary correct word", a: engine.Attempt{Correct: true}, ctx: engine.Ctx{Boss: engine.NoBoss}},
		{name: "a blind word", a: engine.Attempt{Correct: true, Blind: true}, ctx: engine.Ctx{Boss: engine.NoBoss}, want: 1},
		{name: "a word in a boss manche", a: engine.Attempt{Correct: true}, ctx: engine.Ctx{Boss: engine.Presse}, want: 1},
		{name: "a blind word missed", a: engine.Attempt{Blind: true}, ctx: engine.Ctx{Boss: engine.NoBoss}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChild()
			c.GainXP(w, tt.a, tt.ctx, 0, cfg)
			if got := c.XP[engine.Muettes]; got != tt.want {
				t.Errorf("XP = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestALevelCostsTwelveTimesTheLevelAndWaitsThreeWeeks(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()
	w := engine.Word{Traps: map[engine.Color]int{engine.Muettes: 1}}
	blind := engine.Attempt{Correct: true, Blind: true}

	// Level 1 costs twelve.
	var events []engine.Event
	for range int(cfg.XPPerLevel) {
		events = append(events, c.GainXP(w, blind, engine.Ctx{Boss: engine.NoBoss}, 0, cfg)...)
	}
	if c.Level[engine.Muettes] != 2 {
		t.Fatalf("level after %v XP = %d, want 2", cfg.XPPerLevel, c.Level[engine.Muettes])
	}
	if !slices.Contains(events, engine.LevelUp) {
		t.Errorf("events = %v, want LevelUp among them", events)
	}

	// Level 2 costs twenty-four, and not inside the cooldown.
	for range 2 * int(cfg.XPPerLevel) {
		c.GainXP(w, blind, engine.Ctx{Boss: engine.NoBoss}, 1, cfg)
	}
	if c.Level[engine.Muettes] != 2 {
		t.Errorf("level = %d one week later, want 2 — the cooldown is %d weeks",
			c.Level[engine.Muettes], cfg.LevelCooldownW)
	}

	c.GainXP(w, blind, engine.Ctx{Boss: engine.NoBoss}, cfg.LevelCooldownW, cfg)
	if c.Level[engine.Muettes] != 3 {
		t.Errorf("level after the cooldown = %d, want 3", c.Level[engine.Muettes])
	}
}

func TestALevelStopsAtTen(t *testing.T) {
	cfg := engine.DefaultConfig()
	c := newChild()
	c.Level[engine.Muettes] = cfg.LevelMax
	w := engine.Word{Traps: map[engine.Color]int{engine.Muettes: 1}}

	for i := range 500 {
		c.GainXP(w, engine.Attempt{Correct: true, Blind: true}, engine.Ctx{Boss: engine.NoBoss}, int32(i), cfg)
	}

	if got := c.Level[engine.Muettes]; got != cfg.LevelMax {
		t.Errorf("level = %d, want it capped at %d", got, cfg.LevelMax)
	}
}
