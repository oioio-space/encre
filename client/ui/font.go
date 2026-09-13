package ui

import (
	"bytes"
	"fmt"

	gotext "github.com/go-text/typesetting/font"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font"
)

// RequiredRunes returns every character the keyboard of layout l can type, and
// which the game must therefore be able to draw.
//
// ENCRE_02 §5 asks a wider chain of the real fonts than this — it adds ë â û ï
// ö œ and the capitals É È À Ç Œ. That is the acceptance test of the font
// itself, a Design deliverable; this is the narrower thing the keyboard needs.
func RequiredRunes(l Layout) string {
	runes := accentRow
	for _, row := range letterRows[l] {
		runes += row
	}
	return runes
}

// MissingGlyphs reports, in the order they appear, which runes of s the font in
// ttf has no glyph for. It asks the shaper that will actually draw them rather
// than a table of what the font claims, so the answer is what the child would
// see. A rune appearing twice is reported once.
func MissingGlyphs(ttf []byte, s string) ([]rune, error) {
	face, err := gotext.ParseTTF(bytes.NewReader(ttf))
	if err != nil {
		return nil, fmt.Errorf("parsing font: %w", err)
	}

	var missing []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if seen[r] {
			continue
		}
		seen[r] = true
		if _, ok := face.NominalGlyph(r); !ok {
			missing = append(missing, r)
		}
	}
	return missing, nil
}

// LoadFace parses ttf at the given size in logical pixels, refusing to return a
// face that cannot draw every rune of required.
//
// The refusal is the point (ENCRE_04 §2). A missing accent does not crash: it
// reaches the child mid-word as an empty box, in a game whose whole subject is
// spelling the accent correctly. Failing at startup is the cheaper mistake.
func LoadFace(ttf []byte, size float64, required string) (*text.GoTextFace, error) {
	missing, err := MissingGlyphs(ttf, required)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("font cannot draw %d required rune(s): %q", len(missing), string(missing))
	}

	src, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		return nil, fmt.Errorf("loading font face: %w", err)
	}
	return &text.GoTextFace{Source: src, Size: size}, nil
}

// MissingGlyphsInFace reports, in the order they appear, which runes of s the
// face cannot draw. A rune appearing twice is reported once.
//
// It exists alongside [MissingGlyphs] because the two kinds of font this game
// will carry answer in different ways: the real ones of ENCRE_02 §5 arrive as
// TTF bytes, while a bitmap placeholder arrives already rasterised as a
// font.Face, with no outline table to interrogate.
func MissingGlyphsInFace(f font.Face, s string) []rune {
	var missing []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if seen[r] {
			continue
		}
		seen[r] = true
		if _, _, ok := f.GlyphBounds(r); !ok {
			missing = append(missing, r)
		}
	}
	return missing
}
