package main

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/oioio-space/encre/client/ui"
)

// wagerLabelSize and wagerConsequenceSize are brief/ENCRE_06 §7's own two
// sizes on a wager button, both rounded up to [ui.GreffeGrid] (ENCRE_02
// §15): "le libellé reste en 19 px, la conséquence en 14 px" — 19 rounds up
// to 20, and so, coincidentally, does 14.
const wagerTextSize = greffeLabelSize

// wagerDropletCount is the three gold droplets of brief/ENCRE_06 §7: the
// gain is read from how many jump, never from a digit next to them.
const wagerDropletCount = 3

// drawWager draws the pari of brief/ENCRE_06 §7 — the only two texts on
// screen while it is up, since the card and the header it would otherwise
// compete with are not drawn during [phaseWager]: "jamais deux textes
// simultanés" (ENCRE_06 §8) holds by construction rather than by care taken
// inside this one function.
func (c *client) drawWager(dst *ebiten.Image) {
	twice, once := c.screen.WagerRects()

	drawFace(dst, "combien de fois écouter ?", c.faces.greffeLabel, greffeLabelSize,
		float64(c.screen.W)/2, float64(twice.Y)-30, cuir)

	c.drawListenTwiceButton(dst, twice)
	c.drawListenOnceButton(dst, once)
}

// drawListenTwiceButton draws "j'écoute 2 fois": fond Encre, deux oreilles
// en Parchemin clair, une goutte de cuivre — the safe choice, drawn without
// gold since nothing has been risked yet.
func (c *client) drawListenTwiceButton(dst *ebiten.Image, r ui.Rect) {
	x, y, w, h := float32(r.X), float32(r.Y), float32(r.W), float32(r.H)
	roundedRect(dst, x, y, w, h, 12, encre)
	earSide := h * 0.34
	drawEar(dst, x+w*0.28, y+h/2, earSide)
	drawEar(dst, x+w*0.72, y+h/2, earSide)
	vector.FillCircle(dst, x+w/2, y+h*0.62, 4, ambreBrule, true)

	drawFaceTight(dst, "j'écoute 2 fois", c.faces.greffeLabel, wagerTextSize, float64(x+w/2), float64(y+h*0.82), parcheminClair)
}

// drawEar draws one oreille: two nested arcs, Parchemin clair on the Encre
// button — simple enough to read at 52×… but the exact size a wager button
// draws at, and legible without a single word.
func drawEar(dst *ebiten.Image, cx, cy, side float32) {
	vector.StrokeCircle(dst, cx, cy, side/2, 3, parcheminClair, true)
	vector.StrokeCircle(dst, cx, cy, side/3.2, 2, parcheminClair, true)
}

// drawListenOnceButton draws "1 seule fois": fond Parchemin clair, bordure
// Brique, un bandeau sur les yeux, et les trois gouttes d'or qui sautillent
// (brief/ENCRE_06 §7's `sautille`, staggered by
// [anim.Juice.SautilleStagger]) — the gain this choice pays is read from how
// many of the three droplets are drawn, never from a digit.
func (c *client) drawListenOnceButton(dst *ebiten.Image, r ui.Rect) {
	x, y, w, h := float32(r.X), float32(r.Y), float32(r.W), float32(r.H)
	roundedRect(dst, x, y, w, h, 12, parcheminClair)
	vector.StrokeRect(dst, x+1, y+1, w-2, h-2, 2, brique, false)

	// The blindfold: two eye dots under a band, Brique on Parchemin clair.
	bandY := y + h*0.34
	vector.FillCircle(dst, x+w*0.42, bandY, 3, encre, true)
	vector.FillCircle(dst, x+w*0.58, bandY, 3, encre, true)
	vector.FillRect(dst, x+w*0.30, bandY-4, w*0.40, 8, brique, false)

	c.drawSautilleDroplets(dst, x+w/2, y+h*0.58, w*0.22)

	drawFaceTight(dst, "1 seule fois", c.faces.greffeLabel, wagerTextSize, float64(x+w/2), float64(y+h*0.86), brique)
}

// drawSautilleDroplets draws the three gold droplets of `sautille`,
// staggered by [anim.Juice.SautilleStagger] and eased by [anim.Juice.
// Sautille]'s own curve — the one place Or is allowed on this screen
// (ENCRE_02 §3: "l'Or n'existe que sur ce qui a été gagné").
func (c *client) drawSautilleDroplets(dst *ebiten.Image, cx, baseY, spread float32) {
	period := c.juice.Sautille.Duration.Duration()
	stagger := c.juice.SautilleStagger.Duration()
	now := time.Now()

	for i := range wagerDropletCount {
		phase := now.Add(-time.Duration(i) * stagger)
		t := math.Mod(float64(phase.UnixMilli()), float64(period.Milliseconds())) / float64(period.Milliseconds())
		bounce := eased(c.juice.Sautille, math.Abs(2*t-1))
		dx := spread * (float32(i) - 1)
		dy := -float32(6 * (1 - bounce))
		vector.FillCircle(dst, cx+dx, baseY+dy, 5, or, true)
		vector.StrokeCircle(dst, cx+dx, baseY+dy, 5, 1, orClair, true)
	}
}
