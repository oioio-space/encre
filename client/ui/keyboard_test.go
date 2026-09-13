package ui_test

import (
	"strings"
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

// The phone screen of ENCRE_04 §2, with the keyboard band ENCRE_06 §4 gives it.
const (
	phoneW = 390
	boardH = 334
)

func TestEveryLayoutCarriesTheWholeAlphabetAndTheAccentRow(t *testing.T) {
	// ENCRE_02 §11 fixes the accent row; the alphabet is not negotiable.
	const accentRow = "éèêàçùîô-'"

	for _, l := range []ui.Layout{ui.Phone, ui.AZERTY} {
		kb := ui.NewKeyboard(l, phoneW, boardH)
		for _, want := range "abcdefghijklmnopqrstuvwxyz" + accentRow {
			if !kb.Accepts(want) {
				t.Errorf("layout %v cannot produce %q", l, want)
			}
		}
	}
}

func TestThePhoneKeyboardIsAlphabeticalAndSevenWide(t *testing.T) {
	// ENCRE_06 §2 overrides §11 here: ten columns on a 390-wide screen give
	// 39-pixel keys, under the 48 the charter demands, so the phone trades
	// AZERTY for seven alphabetical columns.
	kb := ui.NewKeyboard(ui.Phone, phoneW, boardH)

	var letters []rune
	firstRowY := -1
	for _, k := range kb.Keys() {
		if k.Kind != ui.KeyRune || !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz", k.Rune) {
			continue
		}
		if firstRowY == -1 || k.Y > firstRowY {
			firstRowY = k.Y
		}
		letters = append(letters, k.Rune)
	}
	if got, want := string(letters), "abcdefghijklmnopqrstuvwxyz"; got != want {
		t.Errorf("phone letters in reading order = %q, want %q", got, want)
	}
}

func TestRowsMayHoldDifferentNumbersOfKeys(t *testing.T) {
	// The accent row carries ten; the letter rows carry seven. Real keyboards
	// do the same, and forcing one grid on both is what made the accents
	// unreachable.
	kb := ui.NewKeyboard(ui.Phone, phoneW, boardH)

	byRow := map[int]int{}
	for _, k := range kb.Keys() {
		byRow[k.Y]++
	}
	if len(byRow) != 5 {
		t.Fatalf("phone keyboard has %d rows, want 5 (ENCRE_06 §4)", len(byRow))
	}
	counts := map[int]bool{}
	for _, n := range byRow {
		counts[n] = true
	}
	if !counts[10] || !counts[7] {
		t.Errorf("row sizes = %v, want a row of 10 (accents) and rows of 7 (letters)", byRow)
	}
}

func TestKeysStayInsideTheKeyboardAndNeverOverlap(t *testing.T) {
	for _, l := range []ui.Layout{ui.Phone, ui.AZERTY} {
		keys := ui.NewKeyboard(l, phoneW, boardH).Keys()
		for i, a := range keys {
			if a.X < 0 || a.Y < 0 || a.X+a.W > phoneW || a.Y+a.H > boardH {
				t.Errorf("layout %v: key %q at (%d,%d)+%dx%d escapes %dx%d",
					l, a.Label(), a.X, a.Y, a.W, a.H, phoneW, boardH)
			}
			for _, b := range keys[i+1:] {
				if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
					t.Errorf("layout %v: keys %q and %q overlap", l, a.Label(), b.Label())
				}
			}
		}
	}
}

func TestKeyAtFindsTheKeyUnderItsOwnCentre(t *testing.T) {
	kb := ui.NewKeyboard(ui.Phone, phoneW, boardH)

	for _, want := range kb.Keys() {
		got, ok := kb.KeyAt(want.X+want.W/2, want.Y+want.H/2)
		if !ok || got.Label() != want.Label() {
			t.Errorf("KeyAt(centre of %q) = %q/%v, want %q", want.Label(), got.Label(), ok, want.Label())
		}
	}
}

func TestKeyAtReportsNothingOutsideTheKeyboard(t *testing.T) {
	kb := ui.NewKeyboard(ui.Phone, phoneW, boardH)

	if _, ok := kb.KeyAt(-1, 10); ok {
		t.Error("KeyAt(-1, 10) found a key left of the keyboard")
	}
	if _, ok := kb.KeyAt(10, boardH+1); ok {
		t.Error("KeyAt below the keyboard found a key")
	}
}

func TestEraseAndValidateArePresentInEveryLayout(t *testing.T) {
	for _, l := range []ui.Layout{ui.Phone, ui.AZERTY} {
		var erase, validate bool
		for _, k := range ui.NewKeyboard(l, phoneW, boardH).Keys() {
			switch k.Kind {
			case ui.KeyErase:
				erase = true
			case ui.KeyValidate:
				validate = true
			}
		}
		if !erase || !validate {
			t.Errorf("layout %v: erase=%v validate=%v, want both (ENCRE_02 §11)", l, erase, validate)
		}
	}
}

func TestThePhoneLetterKeysClearFortyEightLogicalPixels(t *testing.T) {
	// The reason ENCRE_06 §2 dropped to seven columns. The comparison is in
	// LOGICAL pixels of the 390-wide screen — those are the density-independent
	// units the 48 of §11 is written in. Multiplying by the device scale, as a
	// first version of this check did, makes every modern phone pass and
	// measures nothing.
	const minLogical = 48

	for _, k := range ui.NewKeyboard(ui.Phone, phoneW, boardH).Keys() {
		if k.Kind != ui.KeyRune || !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz", k.Rune) {
			continue
		}
		if k.W < minLogical || k.H < minLogical {
			t.Errorf("letter key %q is %dx%d logical px, want at least %d on a side",
				k.Rune, k.W, k.H, minLogical)
		}
	}
}
