package lexique

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oioio-space/encre/engine"
)

// The confidence ENCRE_03 §10 asks for is a signal with two states, not a
// score: either the lexicon knew the word and confirmed its traps against the
// pronunciation, or it did not and the analysis rests on spelling alone.
const (
	// UnsureConfidence is the line below which a word is shown to the parent as
	// « à vérifier ».
	UnsureConfidence = 0.6
	// confirmedConfidence is a word the lexicon knows, traps and sounds agreed.
	confirmedConfidence = 0.95
	// guessedConfidence is a word analysed on its patterns alone.
	guessedConfidence = 0.4
)

// Hit is one rule found in one word, with where it is written.
//
// At and Len are rune offsets into the word, which is what the client needs to
// light the trap letters up rather than the whole word.
type Hit struct {
	// Rule is the fine rule of ENCRE_03 §2 that was found.
	Rule engine.Rule
	// Color is the Couleur that rule belongs to, and so the colour the card
	// and the score show it in.
	Color engine.Color
	// At is the rune offset of the first trap letter.
	At int
	// Len is how many letters the trap is written on.
	Len int
}

// Analysis is everything the game needs to know about a word: the traps it
// carries per Couleur, the rules behind them, where they are written, the word
// of the same family that explains a silent letter, and how sure we are.
type Analysis struct {
	// Word is the form that was analysed, exactly as the caller wrote it.
	Word string
	// Hits is every trap found, in the order they are written.
	Hits []Hit
	// Traps counts the hits per Couleur. It is what the engine scores the word
	// on, and it is always derived from Hits rather than kept beside them.
	Traps map[engine.Color]int
	// Family is a word where a silent letter is heard — chat → chaton — so
	// Phalène can justify the trap instead of asserting it (ENCRE_03 §10).
	Family string
	// Confidence is how sure the analysis is, between 0 and 1. Below
	// UnsureConfidence the parent is asked to confirm it.
	Confidence float64
	// Known says whether the lexicon had the word. It is what separates a
	// confident analysis from a guess made on patterns alone.
	Known bool
}

// Rules lists the rules found, in the order they are written.
func (a Analysis) Rules() []engine.Rule {
	rules := make([]engine.Rule, 0, len(a.Hits))
	for _, h := range a.Hits {
		rules = append(rules, h.Rule)
	}
	return rules
}

// Positions lists the rune index of every trap letter, for the illumination.
func (a Analysis) Positions() []int {
	var pos []int
	for _, h := range a.Hits {
		for i := range h.Len {
			pos = append(pos, h.At+i)
		}
	}
	slices.Sort(pos)
	return pos
}

// Unsure reports whether the parent should be asked to confirm this analysis.
func (a Analysis) Unsure() bool { return a.Confidence < UnsureConfidence }

// Entry is one form of the lexicon: how it is said and what grammar it carries.
// Gram and Nombre hold every value the form is recorded with, comma-separated,
// because « grand » is an adjective, an adverb and a noun and only the sentence
// decides which.
type Entry struct {
	// Phon is the pronunciation, in the SAMPA alphabet Lexique 3.83 uses.
	Phon string
	// Gram holds every grammatical category the form is recorded under,
	// comma-separated: « grand » is ADJ,ADV,NOM.
	Gram string
	// Nombre holds every number the form is recorded under, s and p.
	Nombre string
	// Family is the related word where the silent final letter is heard,
	// computed when the database is built.
	Family string
}

// Lexicon analyses words against an embedded French lexicon.
//
// The zero value is useless; build one with Embedded or Load.
type Lexicon struct {
	words map[string]Entry
}

// Lookup returns what the lexicon knows about a form.
func (l *Lexicon) Lookup(word string) (Entry, bool) {
	e, ok := l.words[fold(word)]
	return e, ok
}

// Len is the number of forms the lexicon holds.
func (l *Lexicon) Len() int { return len(l.words) }

