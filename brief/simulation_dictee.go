//go:build ignore

// Command simulation_dictee is the standalone balance simulation from the design
// brief. It is excluded from the module build (see the ignore constraint above)
// until it is ported into the engine package as brief/ENCRE_04 §3 sim/ describes;
// run it directly with: go run brief/simulation_dictee.go

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
)

// ---------- Domain ----------

const NR = 6

var ruleNames = [NR]string{"Muette", "Jumelle", "Accent", "Masquee", "Sosie", "Accordee"}
var ruleDiff = [NR]float64{0.16, 0.12, 0.07, 0.17, 0.20, 0.20}
var ruleFreq = [NR]float64{0.35, 0.25, 0.45, 0.20, 0.10, 0.30}

type Word struct {
	ID      int
	Letters int
	Traps   [NR]int
	Week    int
}

type WordState struct {
	M           float64
	Days        map[int]bool // distinct success days
	FirstDay    int
	Fails       int
	FailWeeks   map[int]bool
	ConsecOK    int
	Gold        bool
	Cursed      bool
	Holo        bool
	LastPlayedW int
	Tarnished   bool
	Seen        bool
}

type Talisman struct {
	Name   string
	Cost   int
	Rarity int // 0 common 1 rare 2 legendary
}

var tals = []Talisman{
	{"Perroquet", 4, 0}, {"Chronometre", 5, 0}, {"Jumeau", 5, 0}, {"Loupe", 5, 0},
	{"Gomme", 4, 0}, {"Collectionneur", 6, 1}, {"Aimant", 5, 0}, {"Sourd", 6, 1},
	// unlockables
	{"Fantome", 6, 1}, {"Horloge", 6, 1}, {"Bibliothecaire", 7, 1}, {"Colosse", 8, 2},
	{"Couronne", 5, 0}, {"Meute", 6, 1}, {"Miroir", 6, 1}, {"Echo", 5, 0},
	{"Phare", 5, 0}, {"Banquier", 4, 0}, {"Alchimiste", 7, 2}, {"Tambour", 6, 1},
}

const (
	Perroquet = iota
	Chronometre
	Jumeau
	Loupe
	Gomme
	Collectionneur
	Aimant
	Sourd
	Fantome
	Horloge
	Bibliothecaire
	Colosse
	Couronne
	Meute
	Miroir
	Echo
	Phare
	Banquier
	Alchimiste
	Tambour
)

type Flags struct {
	Name             string
	RankMult         []float64
	BaseTargets      [3]float64
	XPMode           int     // 0: every success (+1) ; 1: quadratic cost ; 2: only blind/boss successes count + quadratic
	TarnishBonus     bool    // tarnished gold cards give x2 chips when restored -> heuristic picks them
	RecycleOld       int     // old non-gold words injected per run
	GoldNeedsGap     bool    // gold requires 3 distinct days AND >=7 days between first & last
	SafetyNet        bool    // 3 boss fails -> temporary rank-1 next week
	Rencontre        bool    // first exposure of a word teaches without scoring
	BlindMult        float64 // multiplier of chips when blind
	ComboAcross      bool    // combo carries across manches
	BossMods         bool
	AccordeeInSent   bool // accordee words always in sentence, even manche 1
	MaxRank          int
	GardeSlots       int
	GardeGrowth      bool       // slots grow with rank
	WeeklyChallenge  bool       // an extra weekly modifier that changes the boss each week
	PlacementRun     bool       // first week: placement determines starting rank
	TargetFromDeck   bool       // target computed from deck chip potential (dynamic but transparent)
	DeckShare        float64    // fraction of chip potential needed (if TargetFromDeck)
	Seasons          bool       // at MaxRank, "Prestige" resets rank with harder conditions but keeps everything
	K                [3]float64 // expected-combo factors for TargetFromDeck
	RankUpWins       int
	RankUpMinWeeks   int
	LevelCapPerWeek  bool
	BlindTokens      int // 0 = unlimited
	CurseNeedsWeeks  bool
	ComboSoft        bool // combo halves instead of resetting
	MasteryScaled    bool // deck value weighted by mastery of its words
	RankDescent      bool // 3 boss fails in a row -> rank-1 permanently (best rank kept as badge)
	ContentDrip      bool // new Talisman unlocked at spaced boss-win milestones
	QuarterlySeasons bool // every 12 weeks: new season content (bosses, cosmetics) = big novelty for all
	Revanche         bool // near-miss (>=85% of target) -> one immediate retry of the manche with +1 Mult
	ExpectedTarget   bool // deck value = sum chips x estimated hit rate of this child on this word
	WordsPerManche   int
}

