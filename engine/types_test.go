package engine_test

import (
	"testing"

	"github.com/oioio-space/encre/engine"
)

func TestTheSixColoursAreNamedAsTheContentCallsThem(t *testing.T) {
	// The names are read aloud to the child and printed on the score
	// (ENCRE_03 §4), so they are part of the contract, not debug output.
	want := map[engine.Color]string{
		engine.Muettes:    "Muettes",
		engine.Jumelles:   "Jumelles",
		engine.Accentuees: "Accentuées",
		engine.Masquees:   "Masquées",
		engine.Sosies:     "Sosies",
		engine.Accordees:  "Accordées",
	}
	for c, name := range want {
		if got := c.String(); got != name {
			t.Errorf("Color(%d).String() = %q, want %q", c, got, name)
		}
	}
}

func TestColorsListsEverySixOnceInOrder(t *testing.T) {
	got := engine.Colors()

	if len(got) != 6 {
		t.Fatalf("Colors() has %d entries, want 6", len(got))
	}
	seen := map[engine.Color]bool{}
	for _, c := range got {
		if seen[c] {
			t.Errorf("Colors() repeats %v", c)
		}
		seen[c] = true
	}
}

func TestAnUnknownColourSaysSoRatherThanPrintingANumber(t *testing.T) {
	if got := engine.Color(99).String(); got == "" || got == "99" {
		t.Errorf("Color(99).String() = %q, want something a reader can act on", got)
	}
}

func TestTrapsCountsEveryTrapOfTheWord(t *testing.T) {
	// The chips a word pays are proportional to its traps (ENCRE_01), so this
	// total is the base of every score.
	w := engine.Word{Traps: map[engine.Color]int{
		engine.Muettes:    2,
		engine.Accentuees: 1,
	}}

	if got, want := w.TrapCount(), 3; got != want {
		t.Errorf("TrapCount() = %d, want %d", got, want)
	}
}

func TestAWordWithoutTrapsCountsNone(t *testing.T) {
	if got := (engine.Word{}).TrapCount(); got != 0 {
		t.Errorf("TrapCount() on a bare word = %d, want 0", got)
	}
}

func TestTheZeroWordStateIsAWordNeverMet(t *testing.T) {
	// Decks are built from a map that has no entry for a word the child has not
	// seen, so the zero value has to be the honest starting point.
	var st engine.WordState

	switch {
	case st.Mastery != 0:
		t.Errorf("zero WordState.Mastery = %v, want 0", st.Mastery)
	case st.Seen, st.Gold, st.Tarnished, st.Cursed:
		t.Errorf("zero WordState = %+v, want every flag false", st)
	case st.Shine != engine.ShineNone:
		t.Errorf("zero WordState.Shine = %v, want ShineNone", st.Shine)
	}
}
