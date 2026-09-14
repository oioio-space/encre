package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

// minDistinctCells is the floor on how many of the [game.GlyphGrid]²=144
// cells two Couleur glyphs must disagree on to count as different shapes,
// not the same shape shifted or lightly redrawn. It is deliberately well
// above noise: a single recoloured losange would score 0 against itself.
const minDistinctCells = 20

// TestSixGlyphsAreDistinctInGrayscale is encre-amh.1's own acceptance test:
// convert the six Couleur glyphs to luminance — colour thrown away entirely,
// simulating what a deuteranope child sees — and check every pair still
// disagrees on enough cells to read as a different silhouette. brief/
// ENCRE_07 §3.4 names the pair a red-green confusion collides — Accentuées
// and Masquées — but this checks all fifteen pairs, since G111 asks for
// patterns distinct from one another, not from one particular neighbour.
func TestSixGlyphsAreDistinctInGrayscale(t *testing.T) {
	colors := engine.Colors()
	glyphs := make(map[engine.Color]game.ColorGlyph, len(colors))
	for _, c := range colors {
		glyphs[c] = game.Glyph(c)
	}

	for i, a := range colors {
		for _, b := range colors[i+1:] {
			ga, gb := glyphs[a], glyphs[b]

			// Luminance is the channel a grayscale simulation actually sees;
			// two glyphs that differ only by which colour tints them would
			// agree on every cell here.
			distinctLuma := 0
			for y := range game.GlyphGrid {
				for x := range game.GlyphGrid {
					if ga.Luminance(x, y) != gb.Luminance(x, y) {
						distinctLuma++
					}
				}
			}

			if distinctLuma < minDistinctCells {
				t.Errorf("Glyph(%s) vs Glyph(%s): %d cells differ in luminance, want >= %d",
					a, b, distinctLuma, minDistinctCells)
			}
			if got := ga.HammingDistance(gb); got != distinctLuma {
				t.Errorf("Glyph(%s).HammingDistance(Glyph(%s)) = %d, want %d (matches luminance)",
					a, b, got, distinctLuma)
			}
		}
	}
}

// TestEverySixGlyphsDrawsSomething catches a glyph function that silently
// returns an empty mask — indistinguishable from every other empty mask, and
// so a false pass of the distinctness test above were it not excluded here.
func TestEverySixGlyphsDrawsSomething(t *testing.T) {
	for _, c := range engine.Colors() {
		if n := game.Glyph(c).FilledCount(); n == 0 {
			t.Errorf("Glyph(%s).FilledCount() = 0, want a drawn shape", c)
		}
	}
}

// TestFamilyNameNamesEverySixColors documents that a placeholder frame always
// has something to write inside it while the real sprites of ENCRE_06 §9 do
// not exist yet.
func TestFamilyNameNamesEverySixColors(t *testing.T) {
	for _, c := range engine.Colors() {
		if game.FamilyName(c) == "" {
			t.Errorf("FamilyName(%s) = \"\", want a family name", c)
		}
	}
}

// TestGlyphOfAnUnknownColorIsEmptyNotAPanic documents [game.Glyph]'s
// contract on a Color outside the six, the same discipline [engine.Color.
// String] keeps.
func TestGlyphOfAnUnknownColorIsEmptyNotAPanic(t *testing.T) {
	if n := game.Glyph(engine.Color(99)).FilledCount(); n != 0 {
		t.Errorf("Glyph(Color(99)).FilledCount() = %d, want 0", n)
	}
}