// fold puts a word in the shape the lexicon is keyed by: lower case, no
// surrounding punctuation, the typographic apostrophe folded onto the typed one.
func fold(word string) string {
	word = strings.ToLower(strings.TrimSpace(word))
	word = strings.ReplaceAll(word, "’", "'")
	return strings.TrimFunc(word, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\'' && r != '-'
	})
}

// Analyze names every trap in a single word (ENCRE_03 §10).
//
// The order of the work matters. The Masquées run first and mark the letters
// they claim, so that the silent-letter walk does not mistake the l of fille or
// the n of pont — letters inside a sound — for letters written and not heard.
func (l *Lexicon) Analyze(word string) Analysis {
	w := fold(word)
	entry, known := l.words[w]

	runes := []rune(w)
	covered := make([]bool, len(runes))

	a := Analysis{Word: word, Known: known}
	a.Hits = append(a.Hits, matchAll(masquees, w, entry.Phon, covered)...)
	a.Hits = append(a.Hits, matchAll(jumelles, w, entry.Phon, nil)...)
	a.Hits = append(a.Hits, accentHits(runes)...)
	if rule, ok := sosies[w]; ok {
		a.Hits = append(a.Hits, Hit{Rule: rule, Color: engine.Sosies, At: 0, Len: len(runes)})
	}
	a.Hits = append(a.Hits, l.silentTail(w, entry.Phon, covered)...)

	a.Traps = countTraps(a.Hits)

	a.Family = entry.Family
	a.Confidence = guessedConfidence
	if known {
		a.Confidence = confirmedConfidence
	}
	return a
}

// countTraps sorts the hits into reading order and counts them per Couleur. The
// count is always derived from the hits, never kept alongside them, so that a
// rule taken away — an agreement superseding a silent letter — cannot leave a
// tally behind.
func countTraps(hits []Hit) map[engine.Color]int {
	slices.SortStableFunc(hits, func(x, y Hit) int { return cmp.Compare(x.At, y.At) })
	traps := make(map[engine.Color]int, len(hits))
	for _, h := range hits {
		traps[h.Color]++
	}
	return traps
}

// matchAll finds every non-overlapping occurrence of each pattern, keeping only
// those the pronunciation confirms. A word with no pronunciation — one the
// lexicon does not know — is matched on its spelling alone, which is all
// ENCRE_03 §10 promises for it.
//
// When covered is non-nil each match marks the letters it claims, so a later
// pass can tell a letter inside a sound from a letter written and not heard.
func matchAll(patterns []pattern, word, phon string, covered []bool) []Hit {
	var hits []Hit
	for _, p := range patterns {
		if phon != "" && !confirms(p, phon) {
			continue
		}
		for _, m := range p.re.FindAllStringSubmatchIndex(word, -1) {
			lo, hi := m[0], m[1]
			if len(m) >= 4 && m[2] >= 0 {
				lo, hi = m[2], m[3] // the capture group is the trap itself
			}
			at := utf8.RuneCountInString(word[:lo])
			end := at + utf8.RuneCountInString(word[lo:hi])
			hits = append(hits, Hit{Rule: p.rule, Color: p.color, At: at, Len: end - at})
			for i := at; i < end && covered != nil; i++ {
				covered[i] = true
			}
		}
	}
	return hits
}

// confirms reports whether the pronunciation contains a sound this pattern is
// allowed to write. It is what keeps the on of bonne out of the Masquées.
func confirms(p pattern, phon string) bool {
	if len(p.sounds) == 0 {
		return true
	}
	for _, s := range p.sounds {
		if strings.Contains(phon, s) {
			return true
		}
	}
	return false
}

// accentHits reports every written accent, one trap per mark: a word wearing
// two of them is twice the thing to remember.
func accentHits(runes []rune) []Hit {
	var hits []Hit
	for i, r := range runes {
		if rule, ok := accents[r]; ok {
			hits = append(hits, Hit{Rule: rule, Color: engine.Accentuees, At: i, Len: 1})
		}
	}
	return hits
}
