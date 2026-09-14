package engine

// Rank events.
const (
	RankUp   Event = "RankUp"
	RankDown Event = "RankDown"
	LevelUp  Event = "LevelUp"
)

// maxRank is Diamant, the sixth and last (ENCRE_01).
const maxRank = 5

// AdvanceWeek moves the child one week further under their current rank.
//
// It exists because WeeksAtRank has no other honest way to move: Apply runs
// once per finished run, and a session plays several, so incrementing it
// there would count a good evening as several weeks. The caller — today
// [sim.Run], eventually the server's weekly job — calls this exactly once per
// child per week, whether or not they played at all.
func (c *Child) AdvanceWeek() {
	c.WeeksAtRank++
}

// WinBoss records a boss beaten in the given week and returns what changed.
//
// A rank asks for two things at once: enough wins AND enough weeks under the
// current one. The second is what stops a good fortnight carrying a child past
// what they can actually spell — and only one rank is won per week, so two runs
// in an evening cannot move it twice.
func (c *Child) WinBoss(week int32, cfg Config) []Event {
	c.BossFailStreak = 0
	c.BossWinsAtRank++
	c.BossWinsTotal++

	ready := c.BossWinsAtRank >= cfg.RankUpWins &&
		c.WeeksAtRank >= cfg.RankUpMinWeeks &&
		c.Rank < maxRank &&
		week > c.LastRankW
	if !ready {
		return nil
	}
	c.Rank++
	c.BossWinsAtRank, c.WeeksAtRank, c.LastRankW = 0, 0, week
	c.BestRank = max(c.BestRank, c.Rank)
	return []Event{RankUp}
}

// LoseBoss records a boss failed and returns what changed.
//
// The descent exists because the simulation needed it: with no way down every
// child plateaued and left out of boredom. A single win breaks the run, so it
// takes a real slump rather than a bad evening.
func (c *Child) LoseBoss(cfg Config) []Event {
	c.BossFailStreak++
	if c.BossFailStreak < cfg.RankDownFails || c.Rank == 0 {
		return nil
	}
	c.Rank--
	c.BossWinsAtRank, c.WeeksAtRank, c.BossFailStreak = 0, 0, 0
	return []Event{RankDown}
}

// GainXP awards the experience one attempt is worth and returns any level won.
//
// Only a blind word or a boss manche pays. Levelling is meant to come from the
// harder ways of playing, not from repeating an easy manche until it counts.
func (c *Child) GainXP(w Word, a Attempt, ctx Ctx, week int32, cfg Config) []Event {
	if !a.Correct || (!a.Blind && ctx.Boss == NoBoss) {
		return nil
	}
	if c.XP == nil {
		c.XP = map[Color]float64{}
	}
	if c.NextLevelW == nil {
		c.NextLevelW = map[Color]int32{}
	}

	var events []Event
	for colour, traps := range w.Traps {
		if traps == 0 {
			continue
		}
		c.XP[colour]++

		// A level costs twelve times the level it leaves, so the tenth is not
		// the first repeated ten times.
		need := cfg.XPPerLevel * float64(c.Level[colour])
		if c.XP[colour] < need || c.Level[colour] >= cfg.LevelMax || week < c.NextLevelW[colour] {
			continue
		}
		c.XP[colour] -= need
		c.Level[colour]++
		c.NextLevelW[colour] = week + cfg.LevelCooldownW
		events = append(events, LevelUp)
	}
	return events
}
