package main

import (
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/oioio-space/encre/client/anim"
)

// headerFraction is how much of the space above the card the score, the
// combo and the target share — ENCRE_06 §4's own header band, 104 of the
// phone's 844, kept as a ratio so it holds at the AZERTY layout too.
const headerFraction = 0.82

// diagnosticsInset is the strip [client.drawDiagnostics] keeps at the very
// top of the screen — ticket T00's own developer readout (touch-target
// measurements), never part of ENCRE_06's own header — so the two do not
// draw on top of one another.
const diagnosticsInset = 60

// eased runs t (0..1) through j's own curve, holding at the ends the same
// way [game.CardFlip.ScaleX] does: a malformed juice.json is a programming
// error to fall back flat from, not one to crash a frame over.
func eased(j anim.Anim, t float64) float64 {
	t = min(max(t, 0), 1)
	easing, err := j.Curve.Easing()
	if err != nil {
		return t
	}
	return easing.Y(t)
}

// drawHeader draws the score (left), the combo's flame (centre) and the
// target's seal (right) — brief/ENCRE_06 §4's header band — each with its
// own 20 px label, above the score's own 30 px number: the word stays the
// biggest character on screen (§4's own rule) because nothing here is drawn
// larger than greffeScoreSize.
func (c *client) drawHeader(dst *ebiten.Image) {
	headerH := (float64(c.screen.CardY) - diagnosticsInset) * headerFraction
	y := diagnosticsInset + headerH*0.62
	labelY := diagnosticsInset + headerH*0.86

	third := float64(c.screen.W) / 3
	scoreX, comboX, targetX := third*0.5, third*1.5, third*2.5

	drawFace(dst, "score", c.faces.greffeLabel, greffeLabelSize, scoreX, labelY, cuir)
	drawFace(dst, fmt.Sprintf("%d", int(math.Round(c.counterShown))),
		c.faces.greffeScore, greffeScoreSize, scoreX, y, brique)

	drawFace(dst, "combo", c.faces.greffeLabel, greffeLabelSize, comboX, labelY, cuir)
	c.drawFlame(dst, comboX, y)

	drawFace(dst, "cible", c.faces.greffeLabel, greffeLabelSize, targetX, labelY, cuir)
	c.drawSealTarget(dst, targetX, y)

	c.drawTargetBar(dst, diagnosticsInset+headerH)
}

// flameBaseH and flameBaseW are the flame's own size at combo == 1, in
// logical pixels: small enough that the header band it shares with two
// labels stays uncluttered, tall enough to read as a flame rather than a
// dot.
const (
	flameBaseH   = 22.0
	flameGrowth  = 3.0 // added per point of combo above one, before the cap
	flameGrowCap = 44.0
)

// drawFlame draws the candle's flame of brief/ENCRE_02 §10: its height
// tracks the combo directly rather than a digit next to it — "pas de
// chiffres là où des points suffisent" (ENCRE_02 §12) — and it vacille
// faster at a high combo ([anim.Juice.VacilleHautCombo] under
// [anim.Juice.VacilleRepos]'s own period, ENCRE_06 §6).
func (c *client) drawFlame(dst *ebiten.Image, cx, baseY float64) {
	combo := 1.0
	if c.runScore != nil {
		combo = c.runScore.Combo()
	}
	h := min(flameBaseH+flameGrowth*(combo-1), flameGrowCap)
	w := h * 0.55

	period := c.juice.VacilleRepos.Duration.Duration()
	if combo >= highComboThreshold {
		period = c.juice.VacilleHautCombo.Duration.Duration()
	}
	t := math.Mod(float64(time.Now().UnixMilli()), float64(period.Milliseconds())) / float64(period.Milliseconds())
	// vacille has no bezier of its own beyond ease-in-out; sampling it twice
	// a period (up then down) is what makes the flame breathe rather than
	// saw-tooth back to its start.
	half := eased(c.juice.VacilleRepos, math.Abs(2*t-1))
	wobble := 0.85 + 0.15*(1-half)

	var p vector.Path
	top := baseY - h*wobble
	p.MoveTo(float32(cx), float32(top))
	p.LineTo(float32(cx+w/2), float32(baseY))
	p.LineTo(float32(cx-w/2), float32(baseY))
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(braise)
	vector.FillPath(dst, &p, &vector.FillOptions{}, op)

	var inner vector.Path
	ih := h * 0.55 * wobble
	inner.MoveTo(float32(cx), float32(baseY-ih))
	inner.LineTo(float32(cx+w*0.22), float32(baseY))
	inner.LineTo(float32(cx-w*0.22), float32(baseY))
	inner.Close()
	iop := &vector.DrawPathOptions{AntiAlias: true}
	iop.ColorScale.ScaleWithColor(flamme)
	vector.FillPath(dst, &inner, &vector.FillOptions{}, iop)
}

