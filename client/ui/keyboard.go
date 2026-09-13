// Package ui draws the pieces of the run screen. The keyboard is the most
// important of them after the card (brief/ENCRE_02 §11): at seven the finger is
// imprecise, so the geometry here is a correctness concern, not a cosmetic one.
package ui

import "fmt"

// Layout is the arrangement of the letter rows.
type Layout uint8

const (
	// Phone is seven alphabetical columns over five rows (ENCRE_06 §4).
	//
	// ENCRE_06 §2 overrode ENCRE_02 §11 to get here: ten AZERTY columns on a
	// 390-pixel screen give 39-pixel keys, under the 48 the charter demands of
	// a touch target, and a seven-year-old misses them. Alphabetical order is
	// what buys the width back, and at seven it is also the order the child
	// already knows.
	Phone Layout = iota
	// AZERTY is ten columns, for the tablet and the computer, where the width
	// is there and the arrangement can match the physical keyboard.
	AZERTY
)

// KeyKind tells a letter key from the two special ones.
type KeyKind uint8

const (
	// KeyRune types the character in Key.Rune.
	KeyRune KeyKind = iota
	// KeyErase is the eraser: it removes the last letter typed.
	KeyErase
	// KeyValidate is the seal: it submits the word.
	KeyValidate
)

// Key is one key of the drawn keyboard.
//
// X, Y, W and H are the TOUCH area in logical pixels, and the keys of a row
// tile it without gap or overlap. The key drawn inside is inset by a few
// pixels: at seven the finger lands wide of where the eye aimed, so the target
// is deliberately larger than the picture of it. Anything sizing a touch target
// must use these fields, not the drawn rectangle.
type Key struct {
	Kind KeyKind
	// Rune is the character typed, and is zero unless Kind is KeyRune.
	Rune       rune
	X, Y, W, H int
}

// Label names the key for humans — its character, or the name of the special.
func (k Key) Label() string {
	switch k.Kind {
	case KeyErase:
		return "efface"
	case KeyValidate:
		return "valide"
	default:
		return string(k.Rune)
	}
}

// columnsPerAccentRow is how many keys the accent row holds. The letter rows
// hold fewer and wider ones, which is what a key's proportion is judged on.
const columnsPerAccentRow = 10

// LetterColumns is how many letter keys a row of this layout holds. The accent
// row always holds ten, narrower ones.
func LetterColumns(l Layout) int {
	if l == AZERTY {
		return columnsPerAccentRow
	}
	return 7
}

// AccentRow is the row ENCRE_02 §11 keeps visible above the letters at all
// times. The apostrophe and the hyphen ride with the accents because written
// French needs them as often, and neither is reachable any other way.
const AccentRow = "éèêàçùîô-'"

// spec is one key before it has been given a place: a character or a special,
// and how many of the row's cells it spans.
type spec struct {
	kind KeyKind
	r    rune
	span int
}

func runes(s string) []spec {
	out := make([]spec, 0, len(s))
	for _, r := range s {
		out = append(out, spec{kind: KeyRune, r: r, span: 1})
	}
	return out
}

// rows returns the keyboard row by row. Rows deliberately hold different
// numbers of keys: the accent row is ten wide while the letter rows are seven,
// exactly as a physical keyboard staggers its rows. Forcing one grid on both is
// what put the accents out of reach.
func rows(l Layout) [][]spec {
	accents := runes(AccentRow)
	switch l {
	case AZERTY:
		// The specials take two cells each, filling the ten-cell row.
		last := append(runes("wxcvbn"),
			spec{kind: KeyErase, span: 2}, spec{kind: KeyValidate, span: 2})
		return [][]spec{accents, runes("azertyuiop"), runes("qsdfghjklm"), last}
	default:
		last := append(runes("vwxyz"),
			spec{kind: KeyErase, span: 1}, spec{kind: KeyValidate, span: 1})
		return [][]spec{accents, runes("abcdefg"), runes("hijklmn"), runes("opqrstu"), last}
	}
}

// Keyboard is the drawn keyboard for one layout and one keyboard-area size.
type Keyboard struct {
	keys  []Key
	runes map[rune]bool
}

// NewKeyboard lays the keyboard out inside an area of areaW by areaH logical
// pixels, whose origin is the keyboard's own top-left corner. Each row divides
// the width among its own cells, so a row of ten and a row of seven both span
// the keyboard.
func NewKeyboard(l Layout, areaW, areaH int) *Keyboard {
	all := rows(l)
	rowH := areaH / len(all)
	originY := (areaH - rowH*len(all)) / 2

	kb := &Keyboard{runes: make(map[rune]bool, 36)}
	for row, specs := range all {
		cells := 0
		for _, s := range specs {
			cells += s.span
		}
		cellW := areaW / cells
		x := (areaW - cellW*cells) / 2
		for _, s := range specs {
			w := cellW * s.span
			kb.add(Key{Kind: s.kind, Rune: s.r, X: x, Y: originY + row*rowH, W: w, H: rowH})
			x += w
		}
	}
	return kb
}

func (k *Keyboard) add(key Key) {
	k.keys = append(k.keys, key)
	if key.Kind == KeyRune {
		k.runes[key.Rune] = true
	}
}

// Keys returns every key, in reading order.
func (k *Keyboard) Keys() []Key { return k.keys }

// KeyAt returns the key whose touch area contains the logical point (x, y).
func (k *Keyboard) KeyAt(x, y int) (Key, bool) {
	for _, key := range k.keys {
		if x >= key.X && x < key.X+key.W && y >= key.Y && y < key.Y+key.H {
			return key, true
		}
	}
	return Key{}, false
}

// Accepts reports whether r is a character this keyboard can produce, counting
// the accents reachable by holding a key (see [Variants]).
func (k *Keyboard) Accepts(r rune) bool {
	if k.runes[r] {
		return true
	}
	for base := range k.runes {
		for _, v := range Variants(base) {
			if v == r {
				return true
			}
		}
	}
	return false
}

// MinTouchSide returns the shortest side of the smallest touch area, in logical
// pixels — the 48 of ENCRE_02 §11 is a figure in these same units.
func (k *Keyboard) MinTouchSide() int {
	side := 0
	for i, key := range k.keys {
		s := min(key.W, key.H)
		if i == 0 || s < side {
			side = s
		}
	}
	return side
}

// String renders the layout as text, for failure messages.
func (k *Keyboard) String() string { return fmt.Sprintf("keyboard of %d keys", len(k.keys)) }
