package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/content"
)

func TestFinDeTempsLineReturnsAnEmbeddedLine(t *testing.T) {
	pack := content.Embedded()

	got := game.FinDeTempsLine(pack, 0)
	if got == "" {
		t.Fatal("FinDeTempsLine(0) = \"\", want a line from content/data/phalene.json")
	}
	for _, other := range pack.Lines("fin_de_temps") {
		if got == other {
			return
		}
	}
	t.Errorf("FinDeTempsLine(0) = %q, not one of the pack's own fin_de_temps lines", got)
}

func TestFinDeTempsLineWrapsAroundTheAvailableLines(t *testing.T) {
	pack := content.Embedded()
	n := len(pack.Lines("fin_de_temps"))
	if n == 0 {
		t.Fatal("content pack has no fin_de_temps lines to test against")
	}

	if got, want := game.FinDeTempsLine(pack, n), game.FinDeTempsLine(pack, 0); got != want {
		t.Errorf("FinDeTempsLine(%d) = %q, want it to wrap to FinDeTempsLine(0) = %q", n, got, want)
	}
}

func TestFinDeTempsLineOfAPackWithoutTheContextIsEmpty(t *testing.T) {
	empty := &content.Pack{}

	if got := game.FinDeTempsLine(empty, 0); got != "" {
		t.Errorf("FinDeTempsLine on an empty pack = %q, want \"\"", got)
	}
}
