package gen_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/gen"
)

// specPromptBlock extracts the fenced prompt block under ENCRE_03 §9 from
// the brief itself.
var specPromptBlock = regexp.MustCompile(`(?s)## 9\. Génération des phrases.*?` + "```\n(.*?)```")

// TestPromptMatchesENCRE03Exactly reads brief/ENCRE_03_contenu_pedagogique.md
// and checks that [gen.BuildPrompt] reproduces its §9 prompt byte for byte
// — the instruction this ticket repeats twice: "lis-le, ne le paraphrase
// pas". A change to either side that drifts from the other fails this test,
// not a human skim.
func TestPromptMatchesENCRE03Exactly(t *testing.T) {
	raw, err := os.ReadFile("../../brief/ENCRE_03_contenu_pedagogique.md")
	if err != nil {
		t.Fatalf("reading brief/ENCRE_03_contenu_pedagogique.md: %v", err)
	}
	m := specPromptBlock.FindSubmatch(raw)
	if m == nil {
		t.Fatal("could not find the §9 prompt block in the brief")
	}
	want := string(m[1])

	got := gen.BuildPrompt(gen.WordRequest{Mot: "{{liste avec Couleurs}}"})
	if got != want {
		t.Errorf("BuildPrompt() does not match ENCRE_03 §9 verbatim:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestBuildPromptFormatsCouleurs checks the one substitution BuildPrompt is
// allowed to make: the word and its Couleurs, in the "mot (Couleur1,
// Couleur2)" shape the prompt's own example line uses for "cible"/"forme".
func TestBuildPromptFormatsCouleurs(t *testing.T) {
	got := gen.BuildPrompt(gen.WordRequest{Mot: "chats", Couleurs: []engine.Color{engine.Accordees, engine.Muettes}})
	want := "Mots : chats (Accordées, Muettes)\n"
	if len(got) < len(want) || got[len(got)-len(want):] != want {
		t.Errorf("BuildPrompt() tail = %q, want %q", got[max(0, len(got)-len(want)):], want)
	}
}

// TestBuildPromptWithNoCouleurs checks that a word with no Couleur is
// written bare, with no empty parentheses.
func TestBuildPromptWithNoCouleurs(t *testing.T) {
	got := gen.BuildPrompt(gen.WordRequest{Mot: "chat"})
	want := "Mots : chat\n"
	if len(got) < len(want) || got[len(got)-len(want):] != want {
		t.Errorf("BuildPrompt() tail = %q, want %q", got[max(0, len(got)-len(want)):], want)
	}
}
