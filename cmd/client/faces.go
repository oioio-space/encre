package main

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/oioio-space/encre/client/assets"
	"github.com/oioio-space/encre/client/ui"
)

// interfaceExtraRunes are the characters the run screen's own interface text
// needs beyond what the drawn keyboard can ever type (ui.RequiredRunes):
// digits, the Couleur names in capitals — ENCRE_02 §15's one exception to
// "no capitals outside the logo" — and the punctuation of a label or a
// question. Checking these at startup, alongside the keyboard's own runes,
// is what keeps ENCRE_04 §2's rule: a missing glyph is refused here, named,
// rather than reaching a child as an empty box mid-word.
const interfaceExtraRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZÉÈÀÇŒ0123456789:'?…"

// The Le Greffe sizes the run screen draws with, both whole multiples of
// [ui.GreffeGrid] (ENCRE_02 §15): greffeLabelSize is what every label 14 px
// logiques and up rounds up to (ENCRE_06 §8's floor), greffeScoreSize is the
// 30 px brief/ENCRE_06 §4 gives the score number itself.
const (
	greffeLabelSize = 2 * ui.GreffeGrid // 20
	greffeScoreSize = 3 * ui.GreffeGrid // 30
)

// faces holds every face the run screen draws text with, loaded once at
// startup from the same TTFs [ui.Registry] carries (ticket encre-amh's real
// fonts), each refused if it cannot draw everything this screen asks of it.
type faces struct {
	plumeWord   *text.GoTextFace // La Plume, 48 px: the word — the biggest character on screen (ENCRE_06 §4).
	greffeLabel *text.GoTextFace // Le Greffe, 20 px: every other label a child reads.
	greffeScore *text.GoTextFace // Le Greffe, 30 px: the score number alone.
}

// newFaces loads the three faces the run screen needs, refusing any that
// cannot draw everything the keyboard can type plus interfaceExtraRunes.
func newFaces() (*faces, error) {
	required := ui.RequiredRunes(ui.Phone) + ui.RequiredRunes(ui.AZERTY) + "…·×→" + interfaceExtraRunes

	plumeWord, err := ui.LoadFace(assets.FontPlumeTTF, ui.DefaultPlumeSize, required)
	if err != nil {
		return nil, fmt.Errorf("la plume %dpx: %w", ui.DefaultPlumeSize, err)
	}
	greffeLabel, err := ui.LoadFace(assets.FontGreffeTTF, greffeLabelSize, required)
	if err != nil {
		return nil, fmt.Errorf("le greffe %dpx: %w", greffeLabelSize, err)
	}
	greffeScore, err := ui.LoadFace(assets.FontGreffeTTF, greffeScoreSize, required)
	if err != nil {
		return nil, fmt.Errorf("le greffe %dpx: %w", greffeScoreSize, err)
	}
	return &faces{plumeWord: plumeWord, greffeLabel: greffeLabel, greffeScore: greffeScore}, nil
}

// drawFace draws s centred on (cx, cy) with f, letter by letter, opening
// ui.TrackingRatio of space between each pair (encre-tfy.1's correction,
// ENCRE_06 §4 and §5): Zorzi et al. (PNAS 2012) measured this doubling a
// child's reading accuracy, immediately.
func drawFace(dst *ebiten.Image, s string, f text.Face, size float64, cx, cy float64, col color.Color) {
	drawFaceTracked(dst, s, f, size, ui.TrackingRatio, cx, cy, col)
}

// drawFaceTight draws s like [drawFace], but with no extra tracking opened
// between letters — for a short adult-facing caption (a wager button, a
// score label) that must fit a fixed box, rather than the word a child is
// reading letter by letter, which is what encre-tfy.1's tracking correction
// is for (see [ui.TrackingRatio]'s own doc).
func drawFaceTight(dst *ebiten.Image, s string, f text.Face, size float64, cx, cy float64, col color.Color) {
	drawFaceTracked(dst, s, f, size, 0, cx, cy, col)
}

func drawFaceTracked(dst *ebiten.Image, s string, f text.Face, size, ratio float64, cx, cy float64, col color.Color) {
	advance := func(r rune) float64 { g := string(r); return text.AdvanceAt(g, len(g), f) }
	positions, width := trackedAt(s, size, ratio, advance)

	left := cx - width/2
	for i, r := range []rune(s) {
		op := &text.DrawOptions{}
		op.GeoM.Translate(left+positions[i], cy)
		op.Filter = ebiten.FilterNearest
		op.ColorScale.ScaleWithColor(col)
		op.PrimaryAlign, op.SecondaryAlign = text.AlignStart, text.AlignCenter
		text.Draw(dst, string(r), f, op)
	}
}

// trackedAt is [ui.Tracked] with its own tracking ratio in place of
// [ui.TrackingRatio], for [drawFaceTight]'s zero.
func trackedAt(s string, size, ratio float64, advance func(r rune) float64) (positions []float64, width float64) {
	extra := size * ratio
	x := 0.0
	for _, r := range s {
		positions = append(positions, x)
		x += advance(r) + extra
	}
	if len(positions) > 0 {
		x -= extra
	}
	return positions, x
}
