package game

import "github.com/oioio-space/encre/engine"

// RunScore is the running score of a run in progress, shown on the run
// screen as it is played.
//
// ENCRE_04 §1 makes the server's replay of a run's attempts the score that
// counts (see [engine.Replay]); a client that reimplemented the rules to move
// its own counter early could silently drift from it, and a seven-year-old
// would watch the wrong number all game. RunScore never reimplements
// anything: [RunScore.Apply] calls [engine.Score] itself, kept in lockstep
// with the same combo, Revanche bonus, Gomme forgiveness and boss that
// engine.Replay's own manche loop carries — so the two stay identical by
// construction, attempt for attempt, rather than by luck (see
// TestRunScoreMatchesEngineReplayToTheToken).
//
// A caller applies one manche's attempts in order between a [RunScore.StartManche]
// naming that manche's boss and Revanche and the next call to StartManche,
// exactly as they happen at the table — which is also the only order
// engine.Replay itself ever sees them in.
//
// The zero value is not usable; call [NewRunScore].
type RunScore struct {
	cfg    engine.Config
	scores [3]float64
	// combo lives outside any one manche: it carries from one into the next,
	// so the boss (played in the third) is naturally faced at whatever
	// multiplier the first two earned (ENCRE_01 §6) — engine.Replay does the
	// same, which is why StartManche never resets it.
	combo float64

	manche        int
	boss          engine.Boss
	revancheBonus float64
	goldPlayed    int
	// gommeSpent is at most one forgiven fault per manche, spent by the first
	// miss once the Gomme is owned — engine.Replay's own rule.
	gommeSpent bool
}

// NewRunScore starts a run's score at zero on every manche, combo at one.
func NewRunScore(cfg engine.Config) *RunScore {
	return &RunScore{cfg: cfg, combo: 1}
}

// StartManche resets what belongs to one manche alone — which gold words have
// already been played, the Gomme's one forgiveness, and the Revanche bonus
// and boss this manche plays under — without touching the combo, which
// engine.Replay carries across the boundary instead of resetting.
func (r *RunScore) StartManche(manche int, boss engine.Boss, revanche bool) {
	r.manche = manche
	r.boss = boss
	r.goldPlayed = 0
	r.gommeSpent = false
	r.revancheBonus = 0
	if revanche {
		r.revancheBonus = 1
	}
}

// Apply scores one attempt on w, in state st, against levels, exactly as
// engine.Replay scores the same attempt at the same point of the manche
// [RunScore.StartManche] last opened: Fast is derived from a.Millis against
// cfg.ChronoSeconds the same way, GoldPlayed is RunScore's own running count
// rather than anything the caller supplies, and the combo advances, holds for
// one Gomme-forgiven miss, or resets to one on a miss, in that order of
// priority.
//
// It returns the chips and multiplier this attempt earned — 0, 0 for a wrong
// answer, since a broken combo carries no multiplier to apply — so a scene
// can drive its own droplets and flame off what just happened without
// recomputing it.
func (r *RunScore) Apply(
	a engine.Attempt, w engine.Word, st *engine.WordState, owned engine.Talismans, levels map[engine.Color]int,
) (chips, mult float64) {
	fast := a.Millis > 0 && float64(a.Millis) <= r.cfg.ChronoSeconds*1000
	ctx := engine.Ctx{Levels: levels, Boss: r.boss, GoldPlayed: r.goldPlayed, Fast: fast}
	if st.Gold {
		r.goldPlayed++
	}
	chips, mult = engine.Score(a, w, st, owned, r.combo+r.revancheBonus, ctx, r.cfg)
	r.scores[r.manche] += chips * mult

	switch {
	case a.Correct && !a.Copy:
		r.combo++
	case !a.Correct && owned[engine.Gomme] && !r.gommeSpent:
		r.gommeSpent = true
	case !a.Correct:
		r.combo = 1
	}
	return chips, mult
}

// Total returns manche's score so far, exactly as engine.Replay would report
// it for the same attempts applied in the same order.
func (r *RunScore) Total(manche int) float64 { return r.scores[manche] }

// Combo returns the combo carried into the next attempt — the multiplier a
// scene draws as the candle's flame (ENCRE_06 §6 `vacille`).
func (r *RunScore) Combo() float64 { return r.combo }
