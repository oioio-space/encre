package ui_test

import (
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

// viewports is the spread the game has to survive. Browsers resize at will, a
// phone rotates in a pocket, and a tablet is held either way — so the layout is
// checked against the whole range rather than the two sizes it was drawn for.
var viewports = []struct {
	name string
	w, h int
}{
	{"iPhone SE, the smallest phone still sold", 320, 568},
	{"a common Android", 360, 640},
	{"iPhone 8", 375, 667},
	{"iPhone 14, the reference of ENCRE_04 §2", 390, 844},
	{"iPhone 14 Plus", 428, 926},
	{"a phone on its side", 844, 390},
	{"iPad portrait", 768, 1024},
	{"iPad landscape", 1024, 768},
	{"iPad Pro portrait", 1024, 1366},
	{"a laptop", 1280, 720},
	{"a desktop", 1920, 1080},
	{"an ultra-wide", 3440, 1440},
	{"a 4K screen", 3840, 2160},
	{"a window squeezed to nothing", 240, 320},
	{"a window squeezed the other way", 320, 240},
	{"a nearly square window", 800, 800},
}

func TestTheScreenHoldsAtEverySize(t *testing.T) {
	for _, v := range viewports {
		t.Run(v.name, func(t *testing.T) {
			s := ui.NewScreen(v.w, v.h)

			if s.W <= 0 || s.H <= 0 {
				t.Fatalf("logical screen %dx%d, want both positive", s.W, s.H)
			}
			// Every band has to sit inside the screen, in order, without one
			// climbing over the next.
			switch {
			case s.CardY < 0 || s.CardY+s.CardH > s.H:
				t.Errorf("the card runs from %d to %d, outside a screen %d tall", s.CardY, s.CardY+s.CardH, s.H)
			case s.EntryY <= s.CardY+s.CardH:
				t.Errorf("the word being written sits at %d, on top of a card ending at %d", s.EntryY, s.CardY+s.CardH)
			case s.BoardY < 0 || s.BoardY+s.BoardH > s.H:
				t.Errorf("the keyboard runs from %d to %d, outside a screen %d tall", s.BoardY, s.BoardY+s.BoardH, s.H)
			case s.BoardX < 0 || s.BoardX+s.BoardW > s.W:
				t.Errorf("the keyboard runs from %d to %d, outside a screen %d wide", s.BoardX, s.BoardX+s.BoardW, s.W)
			case s.BoardW <= 0 || s.BoardH <= 0:
				t.Errorf("the keyboard is %dx%d, want a real area", s.BoardW, s.BoardH)
			}
		})
	}
}

func TestTheKeyboardNeverEscapesItsAreaAtAnySize(t *testing.T) {
	for _, v := range viewports {
		t.Run(v.name, func(t *testing.T) {
			s := ui.NewScreen(v.w, v.h)
			keys := ui.NewKeyboard(s.Layout, s.BoardW, s.BoardH).Keys()

			if len(keys) == 0 {
				t.Fatal("the keyboard has no keys")
			}
			for i, a := range keys {
				if a.X < 0 || a.Y < 0 || a.X+a.W > s.BoardW || a.Y+a.H > s.BoardH {
					t.Errorf("key %q at (%d,%d)+%dx%d escapes the %dx%d keyboard",
						a.Label(), a.X, a.Y, a.W, a.H, s.BoardW, s.BoardH)
				}
				if a.W <= 0 || a.H <= 0 {
					t.Errorf("key %q is %dx%d, want a real target", a.Label(), a.W, a.H)
				}
				for _, b := range keys[i+1:] {
					if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
						t.Errorf("keys %q and %q overlap", a.Label(), b.Label())
					}
				}
			}
		})
	}
}

func TestEveryKeyIsStillReachableAtAnySize(t *testing.T) {
	for _, v := range viewports {
		t.Run(v.name, func(t *testing.T) {
			s := ui.NewScreen(v.w, v.h)
			kb := ui.NewKeyboard(s.Layout, s.BoardW, s.BoardH)

			for _, want := range kb.Keys() {
				got, ok := kb.KeyAt(want.X+want.W/2, want.Y+want.H/2)
				if !ok || got.Label() != want.Label() {
					t.Errorf("the centre of %q reaches %q/%v", want.Label(), got.Label(), ok)
				}
			}
		})
	}
}

func TestTheWholeAlphabetSurvivesEverySize(t *testing.T) {
	// A layout that drops a letter when the window narrows would leave a child
	// unable to write a word, which is worse than an ugly keyboard.
	for _, v := range viewports {
		t.Run(v.name, func(t *testing.T) {
			s := ui.NewScreen(v.w, v.h)
			kb := ui.NewKeyboard(s.Layout, s.BoardW, s.BoardH)

			for _, r := range "abcdefghijklmnopqrstuvwxyz" + ui.AccentRow {
				if !kb.Accepts(r) {
					t.Errorf("%q is unreachable at %dx%d", r, v.w, v.h)
				}
			}
		})
	}
}

