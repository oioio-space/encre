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

// v1Talismans are the eight the Échoppe offers in V1 (ENCRE_01 §12); the rest
// unlock later and have no place in a first year's simulation.
var v1Talismans = []engine.TalismanID{
	engine.Perroquet, engine.Chronometre, engine.Jumeau, engine.Loupe,
	engine.Gomme, engine.Collectionneur, engine.Aimant, engine.Sourd,
}

const (
	// talismanShopCost prices what the shop function buys. Config carries no
	// Talisman price yet (encre-00q.1 names this as its own follow-up), so the
	// simulation gives itself a placeholder rather than leaving the whole
	// Échoppe unsimulated.
	talismanShopCost = 3
	// maxTalismansCarried caps one run's shopping the same way: no slot count
	// exists in Config either.
	maxTalismansCarried = 3
	// chronoFloorMillis and chronoSpanMillis bound the answer times the
	// simulation draws, spanning either side of the Chronomètre's window so
	// the Talisman is sometimes fast enough to pay and sometimes not.
	chronoFloorMillis = 3000
	chronoSpanMillis  = 18000
)

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
	// knownSorted is knownByID in a stable order, rebuilt only when a word is
	// met for the first time. Go's map order is random and this list feeds the
	// deck's shuffle, so it has to be sorted — and sorting hundreds of words a
	// dozen times an evening was the second most expensive thing in a cohort.
	knownSorted []engine.Word
	// frustration is an exponential average of runs going badly.
	frustration float64
	runs        int
	// lastNew is the week something new last happened; boredom is measured
	// from it.
	lastNew int
	// QuitWeek is when the child stopped, or zero while they are still playing.
	QuitWeek int
	QuitWhy  string
	// Money is what the Échoppe has left the child to spend, carried between
	// runs — a Talisman itself is not (ENCRE_01 §12: "perdus en fin de run").
	Money int

	// lastAttempts is the previous run's attempts, kept only so the package's
	// own tests can look at what was actually played.
	lastAttempts []engine.Attempt
}

// runStats is what one played run adds to the cohort's [Result], beyond the
// manche it may have been lost in.
type runStats struct {
	lostAt             int
	correct, attempted [3]int
	comboMax           int
	carried, paid      map[engine.TalismanID]bool
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
	// RankAtEnd counts, index Blanc through Diamant, how many children of the
	// cohort held each rank at the end of the school year.
	RankAtEnd [6]int
	// CorrectRate is the share of non-Rencontre words spelled right, and
	// CorrectRateByManche the same broken down by manche.
	CorrectRate         float64
	CorrectRateByManche [3]float64
	// TalismanCarried counts the runs each V1 Talisman was carried in, and
	// TalismanPaid the runs it actually changed the score in — a Talisman a
	// child buys and that never pays is the shop's own bug, and this is what
	// the acceptance test of encre-00q.2 checks it against.
	TalismanCarried map[engine.TalismanID]int
	TalismanPaid    map[engine.TalismanID]int
	// ComboMaxMedian is the median, across every run played, of the highest
	// combo that run reached.
	ComboMaxMedian float64
}

// Retention is the share of the cohort still playing at the end.
func (r Result) Retention(cohort int) float64 { return float64(r.Playing) / float64(cohort) }

// AboveBlanc is the share of the cohort that finished the year above the
// entry rank — ENCRE_01's rank-up asks for both wins and weeks, and
// [engine.Child.AdvanceWeek] is what makes the second half of that reachable.
func (r Result) AboveBlanc(cohort int) float64 {
	above := 0
	for _, n := range r.RankAtEnd[1:] {
		above += n
	}
	return float64(above) / float64(cohort)
}

// LearnSpread bounds the range NewChild draws a child's LearnRate and Forget
// from — how fast mastery moves on a success, and how much a week away from
// a word costs it (ENCRE_03 §7). Floor is the slower/more forgetful end of
// the spread; the ceiling is Floor+Span. Individual difference is the point:
// a spread models a class, a single number models one imaginary child.
type LearnSpread struct {
	LearnFloor, LearnSpan   float64
	ForgetFloor, ForgetSpan float64
}

