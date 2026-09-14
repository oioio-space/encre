package assets_test

import (
	"testing"

	"github.com/oioio-space/encre/client/assets"
	"github.com/oioio-space/encre/client/ui"
)

// required is every rune the drawn keyboard can type on either layout plus
// the ENCRE_02 §5 acceptance chain — the same set ticket encre-amh subsetted
// the fonts against.
func required() string {
	return ui.RequiredRunes(ui.Phone) + ui.RequiredRunes(ui.AZERTY) +
		"éèêëàâùûîïôöçœÉÈÀÇŒ"
}

// TestEmbeddedFontsCoverEveryRequiredRune fails the moment any of the three
// embedded fonts is swapped for one that cannot draw a rune the keyboard can
// type — an empty box reaching a child mid-word (ENCRE_04 §2).
func TestEmbeddedFontsCoverEveryRequiredRune(t *testing.T) {
	req := required()
	tests := map[string][]byte{
		"FontGreffeTTF (Ark Pixel 10 px)": assets.FontGreffeTTF,
		"FontPlumeTTF (Ark Pixel 16 px)":  assets.FontPlumeTTF,
		"FontCursiveTTF (EncreCursive)":   assets.FontCursiveTTF,
	}
	for name, ttf := range tests {
		missing, err := ui.MissingGlyphs(ttf, req)
		if err != nil {
			t.Fatalf("%s: MissingGlyphs: %v", name, err)
		}
		if len(missing) > 0 {
			t.Errorf("%s cannot draw %d required rune(s): %q", name, len(missing), string(missing))
		}
	}
}