// highComboThreshold is the combo the flame's vacille speeds up at.
// ENCRE_06 §6 names the two periods (VacilleRepos, VacilleHautCombo) but
// not the threshold between them; five is the point RunScore's own combo
// starts compounding past a beginner's ordinary run of one to four answers.
const highComboThreshold = 5

// sealBaseRadius is the target seal's own radius at rest, in logical pixels
// — the same order of size as [drawSeal]'s use on the valider key, since
// both draw the same wax seal (ENCRE_06 §4).
const sealBaseRadius = 20

// drawSealTarget draws the sceau-cible of brief/ENCRE_02 §10 ("cible : sceau
// de cire qui se fissure à 50% et éclate") at cx, baseY: a hairline once the
// manche is under way, a second fissure at half its target
// ([anim.Juice.Craque] names the beat, not the threshold — ENCRE_06 §6's own
// text does), and a burst of rays once the target is reached.
func (c *client) drawSealTarget(dst *ebiten.Image, cx, baseY float64) {
	p := c.sealProgress()
	r := float32(sealBaseRadius)
	drawSeal(dst, float32(cx), float32(baseY), r)

	line := func(x1, y1, x2, y2, w float32) {
		vector.StrokeLine(dst, x1, y1, x2, y2, w, sangSeche, true)
	}
	switch {
	case p >= 1:
		for i := range 6 {
			a := float64(i) / 6 * 2 * math.Pi
			x2 := float32(cx) + r*1.6*float32(math.Cos(a))
			y2 := float32(baseY) + r*1.6*float32(math.Sin(a))
			line(float32(cx), float32(baseY), x2, y2, 2)
		}
	case p >= 0.5:
		line(float32(cx)-r*0.5, float32(baseY)-r*0.6, float32(cx)+r*0.3, float32(baseY)+r*0.5, 1.5)
		line(float32(cx)+r*0.4, float32(baseY)-r*0.5, float32(cx)-r*0.2, float32(baseY)+r*0.4, 1.5)
	case p > 0:
		line(float32(cx)-r*0.4, float32(baseY)-r*0.5, float32(cx)+r*0.2, float32(baseY)+r*0.3, 1.5)
	}
}

// drawTargetBar draws the 12 px bar of brief/ENCRE_06 §4 under the header: a
// 6 px fill of Braise, radius 3, at [client.sealProgress]'s own width.
func (c *client) drawTargetBar(dst *ebiten.Image, y0 float64) {
	const (
		barH   = 6
		radius = 3
	)
	w := float32(c.screen.W) - 24
	x := float32(12)
	y := float32(y0) + 3
	roundedRect(dst, x, y, w, barH, radius, parcheminVieux)
	if c.runScore == nil {
		return
	}
	fillW := w * float32(c.sealProgress())
	if fillW > 0 {
		roundedRect(dst, x, y, fillW, barH, radius, braise)
	}
}

// drawDroplets draws every droplet of [client.droplets] still in flight,
// along [anim.Juice.Droplet]'s own arc from the card to the counter — an
// ink drop (ENCRE_02 §10's own vocabulary, "jetons : gouttes d'encre qui
// volent"), never gold: on this screen, Or is reserved for what the wager
// already won (ENCRE_02 §3).
func (c *client) drawDroplets(dst *ebiten.Image) {
	if c.runScore == nil {
		return
	}
	from := struct{ x, y float64 }{float64(c.cardX) + float64(c.cardW)/2, float64(c.cardY) + float64(c.cardH)/2}
	to := struct{ x, y float64 }{float64(c.screen.W) / 6, float64(c.screen.CardY) * headerFraction * 0.62}
	now := time.Now()
	for _, d := range c.droplets {
		if d.landed {
			continue
		}
		elapsed := now.Sub(d.start)
		if elapsed < 0 {
			continue
		}
		t := min(float64(elapsed)/float64(c.juice.Droplet.Duration.Duration()), 1)
		e := eased(c.juice.Droplet, t)
		x := from.x + (to.x-from.x)*e
		// The arc: a parabola peaking at the midpoint, the same shape
		// ENCRE_06 §6 draws `jetons` with.
		arc := -1 * 40 * 4 * e * (1 - e)
		y := from.y + (to.y-from.y)*e + arc
		vector.FillCircle(dst, float32(x), float32(y), 4, encre, true)
	}
}

// counterArrivalScale returns the scale the counter's own number draws at,
// snapping to [anim.Juice.CounterArrivalScale] for the instant a droplet
// just landed and easing back to 1 — ENCRE_06 §6's `compteur`: "scale(1.18)
// à l'arrivée des gouttes, jamais avant".
func (c *client) counterArrivalScale() float64 {
	if c.counterBumpAt.IsZero() {
		return 1
	}
	elapsed := time.Since(c.counterBumpAt)
	dur := c.juice.CounterStoryboard.Duration.Duration()
	if elapsed >= dur {
		return 1
	}
	t := float64(elapsed) / float64(dur)
	settle := eased(c.juice.CounterStoryboard, t)
	return c.juice.CounterArrivalScale - (c.juice.CounterArrivalScale-1)*settle
}
