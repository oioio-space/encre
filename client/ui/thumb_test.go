package ui_test

import (
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

// TestSpeakerAndWagerButtonsFallInTheBottomThird holds encre-cs5.2: Hoober's
// 1,333 field observations (2013) name a phone's top corners the hardest zone
// for a one-handed thumb, the bottom the easiest, and both the replay speaker
// and the two wager buttons are tapped nearly every word — so the corrected
// layout keeps every one of them at or below two thirds of the way down,
// never at the card's original top-right corner.
func TestSpeakerAndWagerButtonsFallInTheBottomThird(t *testing.T) {
	s := ui.NewScreen(ui.PortraitWidth, ui.PortraitHeight)
	third := s.H * 2 / 3

	sx, sy, sw, sh := s.SpeakerRect()
	if sy < third {
		t.Errorf("speaker top is at y=%d, want at or below the bottom third boundary y=%d", sy, third)
	}
	if sw < 48 || sh < 48 {
		t.Errorf("speaker is %dx%d, want at least 48x48 (ENCRE_02 §11's touch floor)", sw, sh)
	}
	if sx < 0 || sx+sw > s.W || sy+sh > s.H {
		t.Errorf("speaker at (%d,%d)+%dx%d escapes the %dx%d screen", sx, sy, sw, sh, s.W, s.H)
	}

	listenTwice, listenOnce := s.WagerRects()
	for name, r := range map[string]ui.Rect{"listenTwice": listenTwice, "listenOnce": listenOnce} {
		if r.Y < third {
			t.Errorf("%s top is at y=%d, want at or below the bottom third boundary y=%d", name, r.Y, third)
		}
		if r.H < 48 {
			t.Errorf("%s is %d px tall, want at least 48 (ENCRE_02 §11's touch floor)", name, r.H)
		}
		if r.X < 0 || r.X+r.W > s.W || r.Y+r.H > s.H {
			t.Errorf("%s at (%d,%d)+%dx%d escapes the %dx%d screen", name, r.X, r.Y, r.W, r.H, s.W, s.H)
		}
	}
}

// TestTheTwoWagerButtonsAreTheSameSizeAndDoNotOverlap holds ENCRE_06 §7: two
// buttons of the same size, side by side.
func TestTheTwoWagerButtonsAreTheSameSizeAndDoNotOverlap(t *testing.T) {
	s := ui.NewScreen(ui.PortraitWidth, ui.PortraitHeight)
	listenTwice, listenOnce := s.WagerRects()

	if listenTwice.W != listenOnce.W || listenTwice.H != listenOnce.H {
		t.Errorf("listenTwice is %dx%d, listenOnce is %dx%d, want the same size",
			listenTwice.W, listenTwice.H, listenOnce.W, listenOnce.H)
	}
	if listenTwice.H < 80 || listenTwice.H > 92 {
		t.Errorf("wager button height = %d, want close to the ~86 px of ENCRE_06 §7", listenTwice.H)
	}
	if listenTwice.X+listenTwice.W > listenOnce.X {
		t.Errorf("listenTwice (right edge %d) overlaps listenOnce (left edge %d)",
			listenTwice.X+listenTwice.W, listenOnce.X)
	}
}
