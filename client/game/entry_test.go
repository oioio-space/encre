package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/ui"
)

func newEntry() *game.Entry {
	return game.NewEntry(ui.NewKeyboard(ui.AZERTY, 390, 380))
}

func TestTypeAppendsACharacterTheKeyboardCanProduce(t *testing.T) {
	e := newEntry()

	for _, r := range "ébène" {
		if !e.Type(r) {
			t.Fatalf("Type(%q) = false, want true", r)
		}
	}
	if got, want := e.Text(), "ébène"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestTypeRefusesWhatTheDrawnKeyboardCannotProduce(t *testing.T) {
	// AppendInputChars hands over everything the OS keyboard emits. Only the
	// script lowercase of ENCRE_02 §11 may reach the word.
	for _, r := range []rune{'A', 'É', '€', '4', ' '} {
		e := newEntry()
		if e.Type(r) {
			t.Errorf("Type(%q) = true, want false", r)
		}
		if e.Text() != "" {
			t.Errorf("after refusing %q, Text() = %q, want empty", r, e.Text())
		}
	}
}

func TestEraseRemovesOneWholeAccentedLetter(t *testing.T) {
	// 'é' is two bytes: erasing by byte would leave half a rune behind and
	// render as a replacement character in the middle of the child's word.
	e := newEntry()
	for _, r := range "clé" {
		e.Type(r)
	}

	e.Erase()

	if got, want := e.Text(), "cl"; got != want {
		t.Errorf("after Erase(), Text() = %q, want %q", got, want)
	}
}

func TestEraseOnAnEmptyWordDoesNothing(t *testing.T) {
	e := newEntry()

	e.Erase()

	if got := e.Text(); got != "" {
		t.Errorf("Text() = %q, want empty", got)
	}
}

func TestLenCountsLettersNotBytes(t *testing.T) {
	e := newEntry()
	for _, r := range "élève" {
		e.Type(r)
	}

	if got, want := e.Len(), 5; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
}

func TestClearEmptiesTheWord(t *testing.T) {
	e := newEntry()
	for _, r := range "mot" {
		e.Type(r)
	}

	e.Clear()

	if got := e.Text(); got != "" {
		t.Errorf("after Clear(), Text() = %q, want empty", got)
	}
}
