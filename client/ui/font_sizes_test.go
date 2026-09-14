package ui_test

import (
	"strings"
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

// TestDefaultFaceSizesAreWholeMultiplesOfTheirNativeGrid holds ENCRE_02 §15:
// a pixel face drawn at any other size resamples its glyphs off the grid.
func TestDefaultFaceSizesAreWholeMultiplesOfTheirNativeGrid(t *testing.T) {
	if ui.DefaultGreffeSize%ui.GreffeGrid != 0 {
		t.Errorf("DefaultGreffeSize = %v, not a whole multiple of GreffeGrid = %v", ui.DefaultGreffeSize, ui.GreffeGrid)
	}
	if ui.DefaultPlumeSize%ui.PlumeGrid != 0 {
		t.Errorf("DefaultPlumeSize = %v, not a whole multiple of PlumeGrid = %v", ui.DefaultPlumeSize, ui.PlumeGrid)
	}
}

// TestNoChildReadSizeFallsBelowTheFourteenPixelFloor holds the floor
// ENCRE_06 §8 sets for any text a child reads: no default face size may fall
// under it, even after being rounded up to its native grid.
func TestNoChildReadSizeFallsBelowTheFourteenPixelFloor(t *testing.T) {
	const floor = 14
	sizes := map[string]float64{
		"DefaultGreffeSize":  ui.DefaultGreffeSize,
		"DefaultPlumeSize":   ui.DefaultPlumeSize,
		"DefaultCursiveSize": ui.DefaultCursiveSize,
	}
	for name, size := range sizes {
		if size < floor {
			t.Errorf("%s = %v, want >= %v (ENCRE_06 §8 floor for text a child reads)", name, size, floor)
		}
	}
}

func TestNewDefaultRegistrySetsTheThreeRealFonts(t *testing.T) {
	required := ui.RequiredRunes(ui.Phone) + ui.RequiredRunes(ui.AZERTY) +
		"éèêëàâùûîïôöçœÉÈÀÇŒ"

	reg, err := ui.NewDefaultRegistry(required)
	if err != nil {
		t.Fatalf("NewDefaultRegistry: %v", err)
	}
	for _, role := range []ui.FaceRole{ui.Plume, ui.Greffe, ui.Cursive} {
		if _, ok := reg.Face(role); !ok {
			t.Errorf("Face(%v) reports no face from NewDefaultRegistry", role)
		}
	}
}

func TestNewDefaultRegistryRefusesAFontMissingARequiredRune(t *testing.T) {
	// 漢 is not in any of the three embedded fonts: this asks
	// NewDefaultRegistry for something none of them can draw, and checks the
	// refusal names it (ENCRE_04 §2's "échouer bruyamment").
	_, err := ui.NewDefaultRegistry("café漢")
	if err == nil {
		t.Fatal("NewDefaultRegistry with an undrawable rune returned no error, want one")
	}
	if !strings.ContainsRune(err.Error(), '漢') {
		t.Errorf("error = %q, want it to name the missing rune 漢", err)
	}
}
