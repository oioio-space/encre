// Package sim runs a cohort of simulated children through the real engine.
//
// It is the acceptance test of brief/ENCRE_05 ticket T08 and the descendant of
// brief/simulation_dictee.go, the throwaway that settled the balance before any
// of the game existed. The difference is the point: that one carried its own
// copy of the rules, this one drives [engine], so a change to the rules that
// makes the game unplayable fails the build instead of being discovered by a
// seven-year-old in March.
//
// What it models is only the child — how likely they are to spell a word, how
// fast they forget, how often they play, and when they give up. Everything
// else, from the chips to the rank, is the engine's answer.
package sim

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/oioio-space/encre/engine"
)

// Weeks is a school year, and WordsPerWeek what a parent puts in on Sunday.
const (
	Weeks        = 36
	WordsPerWeek = 10
	// manches is how many a run holds.
	manches = 3
)

// trapFrequency is how often a word carries a trap of each Couleur. Masquées
// dominate because they are the bulk of the CE1 year (ENCRE_03 §1).
var trapFrequency = map[engine.Color]float64{
	engine.Muettes: 0.35, engine.Jumelles: 0.25, engine.Accentuees: 0.45,
	engine.Masquees: 0.20, engine.Sosies: 0.10, engine.Accordees: 0.30,
}

// Child is a simulated player: the engine's Child plus the traits that decide
// how they answer and whether they keep coming back.
type Child struct {
	engine.Child

	// Forget is what a week of not meeting a word does to its mastery.
	Forget float64
	// Sessions is how many runs a week this child plays. It turned out to be
	// the strongest predictor of giving up — stronger than skill.
	Sessions int

	states map[string]*engine.WordState
	// knownByID is every word the child has met, so decks can bring them back.
	knownByID map[string]engine.Word
	// frustration is an exponential average of runs going badly.
	frustration float64
	runs        int
	// lastNew is the week something new last happened; boredom is measured
	// from it.
	lastNew int
	// QuitWeek is when the child stopped, or zero while they are still playing.
	QuitWeek int
	QuitWhy  string
}

// bossOf maps the week's boss to the modifier its manche runs under, so the
// simulation plays the boss the run actually drew.
func bossOf(id engine.BossID) engine.Boss {
	switch id {
	case "Chuchoteur":
		return engine.Chuchoteur
	case "Voleur d'accents":
		return engine.VoleurDAccents
	case "Brouillon":
		return engine.Brouillon
	default:
		return engine.Presse
	}
}

// Result is what a cohort came to.
type Result struct {
	Playing        int
	QuitFrustrated int
	QuitBored      int
	// FailRate is the share of manches lost, per manche.
	FailRate [3]float64
}

// Retention is the share of the cohort still playing at the end.
func (r Result) Retention(cohort int) float64 { return float64(r.Playing) / float64(cohort) }

// NewChild draws a child from the spread the original simulation used.
func NewChild(rng *rand.Rand) *Child {
	c := &Child{
		Forget:    0.88 + rng.Float64()*0.09,
		Sessions:  []int{1, 2, 2, 3, 3, 3, 4, 4, 5, 6}[rng.IntN(10)],
		states:    map[string]*engine.WordState{},
		knownByID: map[string]engine.Word{},
	}
	c.Skill = 0.5 + rng.Float64()*0.4
	c.LearnRate = 0.12 + rng.Float64()*0.18
	c.Kindness = 1
	c.Level = map[engine.Color]int{}
	c.Aff = map[engine.Color]float64{}
	for _, colour := range engine.Colors() {
		c.Level[colour] = 1
		c.Aff[colour] = (rng.Float64() - 0.5) * 0.2
	}
	return c
}

// newWords draws a week's worth of words.
func newWords(week int, rng *rand.Rand) []engine.Word {
	out := make([]engine.Word, 0, WordsPerWeek)
	for i := range WordsPerWeek {
		w := engine.Word{
			ID:      fmt.Sprintf("s%02d-%d", week, i),
			Letters: 3 + rng.IntN(6),
			Traps:   map[engine.Color]int{},
		}
		// Fixed order: drawing in the map's order would consume the generator
		// differently each time and give a different week of words from the same
		// seed, which the determinism test caught.
		for _, colour := range engine.Colors() {
			if rng.Float64() < trapFrequency[colour] {
				w.Traps[colour] = 1 + rng.IntN(2)
			}
		}
		out = append(out, w)
	}
	return out
}

// Run plays a cohort of n children through a school year and reports how it
// went. It is deterministic given seed.
func Run(n int, seed uint64, cfg engine.Config) Result {
	// A simulation that cannot be reproduced proves nothing: a red build has to
	// be reproducible before anyone can chase it.
	// #nosec G404 -- reproducibility is the requirement; nothing here is secret.
	rng := rand.New(rand.NewPCG(seed, 0xC0FFEE))
	children := make([]*Child, n)
	for i := range children {
		children[i] = NewChild(rng)
	}

	var res Result
	var attempted, failed [3]int

	for week := range Weeks {
		words := newWords(week, rng)
		for _, c := range children {
			if c.QuitWeek != 0 {
				continue
			}
			c.forgetUnplayed(int32(week))
			// Two runs a session: a session is about twenty minutes, which is
			// two runs of eight or nine (ENCRE_00).
			for range c.Sessions * 2 {
				lost := c.playRun(words, int32(week), rng, cfg)
				// A manche counts as attempted only if the run got that far. A
				// run that ends in the first never reaches the boss, and
				// counting it as a boss attempt would quietly halve the boss's
				// failure rate — the number this whole test exists to watch.
				last := lost
				if lost < 0 {
					last = manches - 1
				}
				for i := 0; i <= last; i++ {
					attempted[i]++
				}
				if lost >= 0 {
					failed[lost]++
				}
			}
			c.reconsider(week)
		}
	}

	for _, c := range children {
		switch {
		case c.QuitWeek == 0:
			res.Playing++
		case c.QuitWhy == "frustration":
			res.QuitFrustrated++
		default:
			res.QuitBored++
		}
	}
	for i := range res.FailRate {
		if attempted[i] > 0 {
			res.FailRate[i] = float64(failed[i]) / float64(attempted[i])
		}
	}
	return res
}

