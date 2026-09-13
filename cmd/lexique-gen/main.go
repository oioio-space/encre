// Command lexique-gen builds the compressed lexicon the lexique package embeds.
//
// It reads Lexique 3.83 (http://www.lexique.org, CC BY-SA 4.0), keeps the most
// frequent forms, merges the homographs, and works out for each word a relative
// where its silent final letter is heard — chat → chaton — so that Phalène can
// justify a trap instead of asserting it.
//
// Usage:
//
//	go run ./cmd/lexique-gen -in Lexique383.tsv -out lexique/data/lexique.tsv.gz
package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/oioio-space/encre/lexique"
)

// digraphs are the sounds a silent final letter makes once the next letter
// joins it: the c of blanc is not heard alone, but blanche says ch. ENCRE_03 §2
// teaches the family exactly that way.
var digraphs = map[string]string{"ch": "S", "gn": "N", "ph": "f"}

type form struct {
	ortho, phon, lemme string
	gram, nomb         string
	family             string
	freq               float64
}

func main() {
	in := flag.String("in", "Lexique383.tsv", "Lexique 3.83 TSV")
	out := flag.String("out", "lexique/data/lexique.tsv.gz", "compressed lexicon to write")
	keep := flag.Int("keep", 20000, "how many forms to keep, most frequent first")
	flag.Parse()

	all, err := read(*in)
	if err != nil {
		log.Fatal(err)
	}
	merged := merge(all)
	kept := mostFrequent(merged, *keep)
	families(kept, merged)
	if err := write(*out, kept); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d forms kept out of %d, %d with a family\n", len(kept), len(merged), countFamilies(kept))
}

// read parses the columns of Lexique 3.83 this game needs.
func read(path string) ([]form, error) {
	// #nosec G304 -- the path is a build-time flag, typed by the developer
	// regenerating the database; there is no untrusted input in this tool.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() {
		return nil, fmt.Errorf("%s: empty", path)
	}
	cols := index(strings.Split(sc.Text(), "\t"))

	var forms []form
	for sc.Scan() {
		c := strings.Split(sc.Text(), "\t")
		if len(c) < len(cols) {
			continue
		}
		ortho := c[cols["ortho"]]
		if !plainWord(ortho) {
			continue
		}
		films, _ := strconv.ParseFloat(c[cols["freqfilms2"]], 64)
		books, _ := strconv.ParseFloat(c[cols["freqlivres"]], 64)
		forms = append(forms, form{
			ortho: ortho, phon: c[cols["phon"]], lemme: c[cols["lemme"]],
			gram: c[cols["cgram"]], nomb: c[cols["nombre"]],
			freq: max(films, books),
		})
	}
	return forms, sc.Err()
}

func index(header []string) map[string]int {
	idx := map[string]int{}
	for i, name := range header {
		idx[name] = i
	}
	for _, want := range []string{"ortho", "phon", "lemme", "cgram", "nombre", "freqfilms2", "freqlivres"} {
		if _, ok := idx[want]; !ok {
			log.Fatalf("column %q missing from the header", want)
		}
	}
	return idx
}

// plainWord keeps the single words a dictation can contain and drops the rest:
// no spaces, no digits, no abbreviations.
func plainWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r == '\'', r == '-':
		case strings.ContainsRune("àâäçéèêëîïôöùûüÿœæ", r):
		default:
			return false
		}
	}
	return true
}

// merge folds the homographs into one entry per spelling: the pronunciation of
// the most frequent reading, and every grammatical label any reading carries,
// because « grand » is an adjective, an adverb and a noun at once.
func merge(forms []form) map[string]*form {
	byOrtho := map[string]*form{}
	sets := map[string]*labels{}
	for _, f := range forms {
		cur, ok := byOrtho[f.ortho]
		if !ok {
			kept := f
			byOrtho[f.ortho] = &kept
			sets[f.ortho] = &labels{map[string]bool{}, map[string]bool{}, map[string]bool{}}
			cur = &kept
		} else if f.freq > cur.freq {
			cur.phon, cur.freq = f.phon, f.freq
		}
		s := sets[f.ortho]
		add(s.gram, f.gram)
		add(s.nomb, f.nomb)
		add(s.lemme, f.lemme)
	}
	for ortho, f := range byOrtho {
		s := sets[ortho]
		f.gram, f.nomb, f.lemme = join(s.gram), join(s.nomb), join(s.lemme)
	}
	return byOrtho
}

// labels gathers every grammatical value the homographs of one spelling carry.
type labels struct{ gram, nomb, lemme map[string]bool }

func add(set map[string]bool, v string) {
	if v != "" {
		set[v] = true
	}
}

func join(set map[string]bool) string {
	return strings.Join(slices.Sorted(maps.Keys(set)), ",")
}

