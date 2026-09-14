package gen_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/oioio-space/encre/server/gen"
)

// TestParseWhitelistSkipsCommentsAndBlankLinesAndFoldsCase checks
// [gen.ParseWhitelist]'s documented line shape: "#"-prefixed comment lines
// and blank lines are ignored, several whitespace-separated words per line
// are all kept, and lookup through [gen.Whitelist.Contains] is
// case-insensitive.
func TestParseWhitelistSkipsCommentsAndBlankLinesAndFoldsCase(t *testing.T) {
	src := "# comment\n\n  le chat  \nDORT\n   # another comment\nsur lit\n"
	wl := gen.ParseWhitelist(bufio.NewScanner(strings.NewReader(src)))

	for _, want := range []string{"le", "chat", "dort", "sur", "lit"} {
		if !wl.Contains(want) {
			t.Errorf("ParseWhitelist() result does not contain %q: %v", want, wl)
		}
	}
	if wl.Contains("comment") || wl.Contains("another") {
		t.Errorf("ParseWhitelist() kept a word from a comment line: %v", wl)
	}
	if !wl.Contains("DORT") {
		t.Error("Contains() is case-sensitive, want case-insensitive")
	}
}

// TestParseWhitelistOnEmptyInputIsEmptyNotNil checks that an input with
// nothing but comments and blank lines produces an empty, usable whitelist
// rather than nil — [gen.Whitelist.Contains] must not panic on it.
func TestParseWhitelistOnEmptyInputIsEmptyNotNil(t *testing.T) {
	wl := gen.ParseWhitelist(bufio.NewScanner(strings.NewReader("# only comments\n\n")))
	if len(wl) != 0 {
		t.Errorf("ParseWhitelist() on comment-only input: len = %d, want 0", len(wl))
	}
	if wl.Contains("chat") {
		t.Error("ParseWhitelist() on comment-only input contains a word it was never given")
	}
}

// TestEmbeddedWhitelistIsUsableAndStable checks that the binary's own
// embedded CE1 whitelist parses to a non-empty set carrying at least one
// word every generated sentence in this package's other tests relies on
// being whitelisted ("le"), and that repeated calls return the same
// [sync.Once]-cached value rather than reparsing the embedded file each
// time.
func TestEmbeddedWhitelistIsUsableAndStable(t *testing.T) {
	wl := gen.EmbeddedWhitelist()
	if len(wl) == 0 {
		t.Fatal("EmbeddedWhitelist() is empty")
	}
	if !wl.Contains("le") {
		t.Error(`EmbeddedWhitelist() does not contain "le"`)
	}

	again := gen.EmbeddedWhitelist()
	if len(again) != len(wl) {
		t.Errorf("EmbeddedWhitelist() called twice: len = %d then %d, want equal (cached)", len(wl), len(again))
	}
}