// DefaultSpread is the spread the original simulation used (encre-00q.6's
// grid sweep looks for a better one, rather than this becoming the only one
// [sim] can draw a cohort from).
var DefaultSpread = LearnSpread{LearnFloor: 0.12, LearnSpan: 0.18, ForgetFloor: 0.88, ForgetSpan: 0.09}

// NewChild draws a child from [DefaultSpread].
func NewChild(rng *rand.Rand) *Child { return newChild(rng, DefaultSpread) }

// newChild draws a child from sp.
func newChild(rng *rand.Rand, sp LearnSpread) *Child {
	c := &Child{
		Forget:    sp.ForgetFloor + rng.Float64()*sp.ForgetSpan,
		Sessions:  []int{1, 2, 2, 3, 3, 3, 4, 4, 5, 6}[rng.IntN(10)],
		states:    map[string]*engine.WordState{},
		knownByID: map[string]engine.Word{},
	}
	c.Skill = 0.5 + rng.Float64()*0.4
	c.LearnRate = sp.LearnFloor + rng.Float64()*sp.LearnSpan
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
// went. It is deterministic given seed, and draws its cohort from
// [DefaultSpread].
func Run(n int, seed uint64, cfg engine.Config) Result {
	return RunWithSpread(n, seed, cfg, DefaultSpread)
}

// RunWithSpread is [Run], but drawing the cohort's LearnRate and Forget from
// sp rather than [DefaultSpread] — the knob encre-00q.6's balance sweep turns.
func RunWithSpread(n int, seed uint64, cfg engine.Config, sp LearnSpread) Result {
	// A simulation that cannot be reproduced proves nothing: a red build has to
	// be reproducible before anyone can chase it.
	// #nosec G404 -- reproducibility is the requirement; nothing here is secret.
	rng := rand.New(rand.NewPCG(seed, 0xC0FFEE))
	children := make([]*Child, n)
	for i := range children {
		children[i] = newChild(rng, sp)
	}

	var res Result
	res.TalismanCarried = map[engine.TalismanID]int{}
	res.TalismanPaid = map[engine.TalismanID]int{}
	var mancheAttempted, mancheFailed [3]int
	var wordAttempted, wordCorrect [3]int
	var comboMaxes []int

	for week := range Weeks {
		words := newWords(week, rng)
		for _, c := range children {
			if c.QuitWeek != 0 {
				continue
			}
			// Once per week, whether or not the child plays: a rank-up asks
			// for weeks under the current one as well as wins, and Apply runs
			// once per finished run rather than once per week, so it is not
			// where this belongs (encre-00q.1).
			c.AdvanceWeek()
			c.forgetUnplayed(int32(week))
			// Two runs a session: a session is about twenty minutes, which is
			// two runs of eight or nine (ENCRE_00).
			for range c.Sessions * 2 {
				stats := c.playRun(words, int32(week), rng, cfg)
				// A manche counts as attempted only if the run got that far. A
				// run that ends in the first never reaches the boss, and
				// counting it as a boss attempt would quietly halve the boss's
				// failure rate — the number this whole test exists to watch.
				last := stats.lostAt
				if last < 0 {
					last = manches - 1
				}
				for i := 0; i <= last; i++ {
					mancheAttempted[i]++
				}
				if stats.lostAt >= 0 {
					mancheFailed[stats.lostAt]++
				}
				for m := range manches {
					wordAttempted[m] += stats.attempted[m]
					wordCorrect[m] += stats.correct[m]
				}
				comboMaxes = append(comboMaxes, stats.comboMax)
				for id := range stats.carried {
					res.TalismanCarried[id]++
				}
				for id := range stats.paid {
					res.TalismanPaid[id]++
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
		res.RankAtEnd[c.Rank]++
	}
	for i := range res.FailRate {
		if mancheAttempted[i] > 0 {
			res.FailRate[i] = float64(mancheFailed[i]) / float64(mancheAttempted[i])
		}
	}

	var totalAttempted, totalCorrect int
	for m := range manches {
		totalAttempted += wordAttempted[m]
		totalCorrect += wordCorrect[m]
		if wordAttempted[m] > 0 {
			res.CorrectRateByManche[m] = float64(wordCorrect[m]) / float64(wordAttempted[m])
		}
	}
	if totalAttempted > 0 {
		res.CorrectRate = float64(totalCorrect) / float64(totalAttempted)
	}
	res.ComboMaxMedian = median(comboMaxes)
	return res
}

// median returns the middle value of ns, averaging the two middle values on
// an even count, or 0 for an empty slice.
func median(ns []int) float64 {
	if len(ns) == 0 {
		return 0
	}
	sorted := slices.Clone(ns)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[mid])
	}
	return float64(sorted[mid-1]+sorted[mid]) / 2
}

// playRun plays one run and returns what it did for the cohort's stats.
func (c *Child) playRun(week []engine.Word, weekNo int32, rng *rand.Rand, cfg engine.Config) runStats {
	// Sorted, because Go's map order is deliberately random and this list feeds
	// the deck's shuffle: leaving it as the map gives it made two runs of the
	// same seed disagree, which the determinism test caught.
	known := c.knownWords()
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

	// The Échoppe is the one V1 room the engine actually scores (encre-00q.2):
	// without this, run.Talismans was never set and the whole shop was
	// untested.
	talismans := c.shop(deck.Rooms, rng)
	owned := engine.Talismans{}
	for _, id := range talismans {
		owned[id] = true
	}

	run := engine.Run{
		ID:        fmt.Sprintf("r%d-%d", weekNo, c.runs),
		Deck:      deck,
		Rank:      c.Rank,
		Targets:   engine.Targets(&c.Child, deck, c.states, cfg),
		Talismans: talismans,
		Levels:    c.Level,
	}

	for manche := range manches {
		boss := engine.NoBoss
		if manche == manches-1 {
			boss = bossOf(deck.Boss)
		}
		// Rank raises what the word is asked under (ENCRE_01 §8): Argent trims
		// listens on manches 2 and 3, Platine and Diamant everywhere. This is
		// the manche's own context, not the per-attempt one below — it is what
		// [engine.Draw] ranks the pool's PHat under, and the trap measured in
		// encre-00q.3 is ranking it under anything less real than that: doing
		// so outside the boss context sent the boss's own failure rate to
		// 40.6%.
		mancheListens := 2
		if c.Rank >= 4 {
			mancheListens = 1
		}
		drawCtx := engine.Ctx{
			Levels: c.Level, Listens: mancheListens, Sentence: manche > 0, Boss: boss,
		}
		words := engine.Draw(&c.Child, deck, c.states, manche, drawCtx, cfg)
		for i, w := range words {
			// The run's blind tokens are spent on the boss, which is where the
			// triple chips are needed. Playing blind is a gamble — the word is
			// never heard — so it lowers the chance as it raises the pay, and a
			// simulation that never gambles makes the boss look impossible.
			blind := manche == manches-1 && i < cfg.BlindTokens
			st := c.states[w.ID]
			// A word's very first meeting is a Rencontre (ENCRE_01 §4): shown,
			// not asked, and never failed. Anything else is played for real.
			isNew := st == nil
			if isNew {
				st = &engine.WordState{}
				c.states[w.ID] = st
				c.knownByID[w.ID] = w
				c.knownSorted = nil
				blind = false
			}
			// The rank raises what the word is asked under (ENCRE_01 §8):
			// Argent trims listens on manches 2 and 3, Platine and Diamant
			// everywhere; the fill-in and the phrase manches (2 and 3) always
			// ask inside a sentence rather than the word alone.
			listens := 2
			switch {
			case c.Rank >= 4:
				listens = 1
			case c.Rank >= 2 && manche > 0 && rng.Float64() < 0.6:
				listens = 1
			}
			ctx := engine.Ctx{
				Levels: c.Level, Listens: listens, Sentence: manche > 0,
				Blind: blind, Boss: boss,
			}
			correct := isNew || rng.Float64() < engine.PHat(&c.Child, w, st, ctx)
			run.Attempts = append(run.Attempts, engine.Attempt{
				WordID: w.ID, Manche: manche, Correct: correct, Blind: blind, Copy: isNew,
				Millis: chronoFloorMillis + rng.IntN(chronoSpanMillis),
			})
		}
	}
	c.lastAttempts = run.Attempts

	out, err := engine.Replay(run, c.states, cfg)
	if err != nil {
		return runStats{lostAt: -1}
	}
	// A manche short of its target but inside the window earns a Revanche
	// (ENCRE_01 §10): the client asks, the server checks and replays it with
	// one more multiplier on the very same attempts.
	if !out.Won && out.FailedAt >= 0 {
		target := run.Targets[out.FailedAt]
		if out.Scores[out.FailedAt] >= target*cfg.RevancheWindow {
			run.Revanche[out.FailedAt] = true
			if retried, err := engine.Replay(run, c.states, cfg); err == nil {
				out = retried
			}
		}
	}

	stats := runStats{
		lostAt:  -1,
		carried: map[engine.TalismanID]bool{},
		paid:    map[engine.TalismanID]bool{},
	}
	for _, a := range run.Attempts {
		if a.Copy {
			continue
		}
		stats.attempted[a.Manche]++
		if a.Correct {
			stats.correct[a.Manche]++
		}
	}
	stats.comboMax = comboMaxOf(run.Attempts, owned)
	// A Talisman "paid" this run when removing it, and only it, would have
	// scored the run lower — the marginal test the engine itself answers, run
	// once per Talisman carried, rather than a second copy of Score's rules.
	for _, id := range talismans {
		stats.carried[id] = true
		without := run
		without.Talismans = slices.DeleteFunc(slices.Clone(talismans), func(x engine.TalismanID) bool { return x == id })
		if bare, err := engine.Replay(without, c.states, cfg); err == nil && bare.Scores != out.Scores {
			stats.paid[id] = true
		}
	}

	before := c.Rank
	events, err := engine.Apply(&c.Child, run, out, c.states, weekNo*7, weekNo, cfg)
	if err != nil {
		return runStats{lostAt: -1}
	}
	c.Money += out.Money
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
		stats.lostAt = out.FailedAt
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
	return stats
}

// shop plays the two room transitions of a run (ENCRE_01 §5) and returns the
// Talismans the child leaves holding. Only the Échoppe is modelled: the Repos
// and l'Encrier change nothing [engine] scores, so a run through them plays
// identically to skipping them.
func (c *Child) shop(rooms [2][2]engine.RoomID, rng *rand.Rand) []engine.TalismanID {
	owned := map[engine.TalismanID]bool{}
	for _, pair := range rooms {
		if !slices.Contains(pair[:], engine.Echoppe) {
			continue
		}
		if len(owned) >= maxTalismansCarried || c.Money < talismanShopCost {
			continue
		}
		pick := v1Talismans[rng.IntN(len(v1Talismans))]
		if owned[pick] {
			continue
		}
		owned[pick] = true
		c.Money -= talismanShopCost
	}
	out := make([]engine.TalismanID, 0, len(owned))
	for id := range owned {
		out = append(out, id)
	}
	slices.Sort(out) // the map's order is not stable, and a run must be
	return out
}

// comboMaxOf is the highest combo a run's attempts would have reached, kept
// in step by hand with Replay's own bookkeeping (ENCRE_01 §6, §12) since the
// engine does not expose the combo itself — this is a simulation statistic,
// not something a client or the server ever needs to read back.
func comboMaxOf(attempts []engine.Attempt, owned engine.Talismans) int {
	combo := 1
	peak := combo
	manche := -1
	gommeSpent := false
	for _, a := range attempts {
		if a.Manche != manche {
			manche, gommeSpent = a.Manche, false
		}
		switch {
		case a.Correct && !a.Copy:
			combo++
			peak = max(peak, combo)
		case !a.Correct && owned[engine.Gomme] && !gommeSpent:
			gommeSpent = true
		case !a.Correct:
			combo = 1
		}
	}
	return peak
}

// knownWords is every word the child has met, in a stable order.
func (c *Child) knownWords() []engine.Word {
	if c.knownSorted != nil || len(c.knownByID) == 0 {
		return c.knownSorted
	}
	out := make([]engine.Word, 0, len(c.knownByID))
	for _, w := range c.knownByID {
		out = append(out, w)
	}
	slices.SortFunc(out, func(a, b engine.Word) int { return strings.Compare(a.ID, b.ID) })
	c.knownSorted = out
	return out
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
