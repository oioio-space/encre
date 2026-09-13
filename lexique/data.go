package lexique

import (
	"bufio"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"strings"
	"sync"
)

//go:embed data/lexique.tsv.gz
var embedded embed.FS

// Attribution is the credit the embedded data is distributed under. It is
// repeated in LICENSES.md and must travel with any copy of the database
// (ENCRE_04 §5 asks for the licence to be verified before embedding).
const Attribution = "Lexique 3.83 — New, B., Pallier, C., Ferrand, L., Matos, R. — " +
	"http://www.lexique.org — CC BY-SA 4.0"

var (
	once    sync.Once
	shared  *Lexicon
	loadErr error
)

// Embedded returns the lexicon built into the binary, loaded once and shared.
//
// It panics if the embedded database cannot be read, because that is a broken
// build rather than a runtime condition: the file ships inside the binary.
func Embedded() *Lexicon {
	once.Do(func() {
		f, err := embedded.Open("data/lexique.tsv.gz")
		if err != nil {
			loadErr = err
			return
		}
		defer func() { _ = f.Close() }()
		shared, loadErr = Load(f)
	})
	if loadErr != nil {
		panic(fmt.Sprintf("lexique: embedded database unreadable: %v", loadErr))
	}
	return shared
}

// Load reads a gzipped TSV lexicon: one form per line, with the columns
// ortho, phon, cgram, nombre, famille. The first line names them.
func Load(r io.Reader) (*Lexicon, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("lexique: open gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	lex := &Lexicon{words: make(map[string]Entry, 20000)}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 0; sc.Scan(); line++ {
		text := sc.Text()
		if line == 0 || text == "" {
			continue // the header, or the trailing newline
		}
		f := strings.Split(text, "\t")
		if len(f) != 5 {
			return nil, fmt.Errorf("lexique: line %d has %d columns, want 5", line+1, len(f))
		}
		lex.words[f[0]] = Entry{Phon: f[1], Gram: f[2], Nombre: f[3], Family: f[4]}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("lexique: read: %w", err)
	}
	return lex, nil
}