// playRun plays one run and returns the manche that ended it, or -1 when it
// was won.
func (c *Child) playRun(week []engine.Word, weekNo int32, rng *rand.Rand, cfg engine.Config) (lostAt int) {
	// Sorted, because Go's map order is deliberately random and this list feeds
	// the deck's shuffle: leaving it as the map gives it made two runs of the
	// same seed disagree, which the determinism test caught.
	known := make([]engine.Word, 0, len(c.knownByID))
	for _, w := range c.knownByID {
		known = append(known, w)
	}
	slices.SortFunc(known, func(a, b engine.Word) int { return strings.Compare(a.ID, b.ID) })
	// The child keeps their gold words on the shelf; BuildDeck caps the list by
	// rank. Without this the deck is nothing but fresh words, which is the
	// hardest a week can possibly be and not what anyone plays.
	var garde []string
	for id, st := range c.states {
		if st.Gold {
			garde = append(garde, id)
		}
	}
	slices.Sort(garde) // the map's order is not stable, and the run must be
	deck := engine.BuildDeck(&c.Child, week, known, c.states, garde, weekNo, rng.Uint64(), cfg)

	// A manche mixes what is new with what is known, the way a real deck does.
	pool := slices.Concat(deck.Week, deck.Garde, deck.Old, deck.Cursed)
	// #nosec G404 -- the deck's own seed, so the shuffle is reproducible too.
	draw := rand.New(rand.NewPCG(deck.Seed, 0xDEA1))
	draw.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	run := engine.Run{
		ID:      fmt.Sprintf("r%d-%d", weekNo, c.runs),
		Deck:    deck,
		Rank:    c.Rank,
		Targets: engine.Targets(&c.Child, deck, c.states, cfg),
		Levels:  c.Level,
	}

	lostAt = -1
	for manche := range manches {
		boss := engine.NoBoss
		if manche == manches-1 {
			boss = bossOf(deck.Boss)
		}
		for i := range cfg.WordsPerManche {
			// The run's blind tokens are spent on the boss, which is where the
			// triple chips are needed. Playing blind is a gamble — the word is
			// never heard — so it lowers the chance as it raises the pay, and a
			// simulation that never gambles makes the boss look impossible.
			blind := manche == manches-1 && i < cfg.BlindTokens
			w := pool[(manche*cfg.WordsPerManche+i)%len(pool)]
			st := c.states[w.ID]
			if st == nil {
				st = &engine.WordState{}
				c.states[w.ID] = st
				c.knownByID[w.ID] = w
			}
			ctx := engine.Ctx{Levels: c.Level, Listens: 2, Boss: boss, Blind: blind}
			correct := rng.Float64() < engine.PHat(&c.Child, w, st, ctx)
			run.Attempts = append(run.Attempts, engine.Attempt{
				WordID: w.ID, Manche: manche, Correct: correct, Blind: blind,
			})
		}
	}

	out, err := engine.Replay(run, c.states, cfg)
	if err != nil {
		return -1
	}
	before := c.Rank
	events, err := engine.Apply(&c.Child, run, out, c.states, weekNo*7, weekNo, cfg)
	if err != nil {
		return -1
	}
	c.runs++
	// Only something genuinely new resets the boredom clock. Counting any event
	// at all — and a run raises several — made this fire on every run and the
	// signal never spoke, which is the same as not having it.
	for _, e := range events {
		if e == engine.RankUp || e == engine.LevelUp {
			c.lastNew = int(weekNo)
			break
		}
	}
	if c.Rank != before {
		c.lastNew = int(weekNo)
	}

	// What a loss costs depends entirely on where it fell, and the spread is
	// wide on purpose: the boss is MEANT to be lost sometimes, so losing it
	// barely registers, while going down in the first manche — the one that is
	// supposed to reassure — weighs almost seven times as much.
	weight := 0.0
	if !out.Won {
		lostAt = out.FailedAt
		switch out.FailedAt {
		case 2:
			weight = 0.15
		case 1:
			weight = 0.5
		default:
			weight = 1
		}
	}
	c.frustration = 0.92*c.frustration + 0.08*weight
	return lostAt
}

// forgetUnplayed is the week passing over words that were not met.
func (c *Child) forgetUnplayed(week int32) {
	for _, st := range c.states {
		if st.LastPlayedW < week {
			st.Mastery *= c.Forget
		}
		engine.Tarnish(st, week, engine.DefaultConfig())
	}
}

// reconsider decides whether the child keeps playing. Frustration is a run of
// bad evenings; boredom is nothing new for over a month.
func (c *Child) reconsider(week int) {
	if c.QuitWeek != 0 || week < 2 {
		return
	}
	switch {
	case c.frustration > 0.35 && c.runs >= 10:
		c.QuitWeek, c.QuitWhy = week, "frustration"
	case week-c.lastNew >= 8:
		c.QuitWeek, c.QuitWhy = week, "ennui"
	}
}
