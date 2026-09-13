package engine

import "fmt"

// Color is one of the six Couleurs: the families of spelling trap ENCRE_03 §4
// teaches, and the only thing the palette of ENCRE_02 §3 is allowed to signal.
type Color int

// The six Couleurs, in the order ENCRE_04 §4 lists them.
const (
	Muettes    Color = iota // final letters written and not heard: chat, gros
	Jumelles                // doubled consonants: pomme, bonne
	Accentuees              // the accents: école, mère, forêt
	Masquees                // a sound spelled several ways: eau, au, o
	Sosies                  // words that sound alike: vert, verre, vers
	Accordees               // agreement carried by context: les chats noirs
)

// colorNames are the names read aloud to the child and printed on the score,
// so they are part of the contract rather than debug output (ENCRE_03 §4).
var colorNames = [...]string{
	Muettes:    "Muettes",
	Jumelles:   "Jumelles",
	Accentuees: "Accentuées",
	Masquees:   "Masquées",
	Sosies:     "Sosies",
	Accordees:  "Accordées",
}

// String returns the Couleur's name as the game says it.
func (c Color) String() string {
	if c < 0 || int(c) >= len(colorNames) {
		return fmt.Sprintf("Couleur inconnue (%d)", int(c))
	}
	return colorNames[c]
}

// Colors returns the six Couleurs, in order.
func Colors() []Color {
	return []Color{Muettes, Jumelles, Accentuees, Masquees, Sosies, Accordees}
}

// Rule is one spelling rule inside a Couleur, such as "ch" or "t_muet"
// (ENCRE_03 §4). It names the justification said aloud when the word is missed.
type Rule string

// Kind tells what the child is asked to write.
type Kind int

const (
	// KindWord is a single word.
	KindWord Kind = iota
	// KindSentence is a sentence with a gap to fill.
	KindSentence
	// KindDictation is a dictation, cut at its punctuation.
	KindDictation
)

// Shine is the rarity a word can come back wearing (ENCRE_01).
type Shine int

const (
	// ShineNone is an ordinary word.
	ShineNone Shine = iota
	// ShineHolo is the holographic finish, roughly one word in eight.
	ShineHolo
	// ShinePoly is polychrome, roughly one in forty.
	ShinePoly
)

// Word is something to spell, with the traps it carries.
type Word struct {
	ID      string
	Text    string
	Letters int
	// Traps counts the traps per Couleur. It drives the chips the word pays,
	// so a word with none is worth only its letters.
	Traps  map[Color]int
	Rules  []Rule
	Family string
	Kind   Kind
	// Sentence is the sentence with its gap, when Kind is KindSentence.
	Sentence string
}

// TrapCount is how many traps the word carries across every Couleur.
func (w Word) TrapCount() int {
	n := 0
	for _, c := range w.Traps {
		n += c
	}
	return n
}

// WordState is what one child has done with one word.
//
// Its zero value is a word never met, because decks are built from a map with
// no entry for a word the child has not seen yet.
type WordState struct {
	// Mastery rises with success and falls with failure; it feeds PHat.
	Mastery float64
	// SuccessDays are the distinct days the word was spelled right, as days
	// since the epoch. Gold asks for GoldDays of them spread over
	// GoldMinSpanDays, so it means remembered rather than drilled in one go.
	SuccessDays  []int32
	FirstSuccess int32
	Fails        int
	// FailWeeks are the weeks a failure fell in, for the curse.
	FailWeeks []int32
	// ConsecOK is the run of successes that tames a cursed word.
	ConsecOK                      int
	Gold, Tarnished, Cursed, Seen bool
	Shine                         Shine
	// LastPlayedW is the last week the word was played, for the tarnish.
	LastPlayedW int32
}
