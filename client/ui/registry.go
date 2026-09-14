package ui

import (
	"fmt"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font"

	"github.com/oioio-space/encre/client/assets"
)

// FaceRole names one of the three faces of brief/ENCRE_02 §5.
type FaceRole int

const (
	// Plume is running text: the word, labels, everything that is not a
	// number or the Enluminure.
	Plume FaceRole = iota
	// Greffe is digits and counters: the score, the timers.
	Greffe
	// Cursive is the Enluminure, and nothing else.
	Cursive
)

// String names the role, for diagnostics and error messages.
func (r FaceRole) String() string {
	switch r {
	case Plume:
		return "Plume"
	case Greffe:
		return "Greffe"
	case Cursive:
		return "Cursive"
	default:
		return fmt.Sprintf("FaceRole(%d)", int(r))
	}
}

// Registry holds the faces the game draws with, keyed by role. Every face
// registered is checked against a set of required runes first, so a font
// missing an accented glyph is refused here, at startup, rather than reaching
// a child as an empty box mid-word (ENCRE_04 §2).
//
// Its faces are [text.Face], the interface both a bitmap placeholder wrapped
// in [text.GoXFace] and a real TTF loaded as a [text.GoTextFace] satisfy: a
// caller that only ever asks a Registry for a role's face does not change when
// a bitmap placeholder is replaced by the TTF of ticket encre-amh.
type Registry struct {
	faces map[FaceRole]text.Face
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{faces: make(map[FaceRole]text.Face)}
}

// SetBitmapFace registers face for role r, refusing it if it cannot draw every
// rune of required. face is a placeholder: a bitmap font arrives already
// rasterised, with no outline table LoadFace could interrogate, which is why
// this takes a [font.Face] rather than TTF bytes.
func (reg *Registry) SetBitmapFace(r FaceRole, face font.Face, required string) error {
	if missing := MissingGlyphsInFace(face, required); len(missing) > 0 {
		return fmt.Errorf("ui: face %s cannot draw %d required rune(s): %q", r, len(missing), string(missing))
	}
	reg.faces[r] = text.NewGoXFace(face)
	return nil
}

// SetTTFFace registers a TTF face for role r at size logical pixels, refusing
// it if it cannot draw every rune of required (ENCRE_04 §2). This is the path
// ticket encre-amh wires the real faces of ENCRE_02 §5 through, once they
// exist, without any caller of [Registry.Face] having to change.
func (reg *Registry) SetTTFFace(r FaceRole, ttf []byte, size float64, required string) error {
	face, err := LoadFace(ttf, size, required)
	if err != nil {
		return fmt.Errorf("ui: face %s: %w", r, err)
	}
	reg.faces[r] = face
	return nil
}

// Face returns the face registered for role r, and whether one was.
func (reg *Registry) Face(r FaceRole) (text.Face, bool) {
	f, ok := reg.faces[r]
	return f, ok
}

// Native pixel grids of the two pixel faces ticket encre-amh chose (ENCRE_02
// §15): a face drawn at any size that is not a whole multiple of its grid
// resamples its glyphs off it, which is what turns pixel art into mush.
// La Cursive is vectorial and carries no such constraint.
const (
	GreffeGrid = 10 // Ark Pixel 10 px proportional latin
	PlumeGrid  = 16 // Ark Pixel 16 px proportional latin
)

// Default sizes [NewDefaultRegistry] loads each role at, in logical pixels —
// whole multiples of the grid above, taken from brief/ENCRE_02 §5's own
// examples rather than picked independently:
//
//   - DefaultPlumeSize is 48 (3×PlumeGrid), the size §5 gives the word in
//     cours, La Plume's most prominent use.
//   - DefaultGreffeSize is 20 (2×GreffeGrid): the interface labels §6 of
//     ENCRE_06 asks for at 14 px logiques round up, never down, to the next
//     grid step, and the ENCRE_06 §8 floor of 14 px for text a child reads
//     forces the same rounding for every other Greffe use.
//   - DefaultCursiveSize has no grid to round to; 32 matches the height an
//     Enluminure word needs beside the card it replaces.
const (
	DefaultPlumeSize   = 3 * PlumeGrid
	DefaultGreffeSize  = 2 * GreffeGrid
	DefaultCursiveSize = 32
)

// NewDefaultRegistry returns a Registry with all three faces of brief/ENCRE_02
// §5 set to the real fonts ticket encre-amh chose — Ark Pixel 10 px for Le
// Greffe, Ark Pixel 16 px for La Plume, EncreCursive (a renamed, subsetted
// Marelle) for La Cursive — each loaded at its DefaultXSize and checked
// against required. A font missing a rune required refuses here, at startup,
// naming it, rather than reaching a child as an empty box mid-word
// (ENCRE_04 §2).
func NewDefaultRegistry(required string) (*Registry, error) {
	reg := NewRegistry()
	fonts := map[FaceRole]struct {
		ttf  []byte
		size float64
	}{
		Plume:   {assets.FontPlumeTTF, DefaultPlumeSize},
		Greffe:  {assets.FontGreffeTTF, DefaultGreffeSize},
		Cursive: {assets.FontCursiveTTF, DefaultCursiveSize},
	}
	for _, role := range []FaceRole{Plume, Greffe, Cursive} {
		f := fonts[role]
		if err := reg.SetTTFFace(role, f.ttf, f.size, required); err != nil {
			return nil, err
		}
	}
	return reg, nil
}
