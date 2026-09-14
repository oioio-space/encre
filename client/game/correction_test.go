package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
)

// TestCorrectionFindsTheFirstMismatch holds the "correction sans texte" of
// bead encre-cs5: no sentence of explanation, only the first letter that
// diverges from the target — that is what blinks while the child retypes.
func TestCorrectionFindsTheFirstMismatch(t *testing.T) {
	tests := []struct {
		target, typed string
		wantIndex     int
	}{
		{"école", "école", -1}, // correct: nothing to blink
		{"école", "ecole", 0},  // the missing accent, on the very first letter
		{"école", "écol", 4},   // too short: the mismatch is where it runs out
		{"école", "écoles", 5}, // too long: the mismatch is the extra letter
		{"", "", -1},
		{"chat", "", 0},
	}
	for _, tt := range tests {
		if got := game.FirstMismatch(tt.target, tt.typed); got != tt.wantIndex {
			t.Errorf("FirstMismatch(%q, %q) = %d, want %d", tt.target, tt.typed, got, tt.wantIndex)
		}
	}
}

// TestNewCorrectionOnACorrectAttemptNeedsNoRetype holds that a correct word
// never enters correction at all.
func TestNewCorrectionOnACorrectAttemptNeedsNoRetype(t *testing.T) {
	c := game.NewCorrection("chat", "chat")
	if c.NeedsRetype() {
		t.Error("NeedsRetype() on a correct attempt = true, want false")
	}
}

// TestCorrectionRetypeClearsBackToTheMismatch holds the retyping itself: the
// child keeps what was right and rewrites from the missed letter on, letter
// by letter, exactly as brief/ENCRE_06 §6's `bave` shows a letter appearing.
func TestCorrectionRetypeClearsBackToTheMismatch(t *testing.T) {
	c := game.NewCorrection("école", "ecole")
	if !c.NeedsRetype() {
		t.Fatal("NeedsRetype() on a wrong attempt = false, want true")
	}
	if got := c.MismatchIndex(); got != 0 {
		t.Fatalf("MismatchIndex() = %d, want 0", got)
	}
	// The mismatch falls on the word's very first letter, so nothing was kept.
	if got := c.Kept(); got != "" {
		t.Errorf("Kept() = %q, want %q", got, "")
	}
}

func TestCorrectionKeptOnALaterMismatch(t *testing.T) {
	c := game.NewCorrection("école", "écoles")
	if got, want := c.Kept(), "école"; got != want {
		t.Errorf("Kept() = %q, want %q", got, want)
	}
}
