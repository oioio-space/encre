package lexique_test

import (
	"bytes"
	"compress/gzip"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

func TestPositionsPointAtTheTrapLetters(t *testing.T) {
	t.Parallel()

	// chat: ch at 0 and 1, the silent t at 3 — the a is the only letter that is
	// simply said, and the only one not lit.
	got := lexique.Embedded().Analyze("chat").Positions()
	if want := []int{0, 1, 3}; !slices.Equal(got, want) {
		t.Fatalf("Positions(chat) = %v, want %v", got, want)
	}
}

func TestPositionsCountRunesNotBytes(t *testing.T) {
	t.Parallel()

	// The é of école is two bytes and one letter; the silent e is the fifth
	// letter, not the sixth byte.
	a := lexique.Embedded().Analyze("école")
	for _, p := range a.Positions() {
		if p >= len([]rune("école")) {
			t.Errorf("Positions(école) = %v, out of the word's 5 letters", a.Positions())
		}
	}
}

func TestFamilyExplainsTheSilentLetter(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	for word, want := range map[string]string{
		"chat":  "chatte",
		"grand": "grande",
		"gros":  "grosse",
		"blanc": "blanche",
	} {
		if got := lex.Analyze(word).Family; got != want {
			t.Errorf("Analyze(%q).Family = %q, want %q", word, got, want)
		}
	}
}

func TestUnknownWordIsFlaggedForTheParent(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	a := lex.Analyze("zorglubesque")
	if a.Known {
		t.Fatal("zorglubesque is in the lexicon after all")
	}
	if !a.Unsure() {
		t.Errorf("Confidence %.2f, want below %.2f so the parent is asked",
			a.Confidence, lexique.UnsureConfidence)
	}
	// With no pronunciation the patterns still speak, but nothing is claimed
	// about the letters that are not heard.
	if a.Traps[engine.Muettes] != 0 {
		t.Errorf("got %d Muettes on a word with no known sound, want none", a.Traps[engine.Muettes])
	}
}

func TestKnownWordIsConfident(t *testing.T) {
	t.Parallel()

	a := lexique.Embedded().Analyze("chat")
	if !a.Known || a.Unsure() {
		t.Errorf("Analyze(chat): Known=%v Confidence=%.2f, want a confident analysis", a.Known, a.Confidence)
	}
}

func TestSosiesAreNamed(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	for word, want := range map[string]engine.Rule{
		"a": lexique.RuleAA, "à": lexique.RuleAA,
		"et": lexique.RuleEtEst, "est": lexique.RuleEtEst,
		"ont": lexique.RuleOnOnt, "sont": lexique.RuleSonSont,
	} {
		a := lex.Analyze(word)
		if a.Traps[engine.Sosies] == 0 || !slices.Contains(a.Rules(), want) {
			t.Errorf("Analyze(%q) = %v, want the Sosie %q", word, a.Rules(), want)
		}
	}
}

func TestFoldAcceptsWhatAParentPastes(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	for _, in := range []string{"Chat", "  chat ", "chat,", "«chat»"} {
		if a := lex.Analyze(in); !a.Known {
			t.Errorf("Analyze(%q) did not recognise chat", in)
		}
	}
}

func TestLookupAndLen(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	if lex.Len() < 10000 {
		t.Errorf("Len() = %d, want the whole embedded lexicon", lex.Len())
	}
	e, ok := lex.Lookup("chat")
	if !ok || e.Phon != "Sa" {
		t.Errorf("Lookup(chat) = %+v, %v; want the pronunciation Sa", e, ok)
	}
	if _, ok := lex.Lookup("zorglubesque"); ok {
		t.Error("Lookup(zorglubesque) found something")
	}
}

func TestLoadRejectsAMalformedFile(t *testing.T) {
	t.Parallel()

	if _, err := lexique.Load(strings.NewReader("not gzip at all")); err == nil {
		t.Error("Load accepted something that is not gzip")
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("ortho\tphon\n" + "chat\tSa\n")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := lexique.Load(&buf); err == nil {
		t.Error("Load accepted a line with the wrong number of columns")
	}
}

func TestAttributionTravelsWithTheData(t *testing.T) {
	t.Parallel()

	if !strings.Contains(lexique.Attribution, "CC BY-SA 4.0") ||
		!strings.Contains(lexique.Attribution, "lexique.org") {
		t.Errorf("Attribution = %q, want the source and the licence", lexique.Attribution)
	}
}
