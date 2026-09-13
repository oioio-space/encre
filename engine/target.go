package engine

// RollingRate is a success rate kept over a sliding window of fresh words. It
// is what tells a rank-up from a lucky fortnight.
type RollingRate struct {
	Wins, Total int
}

// Rate returns the share of successes, or 0 before anything was tried.
func (r RollingRate) Rate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Wins) / float64(r.Total)
}

// Child is one player's standing (ENCRE_04 §4).
type Child struct {
	Rank, BestRank, Prestige                    int
	BossWinsAtRank, WeeksAtRank, BossFailStreak int
	// Kindness lowers the targets after runs lost early, down to
	// Config.KindnessFloor, and returns to 1 on a win. Its zero value is not
	// usable; NewChild sets it.
	Kindness float64
	// Skill is the child's general spelling level, 0 to 1.
	Skill float64
	// Level is the level reached in each Couleur, and Aff the affinity — a
	// child who finds the Muettes easy carries a positive one.
	Level map[Color]int
	Aff   map[Color]float64
	// LearnRate is how fast mastery moves on a success.
	LearnRate     float64
	XP            map[Color]float64
	LevelUpW      map[Color]int32
	Base          RollingRate
	Unlocked      []TalismanID
	BossWinsTotal int
}

// LoseEarly records a run lost early: the targets come down one step of
// Bienveillance, no further than the floor.
//
// The simulation is why this exists. Without it the middling child never
// recovers from a bad fortnight and stops playing — the single change that took
// retention from nothing to most of the class.
func (c *Child) LoseEarly(cfg Config) {
	c.Kindness = max(c.Kindness-cfg.KindnessStep, cfg.KindnessFloor)
}

// Win clears the Bienveillance: a child who beat the targets does not need
// them lowered.
func (c *Child) Win() { c.Kindness = 1 }

// Deck is the set of words a run is played from (ENCRE_04 §4).
type Deck struct {
	Week   []Word
	Garde  []Word
	Old    []Word
	Cursed []Word
	Rooms  [2][2]RoomID
	Boss   BossID
	Seed   int64
}

// RoomID names a room offered between two manches.
type RoomID string

// BossID names the boss of a week.
type BossID string

// ruleDifficulty is how hard one trap of each Couleur is, calibrated by the
// balance simulation rather than chosen: the Sosies and the Accordées cost
// nearly three times a single accent.
var ruleDifficulty = map[Color]float64{
	Muettes:    0.16,
	Jumelles:   0.12,
	Accentuees: 0.07,
	Masquees:   0.17,
	Sosies:     0.20,
	Accordees:  0.20,
}

// Bounds on the estimate. Zero would tell a child not to try and one would
// promise a word they can still miss, so neither is ever shown.
const (
	pHatFloor   = 0.03
	pHatCeiling = 0.98
)

// PHat estimates the chance this child spells this word right under these
// conditions (ENCRE_03 §7).
//
// It is not a scoring input: it draws the dots on the card, so the child can
// see what is being asked, and it sets the targets so that a deck of hard words
// does not carry an impossible one. The server recomputes it; nothing the
// client says about it is trusted.
func PHat(c *Child, w Word, st *WordState, ctx Ctx) float64 {
	difficulty := 0.0
	for colour, traps := range w.Traps {
		if traps == 0 {
			continue
		}
		// A level in a Couleur takes up to 40% off its traps, spread over the
		// ten levels.
		levelRelief := 1 - 0.4*float64(c.Level[colour]-1)/9.0
		difficulty += float64(traps) * (ruleDifficulty[colour]*levelRelief - c.Aff[colour])
	}

	// A word already mastered is nearly safe whatever its traps; a fresh one
	// falls back on the child's general skill.
	p := st.Mastery*0.93 + (1-st.Mastery)*(c.Skill-difficulty)

	// Every handicap costs less on a word the child already knows, which is
	// why each is scaled by what is left to learn.
	if ctx.Listens == 1 {
		p -= 0.10 * (1 - st.Mastery)
	}
	if ctx.Sentence {
		p -= 0.05
	}
	if ctx.Blind {
		p -= 0.06 * (1 - st.Mastery)
	}
	switch ctx.Boss {
	case Presse:
		p -= 0.05
	case Brouillon:
		p -= 0.10 * (1 - st.Mastery)
	case Chuchoteur:
		// The Chuchoteur takes a listen away, which Ctx.Listens already says.
	case VoleurDAccents, NoBoss:
		// Neither touches the estimate: the Voleur works on the chips.
	}
	return min(max(p, pHatFloor), pHatCeiling)
}

// Targets returns the three the run has to beat, one per manche.
//
// They are computed from the deck the child is actually holding — its words at
// their levels, weighted by the chance of spelling each — rather than from a
// table. A fixed target punishes a hard week and gives away an easy one; the
// simulation found absolute targets killed the middling child outright.
//
// It takes no Talismans, and that is the point: ENCRE_04 §4 requires the
// targets never to depend on them. Were they an argument, a child buying one
// would raise their own target, and the shop would punish them for using it.
func Targets(c *Child, d Deck, states map[string]*WordState, cfg Config) [3]float64 {
	words := d.Week
	if len(words) == 0 {
		return [3]float64{}
	}

	// The chips are taken with no Talismans and at a combo of one — the bare
	// worth of the deck — then weighted by the chance the word is spelled at
	// all, so an unknown word does not set a target the child cannot reach.
	potential := 0.0
	for _, w := range words {
		st := states[w.ID]
		if st == nil {
			st = &WordState{}
		}
		ctx := Ctx{Levels: c.Level, Listens: 2, Boss: NoBoss}
		chips := scoreChips(Attempt{Correct: true}, w, st, nil, ctx, cfg)
		potential += chips * PHat(c, w, st, ctx)
	}
	average := potential / float64(len(words))

	var t [3]float64
	for i := range t {
		t[i] = average * cfg.K[i] * cfg.RankMult[c.Rank] * c.Kindness
	}
	return t
}