func TestPortraitTakesThePhoneKeyboardAndLandscapeTheAZERTY(t *testing.T) {
	// ENCRE_06 §2: seven alphabetical columns where the width is short, AZERTY
	// where it is not.
	if got := ui.NewScreen(390, 844).Layout; got != ui.Phone {
		t.Errorf("a portrait window chose %v, want Phone", got)
	}
	if got := ui.NewScreen(1280, 720).Layout; got != ui.AZERTY {
		t.Errorf("a landscape window chose %v, want AZERTY", got)
	}
}

func TestTheLetterKeysClearFortyEightPixelsOnEveryPhoneStillSold(t *testing.T) {
	// The 48 of ENCRE_02 §11 has to hold on the narrowest phone anyone will
	// play on, not only on the one the layout was drawn for.
	phones := []struct {
		name string
		w, h int
	}{
		{"iPhone SE", 320, 568},
		{"a common Android", 360, 640},
		{"iPhone 14", 390, 844},
	}
	for _, p := range phones {
		t.Run(p.name, func(t *testing.T) {
			s := ui.NewScreen(p.w, p.h)
			for _, k := range ui.NewKeyboard(s.Layout, s.BoardW, s.BoardH).Keys() {
				if k.Kind != ui.KeyRune || k.Rune < 'a' || k.Rune > 'z' {
					continue
				}
				if k.W < 48 || k.H < 48 {
					t.Errorf("letter %q is %dx%d logical px, want at least 48 on a side", k.Rune, k.W, k.H)
				}
			}
		})
	}
}

func TestTheCardIsAWholeMultipleOfItsDesignSize(t *testing.T) {
	// ENCRE_02 §15 forbids fractional scales, and ENCRE_06 §5 flags that its own
	// 180x240 is 1.875 times the 96x128 the card is drawn at. A card at a
	// fractional scale resamples every pixel of its art off the grid.
	const cw, ch = 96, 128

	for _, v := range viewports {
		t.Run(v.name, func(t *testing.T) {
			s := ui.NewScreen(v.w, v.h)
			if s.CardW%cw != 0 || s.CardH%ch != 0 {
				t.Errorf("card %dx%d is not a whole multiple of %dx%d", s.CardW, s.CardH, cw, ch)
			}
			if s.CardW/cw != s.CardH/ch {
				t.Errorf("card %dx%d is stretched: %dx horizontally, %dx vertically",
					s.CardW, s.CardH, s.CardW/cw, s.CardH/ch)
			}
			if s.CardW <= 0 {
				t.Error("the card has no size")
			}
		})
	}
}

func TestThePhoneBandsMatchTheENCRE07CorrectionOfTheCard(t *testing.T) {
	// ENCRE_07 §4.1: the card grows from 180×240 (×1.875, forbidden by ENCRE_02
	// §15) to 192×256 (×2), and the keyboard band gives back the 16 px the card
	// gained, from 334 to 318 — the two together still summing to the 844 of
	// ENCRE_06 §4's table.
	s := ui.NewScreen(ui.PortraitWidth, ui.PortraitHeight)

	if s.CardW != 192 || s.CardH != 256 {
		t.Errorf("card is %dx%d, want 192x256", s.CardW, s.CardH)
	}
	if s.BoardH != 318 {
		t.Errorf("keyboard band is %d px tall, want 318", s.BoardH)
	}
}

func TestNoBandIsLeftMostlyEmpty(t *testing.T) {
	// Empty space that carries nothing reads as a mistake. Each band is checked
	// against what it actually holds: the card, one line of 48-pixel text, and
	// the keyboard's own rows.
	const wordPx = 48

	for _, v := range viewports {
		t.Run(v.name, func(t *testing.T) {
			s := ui.NewScreen(v.w, v.h)

			// The card fills most of the band that exists for it.
			band := s.CardBandH
			if fill := float64(s.CardH) / float64(band); fill < 0.80 {
				t.Errorf("the card fills %.0f%% of its %d-pixel band, want at least 80%%", fill*100, band)
			}
			// The word's band is not more than three times the text in it.
			if s.WordBandH > 3*wordPx {
				t.Errorf("the word sits in a %d-pixel band for %d pixels of text", s.WordBandH, wordPx)
			}
			// The keyboard's rows are not stretched into columns: a row is at
			// most 1.4 times the width of an ordinary letter key.
			kb := ui.NewKeyboard(s.Layout, s.BoardW, s.BoardH)
			letter, row := 0, 0
			for _, k := range kb.Keys() {
				if k.Kind == ui.KeyRune && k.Rune >= 'a' && k.Rune <= 'z' {
					if letter == 0 || k.W < letter {
						letter = k.W
					}
					row = k.H
				}
			}
			if float64(row) > 1.4*float64(letter) {
				t.Errorf("a key is %dx%d — %.1f times taller than wide, want at most 1.4",
					letter, row, float64(row)/float64(letter))
			}
		})
	}
}
