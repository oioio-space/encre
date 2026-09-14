package lexique_test

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/oioio-space/encre/lexique"
)

// corpusWords lists the words of the CE1 corpus, with no regard for their
// labelled Couleurs — it exists only to seed a fuzz corpus with real
// vocabulary rather than starting from nothing.
func corpusWords(tb testing.TB, path string) []string {
	tb.Helper()
	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("corpus: %v", err)
	}
	defer func() { _ = f.Close() }()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		word, _, _ := strings.Cut(line, "\t")
		out = append(out, word)
	}
	if err := sc.Err(); err != nil {
		tb.Fatalf("corpus: %v", err)
	}
	return out
}

// checkHitsFitTheWord is Analysis' one hard safety promise: every Hit the
// analyser returns has to fall inside the word it was found in, in runes, not
// bytes — the client lights up letters at these offsets, and one past the end
// would either panic the display or light up the wrong word entirely.
func checkHitsFitTheWord(t *testing.T, word string, a lexique.Analysis) {
	t.Helper()
	n := utf8.RuneCountInString(word)
	for _, h := range a.Hits {
		if h.At < 0 || h.Len < 0 || h.At+h.Len > n {
			t.Fatalf("Analyze(%q) hit %+v falls outside the word's %d letters", word, h, n)
		}
	}
	pos := a.Positions()
	if !slices.IsSorted(pos) {
		t.Fatalf("Analyze(%q).Positions() = %v, want them sorted", word, pos)
	}
	for _, p := range pos {
		if p < 0 || p >= n {
			t.Fatalf("Analyze(%q).Positions() = %v, %d is outside the word's %d letters", word, pos, p, n)
		}
	}
}

// FuzzAnalyzeNeverPanics is ENCRE_03 §10's first promise: Analyze always
// returns something, on any input a parent's paste box might hand it — never
// a panic, and never a Hit the client could not safely light up. The seed
// corpus mixes real CE1 vocabulary with the inputs most likely to break a
// hand-written parser: the empty string, invalid UTF-8, a pathologically long
// word, a string of nothing but accents, one of nothing but punctuation,
// combining marks stacked on a single letter, and an emoji.
func FuzzAnalyzeNeverPanics(f *testing.F) {
	for _, w := range corpusWords(f, "testdata/ce1_200.tsv") {
		f.Add(w)
	}
	f.Add("")
	f.Add(string([]byte{0xff, 0xfe, 0x80, 0xc0, 0xaf}))
	f.Add(strings.Repeat("a", 10000))
	f.Add("éèêëàâîïôöùûüçÉÈÊËÀÂÎÏÔÖÙÛÜÇ")
	f.Add("!?.,;:—«»()[]{}...")
	f.Add("éèêëȩ")
	f.Add("🐱🐶🦊")
	f.Add("'")
	f.Add("-")
	f.Add("''''''")
	f.Add(strings.Repeat("é", 2000))
	f.Add("\x00\x01\x02")
	f.Add("chat\x00chat")

	lex := lexique.Embedded()
	f.Fuzz(func(t *testing.T, word string) {
		a := lex.Analyze(word)
		checkHitsFitTheWord(t, word, a)

		// Traps is a tally derived from Hits and nothing else; the two must
		// never disagree, on any input, or a card could show a trap count
		// that does not match what it actually highlights.
		want := map[int]int{}
		for _, h := range a.Hits {
			want[int(h.Color)]++
		}
		for c, n := range a.Traps {
			if want[int(c)] != n {
				t.Fatalf("Analyze(%q).Traps[%v] = %d, want %d hits of that Couleur", word, c, n, want[int(c)])
			}
			delete(want, int(c))
		}
		for c, n := range want {
			t.Fatalf("Analyze(%q) has %d hits of Couleur %d missing from Traps", word, n, c)
		}
	})
}

// FuzzAnalyzeSentenceNeverPanics is the sentence-level twin of
// FuzzAnalyzeNeverPanics, checked against ENCRE_04 §5's other promise: every
// Analysis it returns names a word the sentence actually contains, at the
// span it was tokenised at — a paste screen that highlighted a word out of
// thin air would be worse than one that highlighted nothing.
func FuzzAnalyzeSentenceNeverPanics(f *testing.F) {
	for _, w := range corpusWords(f, "testdata/ce1_200.tsv") {
		f.Add("les " + w + " sont là")
	}
	f.Add("")
	f.Add("les chats dorment")
	f.Add("l'arbre et l'oiseau, peut-être")
	f.Add(string([]byte{0xff, 0xfe}))
	f.Add(strings.Repeat("chat ", 500))
	f.Add("🐱 les chats-huants dorment")
	f.Add("' ' ' - - -")
	f.Add("Ça, c'est aujourd'hui.")

	lex := lexique.Embedded()
	f.Fuzz(func(t *testing.T, sentence string) {
		got := lex.AnalyzeSentence(sentence, nil)
		for _, a := range got {
			checkHitsFitTheWord(t, a.Word, a)
			if !strings.Contains(sentence, a.Word) {
				t.Fatalf("AnalyzeSentence(%q) returned %q, which is not a substring of the sentence", sentence, a.Word)
			}
		}
	})
}
