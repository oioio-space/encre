package gen

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// maxSentenceWords is ENCRE_04 §9's hard cap: "rejet de toute phrase de
// plus de 8 mots".
const maxSentenceWords = 8

// gapMarker is the placeholder a generated sentence's gap is written with
// (the prompt's own example: "Le ___ dort sur le lit."). It is exempt from
// the whitelist check — it is not a word — and still counts toward
// maxSentenceWords, the same as any other token.
const gapMarker = "___"

// ErrSentenceTooLong is returned by [Filter] for a sentence over
// [maxSentenceWords] words.
var ErrSentenceTooLong = errors.New("gen: sentence has more than 8 words")

// ErrWordNotWhitelisted is returned by [Filter] for a sentence containing a
// word outside the CE1 [Whitelist].
var ErrWordNotWhitelisted = errors.New("gen: sentence contains a word outside the CE1 whitelist")

// Sentence is one fill-in-the-blank sentence for one word, in the shape
// ENCRE_03 §9's prompt asks the API to answer with.
type Sentence struct {
	// Texte is the sentence, gap included ("Le ___ dort sur le lit.").
	Texte string `json:"texte"`
	// Cible is the word being taught, exactly as the word list carries it.
	Cible string `json:"cible"`
	// Forme is Cible as it is actually written in Texte's gap — accorded,
	// when the list's Couleur asks for that (ENCRE_03 §9).
	Forme string `json:"forme"`
}

// Filter checks one generated sentence against ENCRE_04 §9's two hard
// rules: at most eight words, and no word outside wl except the sentence's
// own target (Cible or Forme, in either case) and the gap marker itself.
//
// It never rejects on anything softer than these two rules — the "5 à 8
// mots", "pas de nom propre" and the rest of the prompt's constraints are
// asked of the model, not re-verified here, because ENCRE_05's acceptance
// for this ticket names exactly these two as what must be enforced in code.
func Filter(s Sentence, wl Whitelist) error {
	words := splitWords(s.Texte)
	if len(words) > maxSentenceWords {
		return fmt.Errorf("%w: %q (%d mots)", ErrSentenceTooLong, s.Texte, len(words))
	}

	cible, forme := fold(s.Cible), fold(s.Forme)
	for _, w := range words {
		f := fold(w)
		if f == gapMarker || f == cible || f == forme {
			continue
		}
		if !wl.Contains(f) {
			return fmt.Errorf("%w: %q", ErrWordNotWhitelisted, w)
		}
	}
	return nil
}

// splitWords cuts s into whitespace-separated tokens, each trimmed of
// surrounding punctuation but keeping internal apostrophes, hyphens and the
// underscores of [gapMarker].
func splitWords(s string) []string {
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		trimmed := strings.TrimFunc(f, func(r rune) bool {
			return !unicode.IsLetter(r) && r != '\'' && r != '-' && r != '_'
		})
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// fold puts a word in the shape [Whitelist] is keyed and compared by: lower
// case, no surrounding punctuation. It mirrors
// [github.com/oioio-space/encre/lexique]'s own unexported fold rather than
// importing it — the two packages have no reason to share this one small
// helper across a module boundary.
func fold(word string) string {
	return strings.ToLower(strings.TrimSpace(word))
}
