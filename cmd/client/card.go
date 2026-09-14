package main

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/ui"
	"github.com/oioio-space/encre/engine"
)

// The card's own bands, as a share of its rendered height or width rather
// than a fixed pixel count: brief/ENCRE_06 §5 gives them against the card at
// 180×240 (before ENCRE_07 §4.1's correction to 192×256, ×2); a ratio is
// what keeps them true at whichever whole scale [ui.Screen] picks.
const (
	cardRibbonHFrac    = 9.0 / 240.0  // ruban de Couleur, 9 px of a 240-tall card
	cardCreatureWFrac  = 88.0 / 180.0 // emplacement de créature, 88 px of a 180-wide card
	cardCreatureYFrac  = 0.14         // top margin under the ribbon, of the card's own height
	cardWordYFrac      = 0.62         // where "le mot à écrire" sits, of the card's own height
	cardPHatYFrac      = 0.80         // the p̂ row
	cardFooterYFrac    = 0.93         // glyph + Couleur name + jetons
	cardBorderInsetPx  = 2.0
	cardPHatDotSpacing = 1.6 // dot diameters between two p̂ dot centres
)

// couleurTeintes are the six Couleurs' own teinte (brief/ENCRE_02 §3), the
// only thing this palette signals: a ruban, a glyph, a score label, never a
// fill (ENCRE_02 §3's own rule).
var couleurTeintes = [...]color.RGBA{
	engine.Muettes:    {R: 0x9F, G: 0xC4, B: 0xE8, A: 0xFF},
	engine.Jumelles:   {R: 0xB0, G: 0x7C, B: 0xE0, A: 0xFF},
	engine.Accentuees: {R: 0xD9, G: 0x52, B: 0x5C, A: 0xFF},
	engine.Masquees:   {R: 0x6F, G: 0xB5, B: 0x7A, A: 0xFF},
	engine.Sosies:     {R: 0x5B, G: 0xC8, B: 0xC4, A: 0xFF},
	engine.Accordees:  {R: 0xE8, G: 0xA9, B: 0x4C, A: 0xFF},
}

// colorTeinte returns c's own teinte, or Encre lavée for a Color outside the
// six — a visible mistake rather than a panic, the same discipline
// [engine.Color.String] keeps.
func colorTeinte(c engine.Color) color.Color {
	if c < 0 || int(c) >= len(couleurTeintes) {
		return color.RGBA{R: 0x55, G: 0x55, B: 0x8A, A: 0xFF}
	}
	return couleurTeintes[c]
}

// cardBorderColor is the 2 px frame [game.CardState] draws with — the one
// cue of brief/ENCRE_02 §7's own state table this prototype has art for; the
// fill and the corner ornaments the table also names wait on the real
// sprites of ENCRE_06 §9.
func cardBorderColor(st game.CardState) color.Color {
	switch st {
	case game.CardGold:
		return or
	case game.CardCursed:
		return color.RGBA{R: 0xB3, G: 0x20, B: 0x2A, A: 0xFF} // Sang
	case game.CardTarnished:
		return color.RGBA{R: 0xB8, G: 0x86, B: 0x0B, A: 0xFF} // Or sombre
	case game.CardRencontre:
		return flamme
	default:
		return encre
	}
}

