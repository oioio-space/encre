package ui_test

import (
	"strings"
	"testing"

	"github.com/oioio-space/encre/client/ui"
	"golang.org/x/image/font/gofont/goregular"
)

func TestRequiredRunesCoverEveryKeyOfTheLayout(t *testing.T) {
	kb := ui.NewKeyboard(ui.AZERTY, phoneW, boardH)

	required := ui.RequiredRunes(ui.AZERTY)
	for _, k := range kb.Keys() {
		if k.Kind != ui.KeyRune {
			continue
		}
		if !strings.ContainsRune(required, k.Rune) {
			t.Errorf("key %q is on the keyboard but not in RequiredRunes", k.Rune)
		}
	}
}

func TestPlaceholderFontDrawsEveryRuneTheKeyboardCanType(t *testing.T) {
	// The startup check ENCRE_04 §2 asks for, run against the font actually
	// shipped. When La Plume replaces it, this is what says whether its accents
	// were drawn — the brief expects they will have to be.
	missing, err := ui.MissingGlyphs(goregular.TTF, ui.RequiredRunes(ui.AZERTY))
	if err != nil {
		t.Fatalf("MissingGlyphs: %v", err)
	}
	if len(missing) > 0 {
		t.Errorf("placeholder font cannot draw %q", string(missing))
	}
}

func TestMissingGlyphsNamesWhatTheFontCannotDraw(t *testing.T) {
	// The Go fonts carry no CJK, which makes this a stable negative.
	missing, err := ui.MissingGlyphs(goregular.TTF, "a漢z")
	if err != nil {
		t.Fatalf("MissingGlyphs: %v", err)
	}
	if want := []rune{'漢'}; string(missing) != string(want) {
		t.Errorf("MissingGlyphs(goregular, %q) = %q, want %q", "a漢z", string(missing), string(want))
	}
}

func TestMissingGlyphsRejectsSomethingThatIsNotAFont(t *testing.T) {
	if _, err := ui.MissingGlyphs([]byte("not a font"), "a"); err == nil {
		t.Error("MissingGlyphs on junk bytes returned no error, want one")
	}
}

func TestLoadFaceRefusesAFontThatCannotDrawWhatIsAskedOfIt(t *testing.T) {
	// "échouer bruyamment s'il en manque" (ENCRE_04 §2): a silently missing
	// accent would reach the child as an empty box mid-word.
	_, err := ui.LoadFace(goregular.TTF, 12, "a漢")
	if err == nil {
		t.Fatal("LoadFace with an undrawable rune returned no error, want one")
	}
	if !strings.ContainsRune(err.Error(), '漢') {
		t.Errorf("LoadFace error = %q, want it to name the missing rune 漢", err)
	}
}

func TestLoadFaceReturnsAUsableFaceWhenEveryRuneIsPresent(t *testing.T) {
	face, err := ui.LoadFace(goregular.TTF, 12, ui.RequiredRunes(ui.AZERTY))
	if err != nil {
		t.Fatalf("LoadFace: %v", err)
	}
	if face == nil {
		t.Fatal("LoadFace returned a nil face and no error")
	}
	if face.Size != 12 {
		t.Errorf("face.Size = %v, want 12", face.Size)
	}
}
