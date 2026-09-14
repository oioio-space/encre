package main

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/oioio-space/encre/client/ui"
)

// blinkPeriod is how fast the one letter a [game.Correction] holds blinks —
// fast enough to draw the eye without being one of the game's own named
// timings: this is a UI convention (a text cursor's own blink rate),not a
// juice.json animation ENCRE_06 §6 names.
const blinkPeriod = 500 * time.Millisecond

// drawEntry draws the word as the child has typed it (bead encre-tfy.5),
// letter by letter with the bave of [client.drawBaveLetters] — or, while a
// [game.Correction] is open, the one letter missed, blinking, and nothing
// past it: "no sentence of explanation, only the word written correctly…
// with the one letter the child missed blinking" (bead encre-cs5).
func (c *client) drawEntry(dst *ebiten.Image) {
	cx := float64(c.cardX) + float64(c.cardW)/2
	cy := float64(c.entryY)

	if c.correction != nil {
		c.drawCorrectionBlink(dst, cx, cy)
		return
	}

	shown := c.entry.Text()
	if shown == "" {
		drawFace(dst, "…", c.faces.plumeWord, float64(ui.DefaultPlumeSize), cx, cy, cuir)
		return
	}
	c.drawBaveLetters(dst, shown, cx, cy)
}

// drawBaveLetters draws s tracked (ui.TrackingRatio) and, behind any letter
// typed within the last [anim.Juice.Bave.Duration], a soft ink halo eased
// down to nothing by [anim.Juice.Bave]'s own curve — ENCRE_02 §12's "l'encre
// bave à l'apparition de chaque lettre".
func (c *client) drawBaveLetters(dst *ebiten.Image, s string, cx, cy float64) {
	f := c.faces.plumeWord
	size := float64(ui.DefaultPlumeSize)
	advance := func(r rune) float64 { g := string(r); return text.AdvanceAt(g, len(g), f) }
	positions, width := ui.Tracked(s, size, advance)
	left := cx - width/2

	now := time.Now()
	baveDur := c.juice.Bave.Duration.Duration()
	for i, r := range []rune(s) {
		x := left + positions[i]
		w := advance(r)

		if i < len(c.letterTimes) {
			if elapsed := now.Sub(c.letterTimes[i]); elapsed >= 0 && elapsed < baveDur {
				t := eased(c.juice.Bave, float64(elapsed)/float64(baveDur))
				overflow := float32(2 * (1 - t))
				halo := fadeAlpha(encreInk, 0.35*(1-t))
				vector.FillCircle(dst, float32(x+w/2), float32(cy), float32(w)/2+overflow, halo, true)
			}
		}
		drawRune(dst, r, f, x, cy, encre)
	}
}

// drawCorrectionBlink draws the kept prefix of [game.Correction], unblinking,
// then the one correct letter the child missed, blinking at [blinkPeriod] —
// nothing of the word past it, so the child is shown the one letter to fix
// rather than the answer.
func (c *client) drawCorrectionBlink(dst *ebiten.Image, cx, cy float64) {
	corr := *c.correction
	kept := []rune(corr.Kept())
	target := []rune(c.currentCard().word.Text)
	idx := corr.MismatchIndex()

	full := string(kept)
	if idx >= 0 && idx < len(target) {
		full += string(target[idx])
	}

	f := c.faces.plumeWord
	size := float64(ui.DefaultPlumeSize)
	advance := func(r rune) float64 { g := string(r); return text.AdvanceAt(g, len(g), f) }
	positions, width := ui.Tracked(full, size, advance)
	left := cx - width/2

	for i, r := range kept {
		drawRune(dst, r, f, left+positions[i], cy, encre)
	}
	if idx < 0 || idx >= len(target) {
		return
	}
	if time.Now().UnixMilli()/blinkPeriod.Milliseconds()%2 == 0 {
		drawRune(dst, target[idx], f, left+positions[len(kept)], cy, brique)
	}
}

// drawRune draws one rune with f, its left edge at (x, y).
func drawRune(dst *ebiten.Image, r rune, f text.Face, x, y float64, col color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterNearest
	op.ColorScale.ScaleWithColor(col)
	op.PrimaryAlign, op.SecondaryAlign = text.AlignStart, text.AlignCenter
	text.Draw(dst, string(r), f, op)
}

// encreInk is the bave's own ink colour, Encre profonde — the darkest of the
// palette's Encres (ENCRE_02 §3), never quite the pure black the charte
// forbids.
var encreInk = color.RGBA{R: 0x16, G: 0x15, B: 0x2A, A: 0xFF}

// fadeAlpha returns col with its alpha scaled by a, clamped to [0,1].
func fadeAlpha(col color.RGBA, a float64) color.RGBA {
	col.A = uint8(float64(col.A) * min(max(a, 0), 1))
	return col
}

// drawSpeaker draws the replay haut-parleur at [ui.Screen.SpeakerRect] —
// moved off the card's top-right corner to the keyboard's own left margin
// by encre-cs5.2's own correction of ENCRE_07 §4bis (Hoober 2013: the top
// corners are the hardest zone for a one-handed thumb, and this is tapped
// nearly every word). Tapping it plays the scratch sound this prototype
// already carries; the parent's own recorded voice (ENCRE_02 §13) is a
// content ticket's, not this drawing one's.
func (c *client) drawSpeaker(dst *ebiten.Image) {
	x, y, w, h := c.screen.SpeakerRect()
	cx, cy := float32(x)+float32(w)/2, float32(y)+float32(h)/2
	r := float32(min(w, h)) / 2
	vector.FillCircle(dst, cx, cy, r, parcheminVieux, true)
	vector.FillCircle(dst, cx, cy, r-2, parcheminClair, true)

	// A simple speaker glyph: a body and two sound arcs, Cuir on Parchemin
	// clair — no words needed to read what it does.
	bodyW, bodyH := r*0.5, r*0.7
	vector.FillRect(dst, cx-bodyW, cy-bodyH/2, bodyW, bodyH, cuir, true)
	const arcSteps = 8
	for _, ir := range []float32{r * 0.45, r * 0.65} {
		for s := range arcSteps {
			a0 := -0.5 + float64(s)/arcSteps
			a1 := -0.5 + float64(s+1)/arcSteps
			x0, y0 := cx+ir*float32(math.Cos(a0)), cy+ir*float32(math.Sin(a0))
			x1, y1 := cx+ir*float32(math.Cos(a1)), cy+ir*float32(math.Sin(a1))
			vector.StrokeLine(dst, x0, y0, x1, y1, 1.5, cuir, true)
		}
	}
}

// speakerTapped reports whether (x, y) landed on [client.drawSpeaker]'s own
// touch area.
func (c *client) speakerTapped(x, y int) bool {
	sx, sy, sw, sh := c.screen.SpeakerRect()
	return x >= sx && x < sx+sw && y >= sy && y < sy+sh
}