type Child struct {
	Skill          float64
	Aff            [NR]float64
	LR             float64
	Forget         float64
	Sessions       int
	Risk           float64
	Words          map[int]*WordState
	XP             [NR]float64
	Level          [NR]int
	Rank           int
	TempRank       int
	BossWins       int
	BossFail       int
	Unlocked       map[int]bool
	Quit           int // week quit, 0 = still playing
	QuitWhy        string
	Novelty        int // last week with novelty event
	Frust          []float64
	BestCombo      int
	Prestige       int
	RankedThisWeek bool
	WeeksAtRank    int
	BigNovelty     int
	BestRank       int
	FrEMA          float64
	BossWinsTotal  int
	LevelUpWk      [NR]int
	// stats
	GoldCount, HoloCount, Cursed, Dompted, Restored int
	Revanches, RevancheWins                         int
	BlindN, BlindOK, SureN                          int
	RunsN                                           int
	FailAt                                          [3]int
	RankAtWk                                        []int
	DicteeWk                                        []float64
	ScoreWk                                         []float64
	SynergyN                                        int
	TalBought                                       int
	GardeUsed                                       map[int]int
}

type Ctx struct {
	listens  int
	sentence bool
	blind    bool
	bossMod  int // -1 none
	fast     bool
}

// ---------- Simulation ----------

type Sim struct {
	ratios [3][]float64
	rng    *rand.Rand
	f      Flags
	words  []Word
	nextID int
}

func (s *Sim) newWord(week int) Word {
	w := Word{ID: s.nextID, Letters: 4 + s.rng.Intn(7), Week: week}
	s.nextID++
	has := false
	for r := 0; r < NR; r++ {
		if s.rng.Float64() < ruleFreq[r] {
			w.Traps[r] = 1
			if r == 2 && s.rng.Float64() < 0.3 {
				w.Traps[r] = 2
			}
			has = true
		}
	}
	if !has {
		w.Traps[s.rng.Intn(NR)] = 1
	}
	return w
}

func (s *Sim) newChild() *Child {
	c := &Child{
		Skill:     0.5 + s.rng.Float64()*0.4,
		LR:        0.12 + s.rng.Float64()*0.18,
		Forget:    0.88 + s.rng.Float64()*0.09,
		Sessions:  []int{1, 2, 2, 3, 3, 3, 4, 4, 5, 6}[s.rng.Intn(10)],
		Risk:      0.2 + s.rng.Float64()*0.6,
		Words:     map[int]*WordState{},
		Unlocked:  map[int]bool{},
		GardeUsed: map[int]int{},
	}
	for r := 0; r < NR; r++ {
		c.Aff[r] = (s.rng.Float64() - 0.5) * 0.2
		c.Level[r] = 1
	}
	for i := 0; i < 8; i++ {
		c.Unlocked[i] = true
	}
	return c
}

