package ui_test

import (
	"strings"
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

func TestVariantsCoverTheAccentsNoRowHasRoomFor(t *testing.T) {
	// The gap that made this necessary, found in the brief's own word lists:
	// û appears in flûte, brûle, sûr, coût and œ in cœur, œil — all CE1
	// vocabulary, and none of them typeable from the ten keys of ENCRE_02 §11.
	// Holding the base letter is what every mobile keyboard does about it, and
	// what ENCRE_06 §5 proposes here.
	want := map[rune]string{
		'a': "àâ",
		'c': "ç",
		'e': "éèêë",
		'i': "îï",
		'o': "ôöœ",
		'u': "ùû", // no ü: outside the chain of ENCRE_02 §5
	}
	for base, accents := range want {
		got := string(ui.Variants(base))
		if got != accents {
			t.Errorf("Variants(%q) = %q, want %q", base, got, accents)
		}
	}
}

func TestVariantsAreEmptyForALetterThatTakesNoAccent(t *testing.T) {
	for _, r := range []rune{'b', 'z', '-', '\''} {
		if got := ui.Variants(r); len(got) > 0 {
			t.Errorf("Variants(%q) = %q, want none", r, string(got))
		}
	}
}

func TestEveryVariantIsOneOfTheCharacterSetTheFontMustCarry(t *testing.T) {
	// ENCRE_02 §5 fixes the chain any font of the game must draw. A variant
	// outside it would reach the child as an empty box.
	const charterChain = "éèêëàâùûîïôöçœ"

	for _, base := range "abcdefghijklmnopqrstuvwxyz" {
		for _, v := range ui.Variants(base) {
			if !strings.ContainsRune(charterChain, v) {
				t.Errorf("Variants(%q) offers %q, which ENCRE_02 §5 does not list", base, v)
			}
		}
	}
}

func TestHoldingAKeyReachesTheWordsTheRowCannotType(t *testing.T) {
	kb := ui.NewKeyboard(ui.Phone, phoneW, boardH)

	for _, r := range "cœur" + "flûte" + "Noël"[1:] {
		if !kb.Accepts(r) {
			t.Errorf("the keyboard cannot produce %q, needed by cœur / flûte / Noël", r)
		}
	}
}
