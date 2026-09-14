package ui

// Rect is a touch area in logical pixels, top-left corner and size.
type Rect struct {
	X, Y, W, H int
}

// speakerSize is the replay speaker's own side, above the 48 px floor of
// ENCRE_02 §11: it is tapped by a hesitant CE1 nearly every word, so it is
// drawn larger than an ordinary key rather than at the floor.
const speakerSize = 56

// wagerButtonH is the wager buttons' own height, brief/ENCRE_06 §7's ~86 px.
const wagerButtonH = 86

// wagerGap is the gap around and between the two wager buttons.
const wagerGap = 12

// bottomThirdFraction is Hoober's 2013 boundary (1,333 field observations of
// how a phone is actually held): two thirds of the way down the screen splits
// the top corners a one-handed thumb struggles to reach from the bottom it
// reaches easily.
const bottomThirdFraction = 2.0 / 3.0

// BottomThirdY is the y coordinate that boundary falls at, for this Screen.
func (s Screen) BottomThirdY() int { return int(float64(s.H) * bottomThirdFraction) }

// SpeakerRect is the touch area of the word's replay speaker — encre-cs5.2's
// correction of ENCRE_07 §4bis. The first design put it at the card's
// top-right corner, the hardest zone of the whole screen for a one-handed
// thumb (Hoober, 2013); it is tapped nearly every word, so it moves to the
// keyboard's own left margin, at or below [Screen.BottomThirdY].
func (s Screen) SpeakerRect() (x, y, w, h int) {
	y = max(s.BottomThirdY(), s.BoardY)
	return s.BoardX + wagerGap, y, speakerSize, speakerSize
}

// WagerRects returns the touch areas of the two buttons of the pari of
// ENCRE_06 §7 — "j'écoute 2 fois" (listenTwice) on the left, "1 seule fois"
// (listenOnce) on the right — the same size, side by side across the
// keyboard's own width, at or below [Screen.BottomThirdY]: encre-cs5.2's
// correction, off the stretch zone the first design put them in, tapped
// every word.
func (s Screen) WagerRects() (listenTwice, listenOnce Rect) {
	y := max(s.BottomThirdY(), s.BoardY)
	w := (s.BoardW - 3*wagerGap) / 2
	listenTwice = Rect{X: s.BoardX + wagerGap, Y: y, W: w, H: wagerButtonH}
	listenOnce = Rect{X: listenTwice.X + w + wagerGap, Y: y, W: w, H: wagerButtonH}
	return listenTwice, listenOnce
}
