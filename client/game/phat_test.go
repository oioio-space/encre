package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
)

// TestPHatPointsNeverShowsDigits holds brief/ENCRE_02 §12 and ENCRE_06 §5:
// p̂ is drawn as ten points, filled and empty, never as a number — "pas de
// chiffres là où des points suffisent".
func TestPHatPointsNeverShowsDigits(t *testing.T) {
	tests := []struct {
		p          float64
		wantFilled int
	}{
		{0, 0},
		{0.03, 0}, // pHatFloor: rounds down to zero filled dots, not one
		{0.5, 5},
		{0.65, 7},  // rounds to nearest, not truncates
		{0.98, 10}, // pHatCeiling: rounds up to a full card
		{1, 10},
		{-1, 0}, // out of PHat's own [0.03, 0.98] range, still clamped
		{2, 10},
	}
	for _, tt := range tests {
		filled, total := game.PHatPoints(tt.p)
		if total != 10 {
			t.Errorf("PHatPoints(%v) total = %d, want 10", tt.p, total)
		}
		if filled != tt.wantFilled {
			t.Errorf("PHatPoints(%v) filled = %d, want %d", tt.p, filled, tt.wantFilled)
		}
	}
}
