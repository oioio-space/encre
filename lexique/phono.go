package lexique

import (
	"strings"

	"github.com/oioio-space/encre/engine"
)

// Heard maps a written letter to the phonemes it can stand for, in the SAMPA
// alphabet Lexique 3.83 writes its pronunciations in. It answers one question:
// is this letter the one being said?
//
// It is exported because cmd/lexique-gen asks the same question from the other
// end — it looks for a relative where a silent letter is heard — and two copies
// of a phonetic table drift the first time one of them is corrected.
var Heard = map[rune][]string{
	't': {"t"}, 'd': {"d"}, 's': {"s", "z"}, 'x': {"ks", "s", "z"},
	'p': {"p"}, 'c': {"k", "s"}, 'e': {"°"}, 'r': {"R"}, 'z': {"z"},
	'g': {"g", "Z"}, 'l': {"l"}, 'n': {"n", "N"}, 'm': {"m"}, 'b': {"b"},
	'f': {"f"}, 'k': {"k"}, 'v': {"v"}, 'q': {"k"}, 'j': {"Z"}, 'w': {"w", "v"},
	'y': {"j", "i"},
}

// muetteRules names the Couleur Muettes rule a silent final letter carries.
// Anything not tabled by ENCRE_03 §2 falls to RuleLettreMuette.
var muetteRules = map[rune]engine.Rule{
	't': RuleTMuet, 'd': RuleDMuet, 's': RuleSMuet,
	'e': RuleEMuet, 'x': RuleXMuet, 'p': RulePCMuet, 'c': RulePCMuet,
}

const vowelLetters = "aeiouyéèêëàâîïôöùûü"

func isVowel(r rune) bool { return strings.ContainsRune(vowelLetters, r) }

// vowelSounds are the phonemes a written vowel can make, in SAMPA. A word whose
// pronunciation ends on one of them ends on a vowel that is being said.
const vowelSounds = "aeiouyEO°2915@§"

func endsOnVowelSound(phon string) bool {
	return phon != "" && strings.ContainsAny(phon[len(phon)-1:], vowelSounds)
}

func isNasalSound(phon string) bool {
	return strings.HasSuffix(phon, "@") || strings.HasSuffix(phon, "§") ||
		strings.HasSuffix(phon, "5") || strings.HasSuffix(phon, "1")
}

// maxSilentTail is how far back the walk will go. Three is temps — m, p and s
// after the nasal — and French does not stack more than that.
const maxSilentTail = 3

// silentTail walks the end of a word and returns the rule of each final letter
// that is written and not heard, outermost first, with the rune index of each.
//
// It compares the spelling against the pronunciation rather than guessing from
// the spelling alone, which is what ENCRE_03 §10 asks for: the d of grand is
// silent, the d of sud is not, and only the sound tells them apart. Without a
// pronunciation it returns nothing — a word the lexicon does not know gets no
// Muettes rather than a wrong one.
//
// covered marks the letters a Masquée already claimed. A letter inside a sound
// is not a silent letter: the l of fille belongs to ill, the n of pont to the
// nasal, the h of blanche to ch. Reaching one ends the walk, because whatever
// lies further in is being pronounced.
func silentTail(word, phon string, covered []bool) []Hit {
	if phon == "" {
		return nil
	}
	letters := []rune(word)

	// -ent is the third person plural, not three silent letters in a row: one
	// ending, one lesson, one trap. The nasal check is what tells it from the
	// -ent of vent and dent, which is a sound and not an ending at all. In a
	// sentence AnalyzeSentence goes further and calls it the Accordée it is.
	if len(letters) > 3 && strings.HasSuffix(word, "ent") && !isNasalSound(phon) {
		return []Hit{{Rule: RuleLettreMuette, Color: engine.Muettes, At: len(letters) - 3, Len: 3}}
	}

	var hits []Hit
	for i := len(letters) - 1; i > 0 && len(hits) < maxSilentTail; i-- {
		r := letters[i]
		if covered[i] || soundsLike(r, phon) {
			break
		}
		if isVowel(r) && r != 'e' {
			break // the final vowel is always said
		}
		// A word-final e is silent — porte, pomme. An e uncovered by stripping a
		// silent letter is the opposite: it is the vowel being said, the e of
		// chanter and of nez. Which of the two it is depends on whether
		// anything was stripped before reaching it, and on nothing else — the e
		// of poupée is still silent, because the sound before it is the é.
		if r == 'e' && len(hits) > 0 && endsOnVowelSound(phon) {
			break
		}
		rule, ok := muetteRules[r]
		if !ok {
			rule = RuleLettreMuette
		}
		hits = append(hits, Hit{Rule: rule, Color: engine.Muettes, At: i, Len: 1})
	}
	return hits
}

// soundsLike reports whether the pronunciation ends on a phoneme this letter
// could have written.
func soundsLike(r rune, phon string) bool {
	for _, p := range Heard[r] {
		if strings.HasSuffix(phon, p) {
			return true
		}
	}
	return false
}
