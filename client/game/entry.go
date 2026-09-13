// Package game holds the client's loop, scenes and state. For the Étape 0
// prototype that is only the word being typed, which is enough to answer the
// one question the prototype exists to answer: can a seven-year-old enter an
// accented word on a real tablet without fighting the interface.
package game

import "github.com/oioio-space/encre/client/ui"

// Entry is the word being typed, letter by letter.
//
// It keeps runes rather than a string on purpose: every accented letter of
// French is two bytes in UTF-8, so erasing by byte would leave half a rune in
// the middle of the child's word. The zero value is not usable; call NewEntry.
type Entry struct {
	kb      *ui.Keyboard
	letters []rune
}

// NewEntry returns an empty word that accepts only what kb can produce.
func NewEntry(kb *ui.Keyboard) *Entry {
	return &Entry{kb: kb}
}

// Type appends r and reports whether it was accepted. It refuses anything the
// drawn keyboard cannot produce: the physical keyboard of the computer emits
// capitals, digits and symbols that have no place in the word.
func (e *Entry) Type(r rune) bool {
	if !e.kb.Accepts(r) {
		return false
	}
	e.letters = append(e.letters, r)
	return true
}

// Erase removes the last letter, and does nothing on an empty word.
func (e *Entry) Erase() {
	if len(e.letters) > 0 {
		e.letters = e.letters[:len(e.letters)-1]
	}
}

// Clear empties the word.
func (e *Entry) Clear() { e.letters = e.letters[:0] }

// Text returns the word as typed.
func (e *Entry) Text() string { return string(e.letters) }

// Len returns the number of letters, not bytes.
func (e *Entry) Len() int { return len(e.letters) }
