package lexique_test

import (
	"fmt"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

// The parent pastes a word; the analyser names the traps it carries and where
// they are written, so the card can light them one Couleur at a time.
func ExampleLexicon_Analyze() {
	a := lexique.Embedded().Analyze("chat")

	fmt.Println("Masquées :", a.Traps[engine.Masquees])
	fmt.Println("Muettes  :", a.Traps[engine.Muettes])
	fmt.Println("règles   :", a.Rules())
	fmt.Println("lettres  :", a.Positions())
	fmt.Println("famille  :", a.Family)
	// Output:
	// Masquées : 1
	// Muettes  : 1
	// règles   : [ch t_muet]
	// lettres  : [0 1 3]
	// famille  : chatte
}

// The Accordées are the one Couleur a word cannot carry alone: only the words
// around it say whether its s is a plural or part of the spelling. Note that
// the s is the plural here and nothing else — out of the sentence the same
// letter would be read as a silent one.
func ExampleLexicon_AnalyzeSentence() {
	for _, a := range lexique.Embedded().AnalyzeSentence("les chats dorment", nil) {
		if a.Traps[engine.Accordees] > 0 {
			fmt.Println(a.Word, a.Rules())
		}
	}
	// Output:
	// chats [ch t_muet pluriel_s]
}