// drawCard draws the current card's recto in full — brief/ENCRE_06 §5's
// anatomy: the ruban de Couleur, the creature's own placeholder frame (see
// [client.drawCreaturePlaceholder]), the word in La Plume, p̂ in points
// rather than digits, and the glyph, the Couleur's name and the jetons.
func (c *client) drawCard(dst *ebiten.Image) {
	x, y := float32(c.cardX), float32(c.cardY)
	w, h := float32(c.cardW), float32(c.cardH)
	card := c.currentCard()
	colr := card.dominantColor()

	raised(dst, x, y, w, h, 2, parcheminClair, true)
	vector.StrokeRect(dst, x+1, y+1, w-2, h-2, cardBorderInsetPx, cardBorderColor(card.state), false)

	ribbonH := float32(c.cardH) * cardRibbonHFrac
	vector.FillRect(dst, x, y, w, ribbonH, colorTeinte(colr), false)

	side := float32(c.cardW) * cardCreatureWFrac
	cx := x + w/2
	creatureY := y + float32(c.cardH)*cardCreatureYFrac
	c.drawCreaturePlaceholder(dst, card, cx-side/2, creatureY, side)

	wordY := y + float32(c.cardH)*cardWordYFrac
	drawFace(dst, "le mot à écrire", c.faces.greffeLabel, greffeLabelSize, float64(cx), float64(wordY)-18, cuir)
	drawFace(dst, card.word.Text, c.faces.plumeWord, ui.DefaultPlumeSize, float64(cx), float64(wordY)+10, encre)

	c.drawPHat(dst, card, cx, y+float32(c.cardH)*cardPHatYFrac)
	c.drawCardFooter(dst, card, colr, cx, y+float32(c.cardH)*cardFooterYFrac)
}

// drawCreaturePlaceholder draws a named frame instead of the sprite ENCRE_06
// §9 has not shipped yet: a dashed border — deliberately unlike any
// finished surface in the game — and the creature family's own name
// ([game.FamilyName]), so a placeholder reads as unfinished rather than as a
// bug. Ticket encre-tfy.5's own brief asks for exactly this: "un cadre avec
// le nom de la famille de créature dedans vaut mieux que du gris".
func (c *client) drawCreaturePlaceholder(dst *ebiten.Image, card runCard, x, y, side float32) {
	const dash, gap = 4, 3
	for along := float32(0); along < side; along += dash + gap {
		end := min(along+dash, side)
		vector.StrokeLine(dst, x+along, y, x+end, y, 1, cuirClair, false)
		vector.StrokeLine(dst, x+along, y+side, x+end, y+side, 1, cuirClair, false)
		vector.StrokeLine(dst, x, y+along, x, y+end, 1, cuirClair, false)
		vector.StrokeLine(dst, x+side, y+along, x+side, y+end, 1, cuirClair, false)
	}
	family := game.FamilyName(card.dominantColor())
	drawFace(dst, family, c.faces.greffeLabel, greffeLabelSize, float64(x+side/2), float64(y+side/2), cuirClair)
}

// drawPHat draws p̂ as [game.PHatPoints] filled and empty points — "pas de
// chiffres là où des points suffisent" (ENCRE_02 §12) — the one place p̂
// reaches the screen.
func (c *client) drawPHat(dst *ebiten.Image, card runCard, cx, y float32) {
	filled, total := game.PHatPoints(card.pHat)
	const dotR = 3.0
	spacing := float32(dotR * 2 * cardPHatDotSpacing)
	left := cx - spacing*float32(total-1)/2
	for i := range total {
		x := left + float32(i)*spacing
		clr := parcheminVieux
		if i < filled {
			clr = braise
		}
		vector.FillCircle(dst, x, y, dotR, clr, true)
	}
}

// drawCardFooter draws the glyph, the Couleur's own name in capitals —
// ENCRE_02 §15's one exception to "no capitals outside the logo and the
// Couleur names at the score" — and the jetons, brief/ENCRE_02 §7's own
// footer row. Jetons are a count the anatomy diagram itself writes as a
// digit ("◆ MUETTE 18"): the "no digits" rule is p̂'s alone.
func (c *client) drawCardFooter(dst *ebiten.Image, card runCard, colr engine.Color, cx, y float32) {
	const glyphSide = 14.0
	drawGlyph(dst, colr, cx-70, y-glyphSide/2, glyphSide, encre)
	label := fmt.Sprintf("%s  %d", strings.ToUpper(colr.String()), card.tokens())
	drawFace(dst, label, c.faces.greffeLabel, greffeLabelSize, float64(cx)+16, float64(y), cuir)
}
