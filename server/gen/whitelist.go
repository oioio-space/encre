package gen

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"strings"
	"sync"
)

//go:embed data/ce1_whitelist.txt
var whitelistFS embed.FS

// Whitelist is the set of words a generated sentence may use outside its
// target word (ENCRE_04 §9): a sentence containing anything else is
// rejected by [Filter]. Every word is stored folded ([fold]), so lookup is
// case- and accent-position-insensitive the same way the target-word
// exemption in [Filter] is.
type Whitelist map[string]bool

// Contains reports whether word (in any case) is in the whitelist.
func (wl Whitelist) Contains(word string) bool { return wl[fold(word)] }

// ParseWhitelist reads one whitelist word (or several, whitespace-separated)
// per line from r. A line whose first non-space character is "#" is a
// comment; blank lines are ignored.
func ParseWhitelist(r *bufio.Scanner) Whitelist {
	wl := Whitelist{}
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for w := range strings.FieldsSeq(line) {
			wl[fold(w)] = true
		}
	}
	return wl
}

var (
	embeddedOnce sync.Once
	embeddedWL   Whitelist
	embeddedErr  error
)

// EmbeddedWhitelist returns the CE1 whitelist built into the binary, parsed
// once and shared. See data/ce1_whitelist.txt's own header comment for what
// it currently covers and how to grow it.
//
// It panics if the embedded file cannot be read, the same broken-build
// contract [content.Embedded] and [lexique.Embedded] already hold.
func EmbeddedWhitelist() Whitelist {
	embeddedOnce.Do(func() {
		b, err := whitelistFS.ReadFile("data/ce1_whitelist.txt")
		if err != nil {
			embeddedErr = err
			return
		}
		embeddedWL = ParseWhitelist(bufio.NewScanner(bytes.NewReader(b)))
	})
	if embeddedErr != nil {
		panic(fmt.Sprintf("gen: embedded CE1 whitelist unreadable: %v", embeddedErr))
	}
	return embeddedWL
}
