package ui_test

import (
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

// accentRow is the row ENCRE_02 §11 requires to stay visible above the letters.
const accentRow = "éèêàçùîô-'"

func TestAZERTYCarriesEveryLetterAndAccent(t *testing.T) {
	kb := ui.NewKeyboard(ui.AZERTY, 390, 380)

	for _, want := range "azertyuiopqsdfghjklmwxcvbn" + accentRow {
		if !kb.Accepts(want) {
			t.Errorf("NewKeyboard(AZERTY).Accepts(%q) = false, want true", want)
		}
	}
}

// phonePortrait is the logical resolution ENCRE_04 §2 fixes for portrait, with
// the keyboard taking the ~45% of the height ENCRE_02 §11 budgets for it.
const (
	phoneW = 390
	phoneH = 844
	boardH = 380
)

func TestKeysStayInsideTheKeyboard(t *testing.T) {
	kb := ui.NewKeyboard(ui.AZERTY, phoneW, boardH)

	for _, k := range kb.Keys() {
		if k.X < 0 || k.Y < 0 || k.X+k.W > phoneW || k.Y+k.H > boardH {
			t.Errorf("key %q at (%d,%d)+%dx%d escapes the %dx%d keyboard",
				k.Label(), k.X, k.Y, k.W, k.H, phoneW, boardH)
		}
	}
}

func TestKeysNeverOverlap(t *testing.T) {
	keys := ui.NewKeyboard(ui.AZERTY, phoneW, boardH).Keys()

	for i, a := range keys {
		for _, b := range keys[i+1:] {
			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				t.Errorf("keys %q and %q overlap: (%d,%d)+%dx%d vs (%d,%d)+%dx%d",
					a.Label(), b.Label(), a.X, a.Y, a.W, a.H, b.X, b.Y, b.W, b.H)
			}
		}
	}
}

func TestKeyAtFindsTheKeyUnderItsOwnCentre(t *testing.T) {
	kb := ui.NewKeyboard(ui.AZERTY, phoneW, boardH)

	for _, want := range kb.Keys() {
		got, ok := kb.KeyAt(want.X+want.W/2, want.Y+want.H/2)
		if !ok {
			t.Errorf("KeyAt(centre of %q) found nothing", want.Label())
			continue
		}
		if got.Label() != want.Label() {
			t.Errorf("KeyAt(centre of %q) = %q, want %q", want.Label(), got.Label(), want.Label())
		}
	}
}

func TestKeyAtReportsNothingOutsideTheKeyboard(t *testing.T) {
	kb := ui.NewKeyboard(ui.AZERTY, phoneW, boardH)

	if _, ok := kb.KeyAt(-1, 10); ok {
		t.Error("KeyAt(-1, 10) found a key left of the keyboard")
	}
	if _, ok := kb.KeyAt(10, boardH+1); ok {
		t.Error("KeyAt(10, boardH+1) found a key below the keyboard")
	}
}

func TestEraseAndValidateArePresentAndWide(t *testing.T) {
	keys := ui.NewKeyboard(ui.AZERTY, phoneW, boardH).Keys()

	var erase, validate, letter *ui.Key
	for i := range keys {
		switch keys[i].Kind {
		case ui.KeyErase:
			erase = &keys[i]
		case ui.KeyValidate:
			validate = &keys[i]
		case ui.KeyRune:
			if letter == nil {
				letter = &keys[i]
			}
		}
	}
	if erase == nil || validate == nil {
		t.Fatalf("erase=%v validate=%v, want both present (ENCRE_02 §11)", erase, validate)
	}
	// "grandes, à droite" — both must be wider than an ordinary letter key.
	if erase.W <= letter.W {
		t.Errorf("erase key width = %d, want wider than a letter key (%d)", erase.W, letter.W)
	}
	if validate.W <= letter.W {
		t.Errorf("validate key width = %d, want wider than a letter key (%d)", validate.W, letter.W)
	}
}

func TestPhysicalPxScalesByWindowRatioThenDeviceScale(t *testing.T) {
	tests := []struct {
		name                        string
		logical, screenLen, outside int
		deviceScale                 float64
		want                        float64
	}{
		{
			name: "screen drawn 1:1 into the window, scale 3",
			// The phone of ENCRE_04 §2: Layout returns exactly the window size
			// in device-independent pixels, so only the device scale applies.
			logical: 39, screenLen: 390, outside: 390, deviceScale: 3,
			want: 117,
		},
		{
			name: "logical screen squeezed into a narrower window",
			// A 360 dip phone still gets the fixed 390 logical screen: every
			// logical pixel is worth less than one dip before the scale.
			logical: 39, screenLen: 390, outside: 360, deviceScale: 2,
			want: 72,
		},
		{
			name:    "desktop window, no device scaling",
			logical: 39, screenLen: 390, outside: 390, deviceScale: 1,
			want: 39,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ui.PhysicalPx(tt.logical, tt.screenLen, tt.outside, tt.deviceScale)
			if got != tt.want {
				t.Errorf("PhysicalPx(%d, %d, %d, %v) = %v, want %v",
					tt.logical, tt.screenLen, tt.outside, tt.deviceScale, got, tt.want)
			}
		})
	}
}

func TestPhysicalPxIsZeroWhenTheScreenHasNoWidth(t *testing.T) {
	// Ebitengine calls Layout before the window exists; a division by zero here
	// would poison the measurement the prototype exists to report.
	if got := ui.PhysicalPx(39, 0, 390, 3); got != 0 {
		t.Errorf("PhysicalPx with screenLen 0 = %v, want 0", got)
	}
}

func TestTouchTargetsClearFortyEightPhysicalPixelsOnTheReferencePhone(t *testing.T) {
	// The acceptance criterion of ticket T00, as arithmetic. The real check is
	// still on the real device — this only stops a layout change from breaking
	// it silently between two of those sessions.
	const minPhysical = 48

	kb := ui.NewKeyboard(ui.AZERTY, phoneW, boardH)
	side := kb.MinTouchSide()
	got := ui.PhysicalPx(side, phoneW, phoneW, 3)

	if got < minPhysical {
		t.Errorf("smallest touch side = %d logical px = %v physical px, want >= %d",
			side, got, minPhysical)
	}
}

func TestMinTouchSideIsTheSmallestSideOfAnyKey(t *testing.T) {
	kb := ui.NewKeyboard(ui.AZERTY, phoneW, boardH)

	want := 1 << 30
	for _, k := range kb.Keys() {
		want = min(want, min(k.W, k.H))
	}
	if got := kb.MinTouchSide(); got != want {
		t.Errorf("MinTouchSide() = %d, want %d", got, want)
	}
}
