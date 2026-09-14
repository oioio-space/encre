package ui_test

import (
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

// fixedAdvance answers every rune with the same width, so the arithmetic under
// test is not entangled with any real font's own metrics.
func fixedAdvance(w float64) func(rune) float64 {
	return func(rune) float64 { return w }
}

func TestTrackedOpensFifteenToTwentyPercentOfTheSizeBetweenLetters(t *testing.T) {
	// Zorzi, Barbiero, Facoetti et al. (PNAS 2012): +15-20% of the body doubles
	// a child's reading accuracy, immediately and without training.
	const size = 48.0

	positions, _ := ui.Tracked("ab", size, fixedAdvance(20))

	if len(positions) != 2 {
		t.Fatalf("Tracked(%q) returned %d position(s), want 2", "ab", len(positions))
	}
	gap := positions[1] - positions[0] - 20 // the advance of 'a' is 20
	if ratio := gap / size; ratio < 0.15 || ratio > 0.20 {
		t.Errorf("tracking opened %.1f%% of the size, want 15%%-20%%", ratio*100)
	}
}

func TestTrackedWidthCoversEveryGlyphAndTheGapsBetweenThem(t *testing.T) {
	positions, width := ui.Tracked("mot", 48, fixedAdvance(10))

	if got, want := positions, []float64{0, 10 + 48*ui.TrackingRatio, 2*(10+48*ui.TrackingRatio) + 10 - 10}; len(got) != len(want) {
		t.Fatalf("Tracked(%q) = %v, want %d position(s)", "mot", got, len(want))
	}
	// The whole string, letters and the tracking between them, but no tracking
	// trailing the last letter — trailing space would throw off anything
	// centred on this width.
	wantWidth := 3*10 + 2*48*ui.TrackingRatio
	if width != wantWidth {
		t.Errorf("Tracked(%q) width = %v, want %v", "mot", width, wantWidth)
	}
}

func TestTrackedOnASingleLetterAddsNoTrailingGap(t *testing.T) {
	positions, width := ui.Tracked("m", 48, fixedAdvance(10))

	if len(positions) != 1 || positions[0] != 0 {
		t.Errorf("Tracked(%q) positions = %v, want [0]", "m", positions)
	}
	if width != 10 {
		t.Errorf("Tracked(%q) width = %v, want 10 (no tracking to trail)", "m", width)
	}
}

func TestTrackedOnEmptyStringReturnsNothing(t *testing.T) {
	positions, width := ui.Tracked("", 48, fixedAdvance(10))

	if len(positions) != 0 || width != 0 {
		t.Errorf("Tracked(\"\") = %v, %v, want nil, 0", positions, width)
	}
}
