package ui_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/oioio-space/encre/client/ui"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// gapFace draws everything except the runes in absent, so the check can be
// tested without depending on some real font's coverage quirks.
type gapFace struct {
	font.Face
	absent map[rune]bool
}

func (f gapFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	if f.absent[r] {
		return fixed.Rectangle26_6{}, 0, false
	}
	return f.Face.GlyphBounds(r)
}

func (f gapFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	if f.absent[r] {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}
	return f.Face.Glyph(dot, r)
}

func TestTheBitmapPlaceholderDrawsEveryAccentTheCharterAsksFor(t *testing.T) {
	// ENCRE_02 §5 fixes this chain as what any font of the game must carry,
	// capitals included, and expects the accents to need drawing by hand. This
	// placeholder needs none — which is the whole reason it was chosen.
	const charterChain = "éèêëàâùûîïôöçœÉÈÀÇŒ"

	missing := ui.MissingGlyphsInFace(bitmapfont.Face, charterChain+ui.RequiredRunes(ui.AZERTY))

	if len(missing) > 0 {
		t.Errorf("the bitmap placeholder cannot draw %q", string(missing))
	}
}

func TestMissingGlyphsInFaceNamesTheAbsentRunesInOrder(t *testing.T) {
	f := gapFace{Face: bitmapfont.Face, absent: map[rune]bool{'ç': true, 'ê': true}}

	missing := ui.MissingGlyphsInFace(f, "forêt garçon")

	if want := "êç"; string(missing) != want {
		t.Errorf("MissingGlyphsInFace = %q, want %q", string(missing), want)
	}
}

func TestMissingGlyphsInFaceReportsEachRuneOnce(t *testing.T) {
	f := gapFace{Face: bitmapfont.Face, absent: map[rune]bool{'é': true}}

	if missing := ui.MissingGlyphsInFace(f, "élévé"); len(missing) != 1 {
		t.Errorf("MissingGlyphsInFace on a repeated rune = %q, want it reported once", string(missing))
	}
}
