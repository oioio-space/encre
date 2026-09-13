package lexique_test

import (
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

// TestAnalyzeSentenceAccordees walks the agreements of ENCRE_03 §2, which are
// the one Couleur a word cannot carry on its own: only the sentence around it
// says whether the s of chats is a plural or part of the word.
func TestAnalyzeSentenceAccordees(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		sentence string
		target   string
		want     engine.Rule // "" when the word carries no agreement
	}{
		{"pluriel en s", "les chats dorment", "chats", lexique.RulePlurielS},
		{"pluriel en x", "des bateaux passent", "bateaux", lexique.RulePlurielX},
		{"féminin en e", "une grande fille", "grande", lexique.RuleFemininE},
		{"verbe en ent", "ils chantent", "chantent", lexique.RuleVerbeEnt},
		{"elles aussi", "elles jouent", "jouent", lexique.RuleVerbeEnt},
		{"singulier", "un chat noir", "chat", ""},
		{"verbe au singulier", "le chat mange", "mange", ""},
		{"nom en s au singulier", "le bras droit", "bras", ""},
		{"nom en e au masculin", "un arbre vert", "arbre", ""},
	}

	lex := lexique.Embedded()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := lex.AnalyzeSentence(tc.sentence, nil)
			i := slices.IndexFunc(got, func(a lexique.Analysis) bool { return a.Word == tc.target })
			if i < 0 {
				t.Fatalf("AnalyzeSentence(%q) did not return %q", tc.sentence, tc.target)
			}
			a := got[i]
			if tc.want == "" {
				if a.Traps[engine.Accordees] != 0 {
					t.Errorf("%q in %q: got Accordées %v, want none", tc.target, tc.sentence, a.Rules())
				}
				return
			}
			if !slices.Contains(a.Rules(), tc.want) {
				t.Errorf("%q in %q: got %v, want rule %q", tc.target, tc.sentence, a.Rules(), tc.want)
			}
		})
	}
}

// TestAnalyzeSentenceTargets checks that naming the targets restricts the
// answer to them, which is how a list of sentences with gaps is analysed.
func TestAnalyzeSentenceTargets(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	const s = "les chats noirs dorment"
	got := lex.AnalyzeSentence(s, []lexique.Span{{Start: 4, End: 9}})
	if len(got) != 1 || got[0].Word != "chats" {
		t.Fatalf("AnalyzeSentence with one target returned %d analyses, want chats alone", len(got))
	}
	if got[0].Traps[engine.Accordees] == 0 {
		t.Errorf("chats in %q carries no Accordée, rules %v", s, got[0].Rules())
	}
}

// TestPluralSupersedesTheSilentLetter guards the rule that one letter teaches
// one thing: out of context the s of chats is silent, inside « les chats » it
// is the plural, and it must never be both at once.
func TestPluralSupersedesTheSilentLetter(t *testing.T) {
	t.Parallel()

	lex := lexique.Embedded()
	alone := lex.Analyze("chats")
	if !slices.Contains(alone.Rules(), lexique.RuleSMuet) {
		t.Errorf("Analyze(chats) = %v, want the s read as a silent letter", alone.Rules())
	}

	got := lex.AnalyzeSentence("les chats dorment", nil)
	i := slices.IndexFunc(got, func(a lexique.Analysis) bool { return a.Word == "chats" })
	if i < 0 {
		t.Fatal("AnalyzeSentence did not return chats")
	}
	a := got[i]
	if slices.Contains(a.Rules(), lexique.RuleSMuet) {
		t.Errorf("chats in a sentence = %v, want the s counted as the plural only", a.Rules())
	}
	if a.Traps[engine.Muettes] != alone.Traps[engine.Muettes]-1 {
		t.Errorf("Muettes = %d, want one fewer than the %d the word carries alone",
			a.Traps[engine.Muettes], alone.Traps[engine.Muettes])
	}
}
