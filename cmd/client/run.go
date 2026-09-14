package main

import (
	"time"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

// runPhase names where one card of the manche is in its own life on screen:
// asked how many times to hear it, being written, held silent after the last
// letter (encre-cs5.1), or paying its chips into the counter droplet by
// droplet.
type runPhase int

const (
	phaseWager runPhase = iota
	phaseWriting
	phaseSilence
	phaseDroplets
)

// dropletAnim is one goutte of [anim.Juice.Droplet] in flight from the card
// to the counter, carrying its own share of the word's chips: one droplet
// per [anim.Juice.DropletPerTokens] tokens (ENCRE_06 §6's `jetons`), rather
// than every token as its own droplet, which is what keeps a big word from
// launching dozens of them.
type dropletAnim struct {
	start  time.Time
	amount float64
	landed bool
}

// startManche resets the client to the first card of a fresh demo manche
// (bead encre-tfy.5): the RunScore engine.Score itself drives
// ([RunScore.Apply]), the cards demoManche supplies, and the target
// [demoTarget] computes from the same [engine.Targets] the server would.
func (c *client) startManche() {
	c.cfg = engine.DefaultConfig()
	c.cards = demoManche()
	c.cardIdx = 0
	c.target = demoTarget(c.cards, c.cfg)
	c.runScore = game.NewRunScore(c.cfg)
	c.runScore.StartManche(0, engine.NoBoss, false)
	c.counterShown = 0
	c.startWager()
}

// currentCard is the card the child is answering.
func (c *client) currentCard() runCard { return c.cards[c.cardIdx] }

// startWager opens the pari of brief/ENCRE_06 §7 for the card now current:
// no writing happens until the child has chosen how many times to hear it.
func (c *client) startWager() {
	c.phase = phaseWager
	c.entry.Clear()
	c.correction = nil
	c.letterTimes = nil
}

// chooseWager records the child's pick — twice, ordinarily, or once, blind
// for [anim.Juice]'s own reward the wager screen draws in gouttes rather
// than a number — and opens the card for writing.
func (c *client) chooseWager(blind bool) {
	c.wagerBlind = blind
	c.phase = phaseWriting
	c.wordStart = time.Now()
}

// onLetterTyped records when a rune just landed in the entry, for the bave
// (anim.Juice.Bave) drawn behind it while it is still wet, and checks
// whether it just missed: FirstMismatch answering anything but -1 is a trap
// letter (ENCRE_02 §12's hitstop), corrected sans texte (bead encre-cs5,
// [game.Correction]) rather than explained.
func (c *client) onLetterTyped() {
	c.letterTimes = append(c.letterTimes, time.Now())

	target := c.currentCard().word.Text
	typed := c.entry.Text()
	mismatch := game.FirstMismatch(target, typed)
	if mismatch < 0 {
		// The prefix typed so far is right again — whether it always was,
		// or the child just retyped past an earlier miss — so any blinking
		// correction from before is over.
		c.correction = nil
		if len([]rune(typed)) == len([]rune(target)) {
			c.completeWord()
		}
		return
	}

	corr := game.NewCorrection(target, typed)
	c.correction = &corr
	c.correctionAt = time.Now()
	c.triggerHitstopAndShake()

	// "le mot écrit correctement… et l'enfant retape à partir de là" — the
	// wrong tail is dropped, not the letters already right.
	for c.entry.Len() > mismatch {
		c.entry.Erase()
	}
	c.letterTimes = c.letterTimes[:min(len(c.letterTimes), mismatch)]
}

// triggerHitstopAndShake freezes input for [anim.Juice.HitstopTrap] and
// starts the screen shaking for [anim.Juice.Tremble.Duration], at an
// amplitude of [anim.Juice.ShakeAmplitude] per point of combo, capped at
// [anim.Juice.ShakeMaxAmplitude] — ENCRE_02 §12's own formula.
func (c *client) triggerHitstopAndShake() {
	now := time.Now()
	c.hitstopUntil = now.Add(c.juice.HitstopTrap.Duration())
	c.shakeUntil = now.Add(c.juice.Tremble.Duration.Duration())
	combo := 1.0
	if c.runScore != nil {
		combo = c.runScore.Combo()
	}
	c.shakeMag = min(c.juice.ShakeAmplitude*combo, c.juice.ShakeMaxAmplitude)
}

// completeWord closes the word correctly typed and opens
// [anim.Juice.ScoreSilence] — the half-second of bead encre-cs5.1, held
// before a single chip leaves for the counter.
func (c *client) completeWord() {
	c.phase = phaseSilence
	c.silenceUntil = time.Now().Add(c.juice.ScoreSilence.Duration())
	c.pendingMillis = int(time.Since(c.wordStart) / time.Millisecond)
}

// tick advances every clock the run screen owns that is not driven directly
// by a keystroke: the silence running out, and droplets landing.
func (c *client) tick(now time.Time) {
	switch c.phase {
	case phaseSilence:
		if !now.Before(c.silenceUntil) {
			c.settleWord()
		}
	case phaseDroplets:
		c.advanceDroplets(now)
	}
}

// settleWord scores the word for real — [RunScore.Apply], the same
// engine.Score the server replays — and turns the chips it earned into
// droplets, one per [anim.Juice.DropletPerTokens], staggered by
// [anim.Juice.DropletStagger] (ENCRE_06 §6's `jetons`).
func (c *client) settleWord() {
	card := c.currentCard()
	a := engine.Attempt{
		WordID:  card.word.ID,
		Manche:  0,
		Blind:   c.wagerBlind,
		Correct: true,
		Typed:   c.entry.Text(),
		Millis:  c.pendingMillis,
	}
	st := &engine.WordState{Seen: true}
	chips, mult := c.runScore.Apply(a, card.word, st, nil, nil)

	total := chips * mult
	perDroplet := float64(c.juice.DropletPerTokens)
	n := max(1, int(total/perDroplet+0.999999))
	now := time.Now()
	c.droplets = c.droplets[:0]
	for i := range n {
		c.droplets = append(c.droplets, dropletAnim{
			start:  now.Add(time.Duration(i) * c.juice.DropletStagger.Duration()),
			amount: total / float64(n),
		})
	}
	c.phase = phaseDroplets
}

// advanceDroplets lands every droplet whose flight ([anim.Juice.Droplet]'s
// duration, from its own staggered start) has finished, bumping the counter
// by its own share the instant it lands — "le compteur ne monte qu'à
// l'arrivée" (ENCRE_02 §12) — and moves on once every droplet is down.
func (c *client) advanceDroplets(now time.Time) {
	allLanded := true
	for i := range c.droplets {
		d := &c.droplets[i]
		if d.landed {
			continue
		}
		if now.Sub(d.start) >= c.juice.Droplet.Duration.Duration() {
			d.landed = true
			c.counterShown += d.amount
			c.counterBumpAt = now
			continue
		}
		allLanded = false
	}
	if !allLanded {
		return
	}
	c.nextCard()
}

// nextCard moves on to the next card of the manche, or wraps back to the
// first — this prototype loops its demo manche rather than ending the run,
// since Salle and Récap are a later ticket's scenes.
func (c *client) nextCard() {
	c.cardIdx = (c.cardIdx + 1) % len(c.cards)
	c.startWager()
}

// sealProgress is how far the running score has come toward c.target, for
// the seal-cible's own crack (ENCRE_06 §6 `craque`: "2e fissure à 50%").
func (c *client) sealProgress() float64 {
	if c.target <= 0 {
		return 0
	}
	return min(c.runScore.Total(0)/c.target, 1)
}