// mostFrequent keeps the n most frequent spellings, sorted so that the written
// file is byte-identical from one build to the next.
func mostFrequent(byOrtho map[string]*form, n int) []*form {
	forms := make([]*form, 0, len(byOrtho))
	for _, f := range byOrtho {
		forms = append(forms, f)
	}
	sort.Slice(forms, func(i, j int) bool {
		if forms[i].freq != forms[j].freq {
			return forms[i].freq > forms[j].freq
		}
		return forms[i].ortho < forms[j].ortho
	})
	if n < len(forms) {
		forms = forms[:n]
	}
	sort.Slice(forms, func(i, j int) bool { return forms[i].ortho < forms[j].ortho })
	return forms
}

const (
	// maxFamilyGrowth is how many letters a relative may add. Beyond that the
	// word stops explaining anything to a seven-year-old.
	maxFamilyGrowth = 6
	// minFamilyFreq keeps the relative to a word a child could have met. Without
	// it blanc is explained by a legal term no child has met, which explains
	// nothing.
	minFamilyFreq = 1.0
)

// derivations are the endings that make a real French family. Sharing a prefix
// and a sound is not enough — litre begins with lit and says its t, and teaches
// nothing about a bed. A relative has to be an inflection of the word, or carry
// one of these.
// A bare -e is deliberately absent: the feminine is already caught as an
// inflection (grande is listed under grand), while loupe is not a loup and
// tarde is not tard.
var derivations = []string{
	"he", "hes", "on", "onne", "ette", "et", "eau", "erie", "age", "ure",
	"in", "ine", "eur", "euse", "eux", "elle", "ement", "ième", "iste",
	"ot", "aine", "ade", "ie",
}

// families finds, for each kept word, a longer word of the whole lexicon that
// begins with it and says one more sound — the sound its last letter writes and
// does not make. chat is silent on its t; chaton is not.
func families(kept []*form, all map[string]*form) {
	orthos := make([]string, 0, len(all))
	for o := range all {
		orthos = append(orthos, o)
	}
	sort.Strings(orthos)

	for _, f := range kept {
		last := []rune(f.ortho)
		wanted := lexique.Heard[last[len(last)-1]]
		if len(wanted) == 0 || f.phon == "" {
			continue
		}
		f.family = pickFamily(f, wanted, orthos, all)
	}
}

// pickFamily returns the relative that explains the silent letter best.
func pickFamily(f *form, wanted []string, orthos []string, all map[string]*form) string {
	lo := sort.SearchStrings(orthos, f.ortho+"\x00")
	var best *form
	for i := lo; i < len(orthos) && strings.HasPrefix(orthos[i], f.ortho); i++ {
		c := all[orthos[i]]
		if len(c.ortho) > len(f.ortho)+maxFamilyGrowth || c.freq < minFamilyFreq ||
			strings.ContainsRune(c.ortho, '-') || !strings.HasPrefix(c.phon, f.phon) {
			continue
		}
		extra := c.phon[len(f.phon):]
		if !startsWithAny(extra, wanted) && !startsWithAny(extra, joined(f, c)) {
			continue
		}
		if !related(f, c) {
			continue
		}
		if best == nil || better(c, best) {
			best = c
		}
	}
	if best == nil {
		return ""
	}
	return best.ortho
}

// better prefers the relative the child is likeliest to already know: the most
// frequent, then the shortest.
func better(c, best *form) bool {
	if c.freq != best.freq {
		return c.freq > best.freq
	}
	if len(c.ortho) != len(best.ortho) {
		return len(c.ortho) < len(best.ortho)
	}
	return c.ortho < best.ortho
}

// related reports whether the candidate is an inflection of the word — chatte
// is listed under the lemma chat — or a derivation with a recognised ending.
func related(f, c *form) bool {
	for _, lemma := range strings.Split(c.lemme, ",") {
		if lemma == f.ortho {
			return true
		}
	}
	added := c.ortho[len(f.ortho):]
	for _, d := range derivations {
		if added == d {
			return true
		}
	}
	return false
}

// joined returns the sound the word's last letter makes once the candidate's
// next letter joins it, if the two make a digraph.
func joined(f, c *form) []string {
	pair := f.ortho[len(f.ortho)-1:] + c.ortho[len(f.ortho):len(f.ortho)+1]
	if sound, ok := digraphs[pair]; ok {
		return []string{sound}
	}
	return nil
}

func startsWithAny(s string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(s, p) })
}

func countFamilies(kept []*form) int {
	n := 0
	for _, f := range kept {
		if f.family != "" {
			n++
		}
	}
	return n
}

func write(path string, kept []*form) error {
	// #nosec G304 -- the path is a build-time flag; see read.
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(gz)
	if _, err := fmt.Fprintln(w, "ortho\tphon\tcgram\tnombre\tfamille"); err != nil {
		return err
	}
	for _, e := range kept {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.ortho, e.phon, e.gram, e.nomb, e.family); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return f.Close()
}
