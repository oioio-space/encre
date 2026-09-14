package lexique

import (
	"slices"
	"strings"
	"unicode"

	"github.com/oioio-space/encre/engine"
)

// Span marks a stretch of a sentence in rune offsets: the word the child is
// asked to write, when the rest of the sentence is given to them.
type Span struct {
	Start, End int
}

// plurals are the words that announce a plural before the word they govern —
// determiners and the two subject pronouns.
var plurals = map[string]bool{
	"les": true, "des": true, "mes": true, "tes": true, "ses": true,
	"ces": true, "nos": true, "vos": true, "leurs": true, "aux": true,
	"deux": true, "trois": true, "quatre": true, "plusieurs": true,
	"quelques": true, "beaucoup": true, "ils": true, "elles": true,
}

// feminines are the words that announce a feminine before the word they agree
// with.
var feminines = map[string]bool{
	"une": true, "la": true, "ma": true, "ta": true, "sa": true,
	"cette": true, "elle": true, "elles": true,
}

// token is one word of a sentence with where it is written.
type token struct {
	word       string
	start, end int // rune offsets
}

// AnalyzeSentence analyses the targeted words of a sentence, adding the
// Accordées that only the context can reveal (ENCRE_04 §5).
//
// With no targets it analyses every word, which is what the parent's paste
// screen shows before anyone has chosen the gaps.
func (l *Lexicon) AnalyzeSentence(s string, targets []Span) []Analysis {
	tokens := tokenize(s)

	out := make([]Analysis, 0, len(tokens))
	for i, tk := range tokens {
		if !targeted(tk, targets) {
			continue
		}
		a := l.Analyze(tk.word)
		if hit, ok := l.agreement(tokens, i); ok {
			supersede(&a, hit)
		}
		out = append(out, a)
	}
	return out
}

// supersede records an agreement, taking over the letters it is written on.
//
// One letter teaches one thing. Out of context the s of chats is a silent
// letter; inside « les chats » it is the plural, and calling it both would make
// the child pay twice for the same s and hear two justifications for it.
func supersede(a *Analysis, hit Hit) {
	a.Hits = slices.DeleteFunc(a.Hits, func(h Hit) bool {
		return h.Color == engine.Muettes && h.At < hit.At+hit.Len && hit.At < h.At+h.Len
	})
	a.Hits = append(a.Hits, hit)
	a.Traps = countTraps(a.Hits)
}

// targeted reports whether this word is one the child has to write. No targets
// at all means the whole sentence is the target.
func targeted(tk token, targets []Span) bool {
	if len(targets) == 0 {
		return true
	}
	for _, t := range targets {
		if tk.start < t.End && t.Start < tk.end {
			return true
		}
	}
	return false
}

// agreement decides whether the word at index i carries an agreement, and which.
//
// It asks two things at once: does the ending look like an agreement, and does
// anything before it in the sentence call for one. A bras is not a plural and
// an arbre is not a feminine, however they end. The -e of the feminine is asked
// of adjectives only, because only they agree: la porte is not a porte that
// agreed with anything (ENCRE_03 §2).
func (l *Lexicon) agreement(tokens []token, i int) (Hit, bool) {
	w := fold(tokens[i].word)
	runes := []rune(w)
	entry := l.words[w]
	announced := announcement(tokens, i)

	switch {
	case strings.HasSuffix(w, "ent") && len(runes) > 3 &&
		strings.Contains(entry.Gram, "VER") && announced.ent:
		return Hit{Rule: RuleVerbeEnt, Color: engine.Accordees, At: len(runes) - 3, Len: 3}, true

	case strings.HasSuffix(w, "x") && announced.plural:
		return Hit{Rule: RulePlurielX, Color: engine.Accordees, At: len(runes) - 1, Len: 1}, true

	case strings.HasSuffix(w, "s") && announced.plural && !singularOnly(entry):
		return Hit{Rule: RulePlurielS, Color: engine.Accordees, At: len(runes) - 1, Len: 1}, true

	case strings.HasSuffix(w, "e") && announced.feminine && strings.Contains(entry.Gram, "ADJ"):
		return Hit{Rule: RuleFemininE, Color: engine.Accordees, At: len(runes) - 1, Len: 1}, true
	}
	return Hit{}, false
}

// announced says what the words before the target call for. ent is only ever
// set together with plural: the third person plural of a verb is what a plural
// subject asks for, and there is no other way to reach it.
type announced struct{ plural, feminine, ent bool }

// announcement scans back to the start of the sentence for the determiner or
// pronoun that governs the target. It stops at the nearest one: in « les chats
// et la fille », the fille is governed by la, not by les.
func announcement(tokens []token, i int) announced {
	var a announced
	for j := i - 1; j >= 0 && i-j <= 3; j-- {
		w := fold(tokens[j].word)
		switch {
		case plurals[w]:
			// A plural subject licenses both the -s of the noun and the -ent of
			// the verb; which of the two applies is settled by what the word is.
			// ENCRE_03 §2 teaches the ending as « Ils : ent », but a dictation
			// says « les chats dorment » far more often than « ils dorment ».
			a.plural, a.ent = true, true
			return a
		case feminines[w]:
			a.feminine = true
			return a
		}
	}
	return a
}

// singularOnly reports whether the lexicon only ever records this form in the
// singular, which is what tells the bras of « le bras droit » from a plural.
func singularOnly(e Entry) bool {
	return e.Nombre == "s"
}

// tokenize cuts a sentence into words, keeping the apostrophes and hyphens that
// belong inside them, and records where each one is written in runes.
func tokenize(s string) []token {
	var (
		tokens []token
		cur    []rune
		start  int
	)
	flush := func(end int) {
		if len(cur) > 0 {
			tokens = append(tokens, token{word: string(cur), start: start, end: end})
			cur = cur[:0]
		}
	}
	i := 0
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || r == '-' || (r == '\'' && len(cur) > 0):
			if len(cur) == 0 {
				start = i
			}
			cur = append(cur, r)
		default:
			flush(i)
		}
		i++
	}
	flush(i)
	return tokens
}
