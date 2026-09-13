package lexique_test

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

// minAccuracy is what ENCRE_05 T10 asks of the analyser: nine words in ten with
// exactly the right Couleurs, not one too many and not one too few.
const minAccuracy = 0.90

// TestCorpusCE1 measures the analyser against 200-odd CE1 words labelled by
// hand from the rules of ENCRE_03 §2, before the analyser ever ran on them.
func TestCorpusCE1(t *testing.T) {
	t.Parallel()

	words := loadCorpus(t, "testdata/ce1_200.tsv")
	lex := lexique.Embedded()

	right := 0
	for _, w := range words {
		a := lex.Analyze(w.word)
		got := colorsOf(a)
		if slices.Equal(got, w.colors) {
			right++
			continue
		}
		t.Logf("%-12s attendu %-40s obtenu %-40s (%v)", w.word,
			strings.Join(names(w.colors), ","), strings.Join(names(got), ","), a.Rules())
	}

	accuracy := float64(right) / float64(len(words))
	t.Logf("%d/%d mots justes — %.1f %%", right, len(words), accuracy*100)
	if accuracy < minAccuracy {
		t.Errorf("justesse %.1f %%, minimum %.0f %%", accuracy*100, minAccuracy*100)
	}
}

type labelled struct {
	word   string
	colors []engine.Color
}

func loadCorpus(t *testing.T, path string) []labelled {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("corpus: %v", err)
	}
	defer func() { _ = f.Close() }()

	byName := map[string]engine.Color{}
	for _, c := range engine.Colors() {
		byName[c.String()] = c
	}

	var out []labelled
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if strings.HasPrefix(text, "#") || strings.TrimSpace(text) == "" {
			continue
		}
		cols := strings.Split(text, "\t")
		if len(cols) != 2 {
			t.Fatalf("%s:%d: %d colonnes, il en faut 2", path, line, len(cols))
		}
		l := labelled{word: cols[0]}
		for _, name := range strings.Split(cols[1], ",") {
			if name == "" {
				continue
			}
			c, ok := byName[name]
			if !ok {
				t.Fatalf("%s:%d: Couleur inconnue %q", path, line, name)
			}
			l.colors = append(l.colors, c)
		}
		slices.Sort(l.colors)
		out = append(out, l)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("corpus: %v", err)
	}
	return out
}

func colorsOf(a lexique.Analysis) []engine.Color {
	var out []engine.Color
	for _, c := range engine.Colors() {
		if a.Traps[c] > 0 {
			out = append(out, c)
		}
	}
	return out
}

func names(cs []engine.Color) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.String())
	}
	return out
}
