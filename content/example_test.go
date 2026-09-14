package content_test

import (
	"fmt"

	"github.com/oioio-space/encre/content"
	"github.com/oioio-space/encre/engine"
)

// A Talisman the engine scores is looked up by the engine's own id, so the card
// on screen and the rule behind it can never drift apart.
func ExamplePack_Talisman() {
	t, ok := content.Embedded().Talisman(engine.Perroquet)
	if !ok {
		return
	}
	fmt.Println(t.Name)
	fmt.Println(t.Line)
	fmt.Println(t.Flavor) // said by Phalène, never written on screen
	// Output:
	// Le Perroquet
	// Réécoute gratuite. +1 Mult.
	// Il répète tout ce qu'on lui dit.
}