func (c *Child) ws(id int) *WordState {
	w, ok := c.Words[id]
	if !ok {
		w = &WordState{Days: map[int]bool{}, FirstDay: -1, FailWeeks: map[int]bool{}}
		c.Words[id] = w
	}
	return w
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func (s *Sim) pCorrect(c *Child, w *Word, st *WordState, ctx Ctx) float64 {
	diff := 0.0
	for r := 0; r < NR; r++ {
		if w.Traps[r] > 0 {
			lvlRed := 1 - 0.4*float64(c.Level[r]-1)/9.0
			diff += float64(w.Traps[r]) * (ruleDiff[r]*lvlRed - c.Aff[r])
		}
	}
	base := c.Skill - diff
	p := st.M*0.93 + (1-st.M)*base
	if ctx.listens == 1 {
		p -= 0.10 * (1 - st.M)
	}
	if ctx.sentence {
		p -= 0.05
	}
	if ctx.blind {
		p -= 0.06 * (1 - st.M)
	}
	switch ctx.bossMod {
	case 0: // Chuchoteur: handled via listens
	case 1: // Voleur d'accents: no p effect (chips)
	case 2: // Presse
		p -= 0.05
	case 3: // Muet
		p -= 0.10 * (1 - st.M)
	}
	return clamp(p, 0.03, 0.98)
}

func (c *Child) learn(st *WordState, ok bool, day int) {
	if ok {
		st.M += c.LR * (1 - st.M)
		st.Days[day] = true
		if st.FirstDay < 0 {
			st.FirstDay = day
		}
		st.ConsecOK++
	} else {
		st.M += 0.5 * c.LR * (1 - st.M)
		st.Fails++
		st.ConsecOK = 0
	}
}

func (s *Sim) chips(c *Child, w *Word, st *WordState, own map[int]bool, ctx Ctx) float64 {
	ch := float64(w.Letters)
	for r := 0; r < NR; r++ {
		if w.Traps[r] == 0 {
			continue
		}
		v := float64(w.Traps[r]) * 10 * float64(c.Level[r])
		if ctx.bossMod == 1 && r == 2 {
			v = 0
		}
		if r == 1 && own[Loupe] {
			v *= 2
		}
		if r == 2 && own[Couronne] {
			v *= 2
		}
		if r == 4 && own[Miroir] {
			v *= 2
		}
		ch += v
	}
	if st.Gold {
		if own[Aimant] {
			ch *= 1.5
		}
		if own[Bibliothecaire] {
			ch *= 2
		}
		if st.Tarnished && s.f.TarnishBonus {
			ch *= 2
		}
	}
	if st.Cursed {
		ch *= 5
	}
	if ctx.blind {
		bm := s.f.BlindMult
		if own[Sourd] {
			bm += 1
		}
		ch *= bm
	}
	return ch
}

func multBonus(c *Child, w *Word, st *WordState, own map[int]bool, ctx Ctx, goldPlayed int) float64 {
	m := 0.0
	if own[Perroquet] {
		m += 1
	}
	if own[Chronometre] && ctx.fast {
		m += 4
	}
	if own[Jumeau] {
		m += 2 * float64(w.Traps[1])
	}
	if own[Fantome] {
		m += 3 * float64(w.Traps[0])
	}
	if own[Meute] {
		m += 2 * float64(w.Traps[5])
	}
	if own[Collectionneur] {
		m += float64(goldPlayed)
	}
	if own[Colosse] {
		m += 5
	}
	if own[Colosse+1000] {
		m += 1
	}
	for id := range own {
		if id >= 100 {
			m += 1
		}
	}
	if own[Tambour] && st.Cursed {
		m += 6
	}
	return m
}

type RunResult struct {
	score    [3]float64
	failedAt int // -1 = won
	target   [3]float64
}

func (s *Sim) targets(c *Child, rank int, deck []*Word) [3]float64 {
	var t [3]float64
	if s.f.TargetFromDeck {
		// chip potential of the deck at current levels, no talismans, average combo ~3
		pot := 0.0
		for _, w := range deck {
			ch := s.chips(c, w, c.ws(w.ID), map[int]bool{}, Ctx{})
			if s.f.ExpectedTarget {
				ch *= s.pCorrect(c, w, c.ws(w.ID), Ctx{listens: 2, bossMod: -1})
			} else if s.f.MasteryScaled {
				ch *= 0.4 + 1.2*c.ws(w.ID).M
			}
			pot += ch
		}
		avg := pot / float64(len(deck))
		t = [3]float64{avg * s.f.K[0], avg * s.f.K[1], avg * s.f.K[2]}
	} else {
		t = s.f.BaseTargets
	}
	rm := s.f.RankMult[rank]
	for i := range t {
		t[i] *= rm
	}
	return t
}

func (s *Sim) rankCtx(rank int, manche int, ctx *Ctx) {
	ctx.listens = 2
	switch {
	case rank >= 4:
		ctx.listens = 1
	case rank >= 2 && manche >= 1:
		if s.rng.Float64() < 0.6 {
			ctx.listens = 1
		}
	}
	if rank >= 3 && manche == 1 {
		ctx.sentence = true
	}
	if rank >= 5 {
		ctx.sentence = true
	}
}

func (s *Sim) chooseGarde(c *Child, slots int) []*Word {
	var golds []*Word
	for id, st := range c.Words {
		if st.Gold {
			golds = append(golds, &s.words[id])
		}
	}
	sort.Slice(golds, func(i, j int) bool {
		a, b := golds[i], golds[j]
		va := s.chips(c, a, c.ws(a.ID), nil, Ctx{})
		vb := s.chips(c, b, c.ws(b.ID), nil, Ctx{})
		if s.f.TarnishBonus {
			ta, tb := c.ws(a.ID).Tarnished, c.ws(b.ID).Tarnished
			if ta != tb {
				return ta
			}
		}
		return va > vb
	})
	if len(golds) > slots {
		golds = golds[:slots]
	}
	return golds
}

func (s *Sim) run(c *Child, week, day int, weekWords []*Word, rank int) RunResult {
	c.RunsN++
	own := map[int]bool{}
	money := 4
	deck := make([]*Word, 0, 20)
	deck = append(deck, weekWords...)
	slots := s.f.GardeSlots
	if s.f.GardeGrowth {
		slots += rank / 2
	}
	garde := s.chooseGarde(c, slots)
	for _, g := range garde {
		c.GardeUsed[g.ID]++
	}
	deck = append(deck, garde...)
	// recycle old non-gold words & cursed
	var old []*Word
	for id, st := range c.Words {
		if st.Seen && !st.Gold && s.words[id].Week < week {
			old = append(old, &s.words[id])
		}
	}
	s.rng.Shuffle(len(old), func(i, j int) { old[i], old[j] = old[j], old[i] })
	n := s.f.RecycleOld
	if n > len(old) {
		n = len(old)
	}
	deck = append(deck, old[:n]...)

	res := RunResult{failedAt: -1}
	res.target = s.targets(c, rank, deck)
	combo := 1.0
	if own[Horloge] {
		combo = 3
	}
	goldPlayed := 0
	bossMod := -1
	if s.f.BossMods && rank >= 1 {
		bossMod = s.rng.Intn(4)
	}
	if s.f.WeeklyChallenge {
		bossMod = week % 4
	}
	// deal order: new words first in manche 1 (Rencontre), then shuffle
	pool := append([]*Word{}, deck...)
	s.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	idx := 0
	next := func() *Word {
		if idx >= len(pool) {
			s.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
			idx = 0
		}
		w := pool[idx]
		idx++
		return w
	}
	blindStreak, muetteStreak := 0, 0
	echoUsed := false
	for m := 0; m < 3; m++ {
		retry := false
	manche:
		if !s.f.ComboAcross || retry {
			combo = 1
			if own[Horloge] {
				combo = 3
			}
		}
		gommeUsed := false
		score := 0.0
		blindLeft := s.f.BlindTokens
		if blindLeft == 0 {
			blindLeft = 99
		}
		nw := s.f.WordsPerManche
		if nw == 0 {
			nw = 5
		}
		for k := 0; k < nw; k++ {
			w := next()
			st := c.ws(w.ID)
			if !st.Seen {
				st.Seen = true
				if s.f.Rencontre {
					// mode Rencontre : la carte est visible, l'enfant la copie. Pas d'echec possible, petit gain.
					okc := s.rng.Float64() < 0.95
					c.learn(st, okc, day)
					st.LastPlayedW = week
					score += s.chips(c, w, st, own, Ctx{bossMod: -1})
					continue
				}
			}
			ctx := Ctx{bossMod: -1}
			s.rankCtx(rank, m, &ctx)
			if m >= 1 {
				ctx.sentence = true
			}
			if s.f.AccordeeInSent && w.Traps[5] > 0 {
				ctx.sentence = true
			}
			if m == 2 {
				ctx.bossMod = bossMod
				if bossMod == 0 {
					ctx.listens = 1
				}
			}
			if own[Phare] || own[Perroquet] {
				if ctx.listens == 1 && s.rng.Float64() < 0.5 {
					ctx.listens = 2
				}
			}
			// blind decision: rational-ish, tempered by risk taste
			pSure := s.pCorrect(c, w, st, ctx)
			ctxB := ctx
			ctxB.blind = true
			pBlind := s.pCorrect(c, w, st, ctxB)
			if blindLeft > 0 && (pBlind > 0.75 || (pBlind > 0.55 && s.rng.Float64() < c.Risk)) {
				blindLeft--
				ctx = ctxB
				c.BlindN++
			} else {
				c.SureN++
			}
			ctx.fast = s.rng.Float64() < st.M*0.7
			p := pSure
			if ctx.blind {
				p = pBlind
			}
			ok := s.rng.Float64() < p
			if !ok && own[Echo] && !echoUsed {
				echoUsed = true
				ok = s.rng.Float64() < p
			}
			c.learn(st, ok, day)
			st.LastPlayedW = week
			if ok {
				if ctx.blind {
					c.BlindOK++
					blindStreak++
				} else {
					blindStreak = 0
				}
				if w.Traps[0] > 0 {
					muetteStreak++
				} else {
					muetteStreak = 0
				}
				if st.Gold {
					goldPlayed++
					if st.Tarnished {
						st.Tarnished = false
						c.Restored++
					}
				}
				ch := s.chips(c, w, st, own, ctx)
				mult := combo + multBonus(c, w, st, own, ctx, goldPlayed)
				score += ch * mult
				combo++
				if combo > float64(c.BestCombo) {
					c.BestCombo = int(combo)
				}
				// XP
				xp := 1.0
				if s.f.XPMode == 2 {
					xp = 0
					if ctx.blind || m == 2 {
						xp = 1
					}
				}
				for r := 0; r < NR; r++ {
					if w.Traps[r] > 0 {
						c.XP[r] += xp
						need := 20.0
						if s.f.XPMode >= 1 {
							need = 12 * float64(c.Level[r])
						}
						if c.XP[r] >= need && c.Level[r] < 10 && (!s.f.LevelCapPerWeek || week+1-c.LevelUpWk[r] >= 3) {
							c.XP[r] -= need
							c.Level[r]++
							c.LevelUpWk[r] = week + 1
							c.Novelty = week
						}
					}
				}
				// gold / curse transitions
				if st.Cursed && st.ConsecOK >= 3 {
					st.Cursed = false
					st.Gold = true
					c.Dompted++
					c.GoldCount++
					c.Novelty = week
				}
				if !st.Gold && !st.Cursed && len(st.Days) >= 3 {
					gap := day - st.FirstDay
					if !s.f.GoldNeedsGap || gap >= 7 {
						st.Gold = true
						c.GoldCount++
						c.Novelty = week
						if s.rng.Float64() < 1.0/8 {
							st.Holo = true
							c.HoloCount++
						}
					}
				}
			} else {
				blindStreak, muetteStreak = 0, 0
				if own[Gomme] && !gommeUsed {
					gommeUsed = true
				} else if s.f.ComboSoft {
					combo = math.Max(1, math.Floor(combo/2))
				} else {
					combo = 1
				}
				if st.Gold {
					st.Gold = false
					st.Tarnished = false
					c.GoldCount--
					st.Days = map[int]bool{}
					st.FirstDay = -1
				}
				st.FailWeeks[week] = true
				if !st.Cursed && st.Fails >= 3 && !st.Gold && (!s.f.CurseNeedsWeeks || len(st.FailWeeks) >= 3) {
					st.Cursed = true
					c.Cursed++
					st.ConsecOK = 0
				}
			}
			// exploits
			if blindStreak >= 5 && !c.Unlocked[Sourd+1] { // placeholder
			}
		}
		res.score[m] = score
		if s.f.TargetFromDeck && rank == 0 {
			s.ratios[m] = append(s.ratios[m], score/(res.target[m]/s.f.K[m]))
		}
		if score < res.target[m] {
			if s.f.Revanche && !retry && score >= 0.85*res.target[m] {
				retry = true
				c.Revanches++
				own[Colosse+1000] = true // marker: +1 mult granted (handled below)
				goto manche
			}
			res.failedAt = m
			c.FailAt[m]++
			break
		}
		if retry {
			c.RevancheWins++
		}
		// money & shop
		money += 4 + int(math.Min(3, score/res.target[m]))
		if own[Banquier] {
			money += 3
		}
		if m < 2 {
			s.shop(c, own, &money, deck)
		}
	}
	// exploit unlocks (milestones)
	unlock := func(id int) {
		if !c.Unlocked[id] {
			c.Unlocked[id] = true
			c.Novelty = week
			c.BigNovelty = week
		}
	}
	if c.BestCombo >= 8 {
		unlock(Horloge)
	}
	if muetteStreak >= 4 {
		unlock(Fantome)
	}
	if c.GoldCount >= 10 {
		unlock(Bibliothecaire)
		unlock(Couronne)
	}
	if c.GoldCount >= 25 {
		unlock(Meute)
		unlock(Echo)
	}
	if c.GoldCount >= 50 {
		unlock(Colosse)
	}
	if c.BlindOK >= 30 {
		unlock(Miroir)
	}
	if c.Dompted >= 3 {
		unlock(Tambour)
	}
	if rank >= 2 {
		unlock(Phare)
		unlock(Banquier)
	}
	if c.Restored >= 5 {
		unlock(Alchimiste)
	}
	if s.f.ContentDrip {
		for i, th := range []int{4, 9, 15, 22, 30, 39, 49, 60, 72, 85, 99, 114, 130, 147, 165, 184, 204, 225, 247, 270, 294, 319, 345, 372} {
			if c.BossWinsTotal >= th {
				unlock(100 + i)
			}
		}
	}
	return res
}

func (s *Sim) shop(c *Child, own map[int]bool, money *int, deck []*Word) {
	var pool []int
	for id := range c.Unlocked {
		if !own[id] {
			pool = append(pool, id)
		}
	}
	s.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if len(pool) > 3 {
		pool = pool[:3]
	}
	// deck trap profile
	var traps [NR]int
	for _, w := range deck {
		for r := 0; r < NR; r++ {
			traps[r] += w.Traps[r]
		}
	}
	value := func(id int) float64 {
		v := 1.0
		switch id {
		case Jumeau, Loupe:
			v = float64(traps[1]) * 0.5
			if (id == Jumeau && own[Loupe]) || (id == Loupe && own[Jumeau]) {
				v *= 2
				c.SynergyN++
			}
		case Fantome:
			v = float64(traps[0]) * 0.6
		case Couronne:
			v = float64(traps[2]) * 0.5
		case Meute:
			v = float64(traps[5]) * 0.6
		case Miroir:
			v = float64(traps[4]) * 0.8
		case Collectionneur, Aimant, Bibliothecaire:
			v = float64(c.GoldCount) * 0.15
			if (id == Collectionneur && own[Aimant]) || (id == Aimant && own[Collectionneur]) {
				v *= 2
				c.SynergyN++
			}
		case Chronometre:
			v = 2.5
			if own[Perroquet] {
				v *= 1.5
				c.SynergyN++
			}
		case Colosse:
			v = 5
		case Tambour:
			v = float64(c.Cursed-c.Dompted) * 1.5
		}
		if id >= 100 {
			v = 1.2
		}
		return v
	}
	sort.Slice(pool, func(i, j int) bool { return value(pool[i]) > value(pool[j]) })
	for _, id := range pool {
		cost := 5
		if id < len(tals) {
			cost = tals[id].Cost
		}
		if *money >= cost && value(id) > 0.5 {
			*money -= cost
			own[id] = true
			c.TalBought++
			break
		}
	}
}

func (s *Sim) week(c *Child, week int) {
	// new words
	ww := make([]*Word, 0, 10)
	for i := 0; i < 10; i++ {
		w := s.newWord(week)
		s.words = append(s.words, w)
		ww = append(ww, &s.words[len(s.words)-1])
	}
	rank := c.Rank
	if c.TempRank > 0 {
		rank = c.Rank - 1
		c.TempRank = 0
	}
	fails, runs := 0, 0
	c.RankedThisWeek = false
	if s.f.QuarterlySeasons && week > 0 && week%12 == 0 {
		c.BigNovelty = week
		c.Novelty = week
	}
	c.WeeksAtRank++
	scoreSum := 0.0
	days := []int{1, 2, 3, 4, 5, 6}
	s.rng.Shuffle(len(days), func(i, j int) { days[i], days[j] = days[j], days[i] })
	for si := 0; si < c.Sessions; si++ {
		day := week*7 + days[si%6]
		for r := 0; r < 2; r++ {
			res := s.run(c, week, day, ww, rank)
			runs++
			scoreSum += res.score[0] + res.score[1] + res.score[2]
			fv := 0.0
			if res.failedAt >= 0 {
				switch res.failedAt {
				case 2:
					fails++
					c.BossFail++
					fv = 0.15
				case 1:
					fails += 2
					fv = 0.5
				default:
					fails += 2
					fv = 1
				}
			} else {
				c.BossFail = 0
				c.BossWins++
				c.BossWinsTotal++
				if c.BossWins >= s.f.RankUpWins && c.WeeksAtRank >= s.f.RankUpMinWeeks && c.Rank < s.f.MaxRank && rank == c.Rank && !c.RankedThisWeek {
					c.WeeksAtRank = 0
					c.RankedThisWeek = true
					c.Rank++
					c.BossWins = 0
					c.Novelty = week
				} else if c.BossWins >= s.f.RankUpWins && c.Rank == s.f.MaxRank && s.f.Seasons {
					c.Prestige++
					c.Rank = 2
					c.BossWins = 0
					c.Novelty = week
					c.BigNovelty = week
				}
			}
			c.FrEMA = 0.92*c.FrEMA + 0.08*fv
			if s.f.SafetyNet && c.BossFail >= 3 && c.Rank > 0 {
				if s.f.RankDescent {
					c.Rank--
					c.BossWins = 0
					c.WeeksAtRank = 0
					rank = c.Rank
				} else {
					c.TempRank = 1
				}
				c.BossFail = 0
			}
			if c.Rank > c.BestRank {
				c.BestRank = c.Rank
			}
		}
	}
	// weekly forgetting & tarnish
	for id, st := range c.Words {
		if st.LastPlayedW < week {
			st.M *= c.Forget
		}
		if st.Gold && week-st.LastPlayedW >= 4 {
			st.Tarnished = true
		}
		_ = id
	}
	// real dictée (Friday)
	okN := 0
	for _, w := range ww {
		st := c.ws(w.ID)
		p := s.pCorrect(c, w, st, Ctx{listens: 1, sentence: true, bossMod: -1})
		if s.rng.Float64() < p {
			okN++
		}
	}
	c.DicteeWk = append(c.DicteeWk, float64(okN)/10)
	c.ScoreWk = append(c.ScoreWk, scoreSum/float64(runs))
	c.RankAtWk = append(c.RankAtWk, c.Rank)
	// engagement
	fr := float64(fails) / float64(2*runs)
	c.Frust = append(c.Frust, fr)
	if c.Quit == 0 && week >= 2 && c.Quit != -1 {
		if c.FrEMA > 0.35 && c.RunsN >= 10 {
			c.Quit = week
			c.QuitWhy = "frustration"
		} else if week-c.Novelty >= 5 || week-c.BigNovelty >= 8 {
			c.Quit = week
			c.QuitWhy = "ennui"
		}
	}
}

// control child: school only (2 exposures per word per week, then forget)
func (s *Sim) controlDictee(c *Child, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	out := []float64{}
	for wk := 0; wk < 36; wk++ {
		okN := 0
		for i := 0; i < 10; i++ {
			w := s.newWord(wk)
			st := &WordState{Days: map[int]bool{}}
			for e := 0; e < 3; e++ {
				p := s.pCorrect(c, &w, st, Ctx{listens: 2, bossMod: -1})
				ok := r.Float64() < p
				c.learn(st, ok, e)
			}
			p := s.pCorrect(c, &w, st, Ctx{listens: 1, sentence: true, bossMod: -1})
			if r.Float64() < p {
				okN++
			}
		}
		out = append(out, float64(okN)/10)
	}
	return out
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	c := append([]float64{}, xs...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func pearson(x, y []float64) float64 {
	n := float64(len(x))
	var sx, sy, sxx, syy, sxy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
		sxx += x[i] * x[i]
		syy += y[i] * y[i]
		sxy += x[i] * y[i]
	}
	num := n*sxy - sx*sy
	den := math.Sqrt((n*sxx - sx*sx) * (n*syy - sy*sy))
	if den == 0 {
		return 0
	}
	return num / den
}

func runVersion(f Flags, seed int64) {
	s := &Sim{rng: rand.New(rand.NewSource(seed)), f: f}
	const N = 100
	children := make([]*Child, N)
	for i := range children {
		children[i] = s.newChild()
	}
	for wk := 0; wk < 36; wk++ {
		for _, c := range children {
			if c.Quit == 0 {
				s.week(c, wk)
			}
		}
	}
	// metrics
	fmt.Printf("\n===== %s =====\n", f.Name)
	playing, frust, ennui := 0, 0, 0
	rankDist := make([]int, 8)
	var failRate [3]float64
	totalRuns := 0
	var d1, d36, ctrl1, ctrl36 []float64
	var gameScore, dictee []float64
	var levelsMax []float64
	blindN, blindOK, sureN := 0, 0, 0
	golds, tarn, restored, holos, synergy, dompted := 0, 0, 0, 0, 0, 0
	rev, revW := 0, 0
	gardeDistinct := 0
	weeksToRank := map[int][]float64{}
	quitWeeks := []float64{}
	var qs, qk, ss, sk float64
	nq, ns := 0, 0
	for _, c := range children {
		if c.Quit == 0 {
			playing++
			ss += float64(c.Sessions)
			sk += c.Skill
			ns++
		} else {
			qs += float64(c.Sessions)
			qk += c.Skill
			nq++
			quitWeeks = append(quitWeeks, float64(c.Quit))
			if c.QuitWhy == "frustration" {
				frust++
			} else {
				ennui++
			}
		}
		rankDist[c.Rank+c.Prestige*0]++
		for i := 0; i < 3; i++ {
			failRate[i] += float64(c.FailAt[i])
		}
		totalRuns += c.RunsN
		n := len(c.DicteeWk)
		if n >= 4 {
			d1 = append(d1, (c.DicteeWk[0]+c.DicteeWk[1]+c.DicteeWk[2]+c.DicteeWk[3])/4)
		}
		if n >= 36 {
			d36 = append(d36, (c.DicteeWk[32]+c.DicteeWk[33]+c.DicteeWk[34]+c.DicteeWk[35])/4)
		}
		cd := s.controlDictee(c, seed+int64(len(ctrl1)))
		ctrl1 = append(ctrl1, (cd[0]+cd[1]+cd[2]+cd[3])/4)
		ctrl36 = append(ctrl36, (cd[32]+cd[33]+cd[34]+cd[35])/4)
		for i := 4; i < n; i++ {
			gameScore = append(gameScore, math.Log1p(c.ScoreWk[i]))
			dictee = append(dictee, c.DicteeWk[i])
		}
		mx := 0
		for r := 0; r < NR; r++ {
			if c.Level[r] > mx {
				mx = c.Level[r]
			}
		}
		levelsMax = append(levelsMax, float64(mx))
		blindN += c.BlindN
		blindOK += c.BlindOK
		sureN += c.SureN
		golds += c.GoldCount
		holos += c.HoloCount
		synergy += c.SynergyN
		dompted += c.Dompted
		rev += c.Revanches
		revW += c.RevancheWins
		restored += c.Restored
		for _, st := range c.Words {
			if st.Gold && st.Tarnished {
				tarn++
			}
		}
		gardeDistinct += len(c.GardeUsed)
		for wk, rk := range c.RankAtWk {
			if wk == 0 || rk > c.RankAtWk[wk-1] {
				weeksToRank[rk] = append(weeksToRank[rk], float64(wk))
			}
		}
	}
	fmt.Printf("Encore en jeu à la semaine 36 : %d%%   (arrêts: %d frustration, %d ennui ; semaine médiane d'arrêt %.0f)\n", playing, frust, ennui, median(quitWeeks))
	fmt.Printf("Profil des arrêts : %.1f sessions/sem, skill %.2f  | des fidèles : %.1f sessions/sem, skill %.2f\n", qs/math.Max(1, float64(nq)), qk/math.Max(1, float64(nq)), ss/math.Max(1, float64(ns)), sk/math.Max(1, float64(ns)))
	fmt.Printf("Taux d'échec par manche (sur %d runs) : M1 %.1f%%  M2 %.1f%%  Boss %.1f%%\n",
		totalRuns, 100*failRate[0]/float64(totalRuns), 100*failRate[1]/float64(totalRuns), 100*failRate[2]/float64(totalRuns))
	fmt.Printf("Rangs en semaine 36 : Blanc %d  Bronze %d  Argent %d  Or %d  Platine %d  Diamant %d\n", rankDist[0], rankDist[1], rankDist[2], rankDist[3], rankDist[4], rankDist[5])
	fmt.Printf("Semaine médiane d'accès : Bronze %.0f  Argent %.0f  Or %.0f  Platine %.0f  Diamant %.0f\n", median(weeksToRank[1]), median(weeksToRank[2]), median(weeksToRank[3]), median(weeksToRank[4]), median(weeksToRank[5]))
	fmt.Printf("Niveau de règle max (médiane) : %.0f/10\n", median(levelsMax))
	fmt.Printf("Dictée réelle : sem 1-4 %.0f%% -> sem 33-36 %.0f%%   | témoin (école seule) %.0f%% -> %.0f%%\n", 100*median(d1), 100*median(d36), 100*median(ctrl1), 100*median(ctrl36))
	fmt.Printf("Corrélation score de jeu / dictée réelle : r = %.2f\n", pearson(gameScore, dictee))
	fmt.Printf("À l'aveugle : %.0f%% des mots, réussite %.0f%%\n", 100*float64(blindN)/float64(blindN+sureN), 100*float64(blindOK)/float64(max(blindN, 1)))
	fmt.Printf("Dorées/enfant %.1f  ternies %.1f  restaurées %.1f  holo %.1f  | Garde: %.1f cartes distinctes utilisées\n", float64(golds)/N, float64(tarn)/N, float64(restored)/N, float64(holos)/N, float64(gardeDistinct)/N)
	fmt.Printf("Maudites domptées/enfant %.1f   Synergies achetées/enfant %.1f   Revanches/enfant %.1f (gagnées %.0f%%)\n", float64(dompted)/N, float64(synergy)/N, float64(rev)/N, 100*float64(revW)/float64(max(rev, 1)))
}

func main() {
	seed := int64(42)
	if len(os.Args) > 1 {
		fmt.Sscan(os.Args[1], &seed)
	}
	base := Flags{
		Name:         "V1 — concept tel que décrit",
		RankMult:     []float64{1, 1.6, 2.5, 4, 6},
		BaseTargets:  [3]float64{150, 450, 900},
		XPMode:       0,
		RecycleOld:   4,
		Rencontre:    true,
		SafetyNet:    true,
		TarnishBonus: true,
		GoldNeedsGap: true,
		BlindMult:    3,
		ComboAcross:  true,
		BossMods:     true,
		MaxRank:      4,
		GardeSlots:   3,
		GardeGrowth:  true,
	}
	base.RankUpWins = 3
	base.RankUpMinWeeks = 0
	versions := []Flags{base}
	v := base
	v.Name = "V2 — cibles = valeur du deck x combo attendu (fin du combo parfait)"
	v.TargetFromDeck = true
	v.K = [3]float64{3.5, 7, 16}
	v.RankMult = []float64{1, 1.15, 1.3, 1.45, 1.6}
	versions = append(versions, v)
	v.Name = "V3 — + 7 mots/manche, valeur du deck = jetons x taux de reussite personnel"
	v.WordsPerManche = 7
	v.ExpectedTarget = true
	v.K = [3]float64{4.5, 10, 22}
	versions = append(versions, v)
	v.Name = "V4 — + rang : 8 victoires et 3 semaines mini, 6 rangs, paliers plus durs"
	v.RankUpWins = 8
	v.RankUpMinWeeks = 3
	v.MaxRank = 5
	v.RankMult = []float64{1, 1.3, 1.7, 2.2, 2.8, 3.5}
	v.RankDescent = true
	versions = append(versions, v)
	v.Name = "V5 — + XP seulement aveugle/boss, cout croissant, 1 niveau/regle/semaine"
	v.XPMode = 2
	v.LevelCapPerWeek = true
	versions = append(versions, v)
	v.Name = "V6 — + 2 paris aveugle/manche, maudites sur 2 semaines, Accordees en phrase"
	v.BlindTokens = 2
	v.CurseNeedsWeeks = true
	v.AccordeeInSent = true
	v.K = [3]float64{2.5, 6.5, 17}
	versions = append(versions, v)
	v.Name = "V7 — + defi hebdo rotatif, prestige, 24 Talismans au compte-gouttes, 3 saisons/an"
	v.WeeklyChallenge = true
	v.Seasons = true
	v.ContentDrip = true
	v.QuarterlySeasons = true
	versions = append(versions, v)
	v.Name = "V8 — + Revanche : a moins de 15%% de la cible, on rejoue la manche avec +1 Mult"
	v.Revanche = true
	versions = append(versions, v)
	v.Name = "V9 — 7 ans : 6 mots/manche"
	v.WordsPerManche = 6
	if kk := os.Getenv("K"); kk != "" {
		fmt.Sscanf(kk, "%f,%f,%f", &v.K[0], &v.K[1], &v.K[2])
	}
	versions = append(versions, v)
	if len(os.Args) > 2 && os.Args[2] == "diag" {
		diag(versions[6], seed)
		return
	}
	for _, f := range versions {
		runVersion(f, seed)
	}
}

func diag(f Flags, seed int64) {
	s := &Sim{rng: rand.New(rand.NewSource(seed)), f: f}
	children := make([]*Child, 100)
	for i := range children {
		children[i] = s.newChild()
		children[i].Quit = -1 // never quit in diag
	}
	for wk := 0; wk < 8; wk++ {
		for _, c := range children {
			s.week(c, wk)
		}
	}
	for m := 0; m < 3; m++ {
		r := append([]float64{}, s.ratios[m]...)
		sort.Float64s(r)
		if len(r) > 0 {
			fmt.Printf("manche %d ratio score/chips: p5 %.1f p10 %.1f p20 %.1f p50 %.1f p80 %.1f\n", m+1, r[len(r)/20], r[len(r)/10], r[len(r)/5], r[len(r)/2], r[len(r)*4/5])
		}
	}
	fmt.Println("skill | sessions | fail rate wk0..7 | rank wk7 | lvlmax")
	sort.Slice(children, func(i, j int) bool { return children[i].Skill < children[j].Skill })
	for i, c := range children {
		if i%10 != 0 {
			continue
		}
		mx := 0
		for r := 0; r < NR; r++ {
			if c.Level[r] > mx {
				mx = c.Level[r]
			}
		}
		fmt.Printf("%.2f | %d | ", c.Skill, c.Sessions)
		for _, fr := range c.Frust {
			fmt.Printf("%.0f ", fr*100)
		}
		fmt.Printf("| %d | %d | ema %.2f | M1 %d M2 %d B %d / %d runs\n", c.Rank, mx, c.FrEMA, c.FailAt[0], c.FailAt[1], c.FailAt[2], c.RunsN)
	}
}
