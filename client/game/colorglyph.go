package game

import "github.com/oioio-space/encre/engine"

// GlyphGrid is how wide and tall a [ColorGlyph] mask is, in cells. A card
// glyph is small — brief/ENCRE_02 §7 draws it a few pixels across — so a
// coarse grid is what the drawing code and this package's own test agree on,
// rather than either resampling a finer shape down.
const GlyphGrid = 12

// ColorGlyph is a silhouette, filled cell by filled cell on a [GlyphGrid]
// square: true where the glyph's ink falls, false where the parchment shows
// through. It carries no colour of its own — that is Color's own teinte,
// applied wherever this mask is drawn — which is what makes it survive being
// converted to grayscale: the shape is the whole signal.
type ColorGlyph [GlyphGrid][GlyphGrid]bool

// Glyph returns the silhouette for c: six shapes distinct enough to read
// without colour (brief/ENCRE_07 §3.4, ticket encre-amh.1), each echoing the
// family's own silhouette from brief/ENCRE_02 §8 rather than one shape
// recoloured six times — a losange recoloured satisfies WCAG 1.4.1's letter
// but not technique G111's "use patterns", and it is exactly the two Couleurs
// a red-green confusion collides — Accentuées #D9525C and Masquées #6FB57A —
// that most need telling apart by shape.
//
// An unknown Color returns the zero glyph — an empty mask, not a crash — the
// same discipline [Color.String] keeps for a value outside the six.
func Glyph(c engine.Color) ColorGlyph {
	switch c {
	case engine.Muettes:
		return glyphDrop() // Fantômes: goutte allongée, un ovale debout
	case engine.Jumelles:
		return glyphTwinDrops() // Jumeaux: deux gouttes collées, côte à côte
	case engine.Accentuees:
		return glyphSpikedDrop() // Couronnés: goutte couronnée de piques, l'aigrette
	case engine.Masquees:
		return glyphMaskedDrop() // Masqués: goutte ronde barrée du masque
	case engine.Sosies:
		return glyphBlurredDrop() // Métamorphes: deux silhouettes décalées
	case engine.Accordees:
		return glyphTripleDrop() // Meutes: trois petites gouttes, jamais seules
	default:
		return ColorGlyph{}
	}
}

// glyphDrop is a tall, narrow ellipse standing on the grid's own centre
// column: Muettes, the Fantôme's goutte allongée flottante.
func glyphDrop() ColorGlyph {
	return fillCells(func(x, y int) bool {
		return inEllipse(x, y, 5.5, 6, 2.6, 5.4)
	})
}

// glyphTwinDrops is two small circles side by side, touching at the middle:
// Jumelles, the Jumeaux standing symétriques.
func glyphTwinDrops() ColorGlyph {
	return fillCells(func(x, y int) bool {
		return inEllipse(x, y, 3.6, 6, 2.9, 2.9) || inEllipse(x, y, 8.4, 6, 2.9, 2.9)
	})
}

// glyphSpikedDrop is a stout circle crowned by three points: Accentuées, the
// Couronné's aigrette — the shape that most needs to differ from
// glyphMaskedDrop below, since the two Couleurs are what a red-green
// confusion collides (brief/ENCRE_07 §3.4).
func glyphSpikedDrop() ColorGlyph {
	body := func(x, y int) bool { return inEllipse(x, y, 5.5, 7, 4.2, 3.8) }
	spike := func(x, y, cx int, h float64) bool {
		dx := float64(x - cx)
		top := 1.5
		return float64(y) >= top && float64(y) <= top+h && dx*dx <= (h+top-float64(y))*0.7
	}
	return fillCells(func(x, y int) bool {
		return body(x, y) || spike(x, y, 3, 3) || spike(x, y, 5, 4.2) || spike(x, y, 8, 3)
	})
}

// glyphMaskedDrop is a round drop with a horizontal band crossing it at the
// eyes: Masquées, the Masqué's masque de parchemin.
func glyphMaskedDrop() ColorGlyph {
	return fillCells(func(x, y int) bool {
		inBody := inEllipse(x, y, 5.5, 6, 4.4, 4.6)
		inBand := y >= 5 && y <= 6
		return inBody && !inBand
	})
}

