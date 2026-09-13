package lexique_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

// TestAnalyzeSemaineA walks the ten words ENCRE_03 §3 labels by hand. It is the
// only table in the brief that gives both the Couleurs and the rules, so it is
// the closest thing to a ground truth the analyser has.
func TestAnalyzeSemaineA(t *testing.T) {
	t.Parallel()

	cases := []struct {
		word   string
		colors []engine.Color
		rules  []engine.Rule
	}{
		{"chat", []engine.Color{engine.Masquees, engine.Muettes}, []engine.Rule{lexique.RuleCH, lexique.RuleTMuet}},
		{"loup", []engine.Color{engine.Masquees, engine.Muettes}, []engine.Rule{lexique.RuleOU, lexique.RulePCMuet}},
		{"pomme", []engine.Color{engine.Jumelles, engine.Muettes}, []engine.Rule{lexique.RuleMM, lexique.RuleEMuet}},
		{"école", []engine.Color{engine.Accentuees, engine.Muettes}, []engine.Rule{lexique.RuleEAigu, lexique.RuleEMuet}},
		{"jour", []engine.Color{engine.Masquees}, []engine.Rule{lexique.RuleOU}},
		{"fille", []engine.Color{engine.Masquees, engine.Jumelles}, []engine.Rule{lexique.RuleILL, lexique.RuleLL}},
	}

	lex := lexique.Embedded()
	for _, tc := range cases {
		t.Run(tc.word, func(t *testing.T) {
			t.Parallel()
			got := lex.Analyze(tc.word)
			for _, c := range tc.colors {
				if got.Traps[c] == 0 {
					t.Errorf("Analyze(%q).Traps[%v] = 0, want at least 1 (rules %v)", tc.word, c, got.Rules())
				}
			}
			for _, r := range tc.rules {
				if !slices.Contains(got.Rules(), r) {
					t.Errorf("Analyze(%q) missed rule %q, got %v", tc.word, r, got.Rules())
				}
			}
		})
	}
}
