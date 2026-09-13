// Package ui draws the pieces of the run screen. The keyboard is the most
// important of them after the card (brief/ENCRE_02 §11): at seven the finger is
// imprecise, so the geometry here is a correctness concern, not a cosmetic one.
package ui

import "fmt"

// Layout is the arrangement of the letter rows.
type Layout uint8

const (
	// AZERTY matches the physical keyboard of the computer, and is the default.
	AZERTY Layout = iota
	// ABC is alphabetical, offered in the parent panel for the first weeks.
	ABC
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
// X, Y, W and H are the TOUCH area in logical pixels, and they tile the
// keyboard without gap or overlap. The key drawn inside is inset by a few
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
		return "effacer"
	case KeyValidate:
		return "valider"
	default:
		return fmt.Sprintf("%c", k.Rune)
	}
}

// accentRow stays visible above the letters at all times (ENCRE_02 §11). The
// apostrophe rides with the accents because French spelling needs it as often.
const accentRow = "éèêàçùîô-'"

// columns is the width of every row in key cells. ENCRE_02 §11 fixes three rows
// of ten across the full width; the last row spends its four spare cells on the
// two special keys, two each, which is what makes them "grandes, à droite".
const columns = 10

var letterRows = map[Layout][]string{
	AZERTY: {"azertyuiop", "qsdfghjklm", "wxcvbn"},
	ABC:    {"abcdefghij", "klmnopqrst", "uvwxyz"},
}

// Keyboard is the drawn keyboard for one layout and one keyboard-area size.
type Keyboard struct {
	keys  []Key
	runes map[rune]bool
}

// NewKeyboard lays the keyboard out inside an area of areaW by areaH logical
// pixels, whose origin is the keyboard's own top-left corner. The rows are
// centred in whatever the cell arithmetic leaves over.
func NewKeyboard(l Layout, areaW, areaH int) *Keyboard {
	rows := append([]string{accentRow}, letterRows[l]...)

	cellW := areaW / columns
	cellH := areaH / len(rows)
	originX := (areaW - cellW*columns) / 2
	originY := (areaH - cellH*len(rows)) / 2

	kb := &Keyboard{runes: make(map[rune]bool, 36)}
	for row, chars := range rows {
		col := 0
		for _, r := range chars {
			kb.add(Key{
				Kind: KeyRune, Rune: r,
				X: originX + col*cellW, Y: originY + row*cellH,
				W: cellW, H: cellH,
			})
			col++
		}
		// The letters of the last row stop short of the full width; the two
		// special keys take the remaining cells, split evenly.
		if row == len(rows)-1 && col < columns {
			spare := columns - col
			for i, kind := range [2]KeyKind{KeyErase, KeyValidate} {
				w := spare / 2
				if i == 1 {
					w = spare - spare/2 // the odd cell, if any, goes to Valider
				}
				kb.add(Key{
					Kind: kind,
					X:    originX + col*cellW, Y: originY + row*cellH,
					W: w * cellW, H: cellH,
				})
				col += w
			}
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

// Accepts reports whether r is a character this keyboard can produce.
func (k *Keyboard) Accepts(r rune) bool { return k.runes[r] }

// MinTouchSide returns the shortest side of the smallest touch area, in logical
// pixels. Pair it with [PhysicalPx] to check the >= 48 physical pixels that
// ENCRE_02 §11 requires.
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

// PhysicalPx converts a length in logical pixels to physical device pixels.
//
// Ebitengine stretches the screenLen logical pixels the game draws across the
// outsideLen device-independent pixels the window occupies, and the device then
// renders each of those with deviceScale physical pixels. Both steps count: a
// game that ignores the first measures the phone it was written on rather than
// the phone it is running on.
//
// It returns 0 when screenLen is 0, which is what Ebitengine passes before the
// window exists.
func PhysicalPx(logical, screenLen, outsideLen int, deviceScale float64) float64 {
	if screenLen == 0 {
		return 0
	}
	return float64(logical) * float64(outsideLen) / float64(screenLen) * deviceScale
}