// glyphBlurredDrop is two overlapping drops, one offset from the other: the
// Métamorphe's own silhouette floue — two shapes where the other five have
// one, so it reads as motion-blurred even before it is animated.
func glyphBlurredDrop() ColorGlyph {
	return fillCells(func(x, y int) bool {
		return inEllipse(x, y, 4.5, 5, 3.6, 3.6) || inEllipse(x, y, 7.2, 7.2, 3.6, 3.6)
	})
}

// glyphTripleDrop is three small circles in a loose triangle: Accordées, the
// Meute qui n'est jamais seule.
func glyphTripleDrop() ColorGlyph {
	return fillCells(func(x, y int) bool {
		return inEllipse(x, y, 5.5, 2.8, 2.1, 2.1) ||
			inEllipse(x, y, 2.6, 8.2, 2.1, 2.1) ||
			inEllipse(x, y, 8.4, 8.2, 2.1, 2.1)
	})
}

// inEllipse reports whether the cell (x, y) falls inside the ellipse centred
// on (cx, cy) with the given half-width rx and half-height ry, cell centres
// taken at their own integer coordinate plus one half.
func inEllipse(x, y int, cx, cy, rx, ry float64) bool {
	dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
	return (dx*dx)/(rx*rx)+(dy*dy)/(ry*ry) <= 1
}

// fillCells builds a ColorGlyph by asking filled(x, y) about every cell of
// the grid.
func fillCells(filled func(x, y int) bool) ColorGlyph {
	var g ColorGlyph
	for y := range GlyphGrid {
		for x := range GlyphGrid {
			g[y][x] = filled(x, y)
		}
	}
	return g
}

// Luminance returns the grayscale value of cell (x, y) as a simulation of
// what the child sees with no colour signal at all — a deuteranope's own
// worst case, and what technique G111 asks a shape to survive: ink (true)
// reads as Encre profonde's own dark, parchment (false) as Parchemin clair's
// own light, since neither Couleur teinte ever changes what those two
// surfaces are.
func (g ColorGlyph) Luminance(x, y int) float64 {
	if g[y][x] {
		return 0.12 // Encre profonde-ish: near-black ink
	}
	return 0.93 // Parchemin clair-ish: near-white page
}

// FilledCount is how many cells of the grid this glyph fills, for a caller
// that wants a density figure rather than the mask itself.
func (g ColorGlyph) FilledCount() int {
	n := 0
	for _, row := range g {
		for _, cell := range row {
			if cell {
				n++
			}
		}
	}
	return n
}

// familyNames ties each Couleur to the creature family that signs it
// (brief/ENCRE_02 §8's own table): the name a placeholder frame carries until
// the eighteen sprites of ENCRE_06 §9 exist.
var familyNames = [...]string{
	engine.Muettes:    "Fantômes",
	engine.Jumelles:   "Jumeaux",
	engine.Accentuees: "Couronnés",
	engine.Masquees:   "Masqués",
	engine.Sosies:     "Métamorphes",
	engine.Accordees:  "Meutes",
}

// FamilyName returns the creature family that signs c, for a card's
// placeholder frame ([Glyph] already gives it its own shape; this gives it
// its own name). An unknown Color returns "", the same discipline
// [Color.String] keeps for a value outside the six.
func FamilyName(c engine.Color) string {
	if c < 0 || int(c) >= len(familyNames) {
		return ""
	}
	return familyNames[c]
}

// HammingDistance is how many cells g and h disagree on: the count a shape
// test reads as "how different are these two silhouettes", since a glyph
// that is merely the other one recoloured leaves every cell in agreement
// once colour is thrown away.
func (g ColorGlyph) HammingDistance(h ColorGlyph) int {
	n := 0
	for y := range GlyphGrid {
		for x := range GlyphGrid {
			if g[y][x] != h[y][x] {
				n++
			}
		}
	}
	return n
}
