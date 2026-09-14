package gen_test

import (
	"errors"
	"testing"

	"github.com/oioio-space/encre/server/gen"
)

func testWhitelist() gen.Whitelist {
	return gen.Whitelist{"le": true, "chat": true, "dort": true, "sur": true, "lit": true}
}

// TestFilterAcceptsAValidSentence is the happy path: eight words or fewer,
// every non-target word in the whitelist.
func TestFilterAcceptsAValidSentence(t *testing.T) {
	s := gen.Sentence{Texte: "Le chat dort sur le lit.", Cible: "chat", Forme: "chat"}
	if err := gen.Filter(s, testWhitelist()); err != nil {
		t.Errorf("Filter() error = %v, want nil", err)
	}
}

// TestFilterRejectsSentenceOverEightWords is the ticket's first named
// acceptance case: "une phrase de neuf mots est rejetée".
func TestFilterRejectsSentenceOverEightWords(t *testing.T) {
	s := gen.Sentence{
		Texte: "Le chat dort sur le lit avec le chien vraiment",
		Cible: "chat", Forme: "chat",
	}
	err := gen.Filter(s, gen.Whitelist{
		"le": true, "chat": true, "dort": true, "sur": true, "lit": true,
		"avec": true, "chien": true, "vraiment": true,
	})
	if !errors.Is(err, gen.ErrSentenceTooLong) {
		t.Errorf("Filter() on a nine-word sentence: error = %v, want ErrSentenceTooLong", err)
	}
}

// TestFilterRejectsWordOutsideWhitelist is the ticket's second named case:
// "une phrase avec un mot hors liste blanche est rejetée".
func TestFilterRejectsWordOutsideWhitelist(t *testing.T) {
	s := gen.Sentence{Texte: "Le chat somnole sur le lit.", Cible: "chat", Forme: "chat"}
	err := gen.Filter(s, testWhitelist())
	if !errors.Is(err, gen.ErrWordNotWhitelisted) {
		t.Errorf("Filter() with a non-whitelisted word: error = %v, want ErrWordNotWhitelisted", err)
	}
}

// TestFilterExemptsTheTargetWord checks that the word being taught never
// has to be in the whitelist itself — that would defeat the point, since a
// trap word is exactly the kind of word a CE1 whitelist would not carry.
func TestFilterExemptsTheTargetWord(t *testing.T) {
	s := gen.Sentence{Texte: "Le chat dort sur le paillasson.", Cible: "paillasson", Forme: "paillasson"}
	wl := gen.Whitelist{"le": true, "chat": true, "dort": true, "sur": true}
	if err := gen.Filter(s, wl); err != nil {
		t.Errorf("Filter() rejecting the sentence's own target word: error = %v, want nil", err)
	}
}

// TestFilterExemptsTheGapMarker checks that "___" is never treated as a
// non-whitelisted word.
func TestFilterExemptsTheGapMarker(t *testing.T) {
	s := gen.Sentence{Texte: "Le ___ dort sur le lit.", Cible: "chat", Forme: "chat"}
	if err := gen.Filter(s, testWhitelist()); err != nil {
		t.Errorf("Filter() rejecting the gap marker: error = %v, want nil", err)
	}
}

// TestFilterAcceptsExactlyEightWords checks the boundary: eight is allowed,
// only strictly more is rejected.
func TestFilterAcceptsExactlyEightWords(t *testing.T) {
	s := gen.Sentence{Texte: "Le chat dort sur le lit le soir", Cible: "chat", Forme: "chat"}
	wl := gen.Whitelist{"le": true, "chat": true, "dort": true, "sur": true, "lit": true, "soir": true}
	if err := gen.Filter(s, wl); err != nil {
		t.Errorf("Filter() on an eight-word sentence: error = %v, want nil", err)
	}
}
