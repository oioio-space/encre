package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

// drawGlyph draws the six-shape, colour-blind-safe glyph of [game.Glyph] for
// c inside the size×size square whose top-left corner is (x, y) — ticket
// encre-amh.1: a shape, not a losange recoloured, so a child who cannot yet
// read the Couleur's name still recognises it, and a child who cannot tell
// the teinte from another still tells the shape.
//
// It blits [game.ColorGlyph]'s own grid cell by cell, at nearest-neighbour
// (no anti-aliasing): the same pixel-art discipline every other surface of
// the game keeps (ENCRE_02 §15), and the one that lets the test of
// client/game/colorglyph_test.go and what is actually drawn stay the same
// shape rather than drift apart under a smoothing filter.
func drawGlyph(dst *ebiten.Image, c engine.Color, x, y, size float32, ink color.Color) {
	glyph := game.Glyph(c)
	cell := size / float32(game.GlyphGrid)
	for row := range game.GlyphGrid {
		for col := range game.GlyphGrid {
			if !glyph[row][col] {
				continue
			}
			vector.FillRect(dst, x+float32(col)*cell, y+float32(row)*cell, cell+0.5, cell+0.5, ink, false)
		}
	}
}
