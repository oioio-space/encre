package ui

// The logical resolutions of brief/ENCRE_04 §2. Every size the game is ever
// drawn at is one of these two, scaled to the window — which is what makes the
// layout hold on a screen nobody designed it for.
const (
	// PortraitWidth is the logical width on a phone or a tablet held upright.
	PortraitWidth = 390
	// PortraitHeight is its logical height.
	PortraitHeight = 844
	// LandscapeWidth is the logical width on a computer, or a tablet on its side.
	LandscapeWidth = 1280
	// LandscapeHeight is its logical height.
	LandscapeHeight = 720
)

// The card is drawn at 96 by 128 and shown at a whole multiple of it. ENCRE_02
// §15 forbids fractional scales, and ENCRE_06 §5 notes that its own 180×240 is
// 1.875 times the art — which resamples every pixel of it off the grid.
const (
	cardArtW = 96
	cardArtH = 128
)

// keyAspect is the tallest an ordinary letter key may be against its width.
// Past it a keyboard reads as a grid of columns rather than of keys, and the
// space between the rows reads as a mistake.
const keyAspect = 1.3

// Screen is where each band of the run screen sits, for one window size.
//
// It lives here rather than in the client's loop because it is arithmetic, not
// drawing: a browser is resized at will, a phone rotates in a pocket and a
// tablet is held either way, so the layout has to hold at sizes nobody drew it
// for. Being a function of two integers, it can be swept by a test on every
// build instead of being checked by eye at the two sizes someone remembered.
type Screen struct {
	// W and H are the logical resolution the game draws in.
	W, H int
	// Layout is the keyboard arrangement this shape calls for.
	Layout Layout
	// CardX and CardY are the card's top-left corner, CardW and CardH its size —
	// always a whole multiple of the art it is drawn from.
	CardX, CardY, CardW, CardH int
	// CardBandH is the height set aside for the card, which the card itself
	// nearly fills.
	CardBandH int
	// EntryY is the middle of the line where the word is written, and
	// WordBandH the height that line is given.
	EntryY, WordBandH int
	// BoardX, BoardY, BoardW and BoardH are the keyboard's own area.
	BoardX, BoardY, BoardW, BoardH int
}

// NewScreen lays out the run screen for a window of outsideW by outsideH
// device-independent pixels.
//
// The orientation picks the resolution and with it the keyboard: seven
// alphabetical columns where the width is short, AZERTY where it is not
// (ENCRE_06 §2). Every band is then sized by what it holds rather than by a
// fixed fraction — the card by the largest whole multiple of its art that fits,
// the word by the text in it, the keyboard by its own rows. Bands sized by
// fractions alone left the middle of the screen empty, which reads as a mistake
// rather than as breathing room.
func NewScreen(outsideW, outsideH int) Screen {
	s := Screen{W: LandscapeWidth, H: LandscapeHeight, Layout: AZERTY}
	if outsideH >= outsideW {
		s = Screen{W: PortraitWidth, H: PortraitHeight, Layout: Phone}
	}

	// The keyboard takes only the height its rows need. Given the width it has
	// and the columns of its layout, a row is as tall as a key may be — so the
	// keys tile their area instead of floating in it.
	s.BoardW = s.W
	if s.Layout == AZERTY {
		s.BoardW = s.W * 55 / 100
	}
	// A row is as tall as a letter key may be, so the keys tile their area
	// instead of floating in it. The width comes from the layout's own column
	// count, not from counting a row's entries — the last row holds two special
	// keys and would give a misleading answer.
	rowH := int(float64(s.BoardW/LetterColumns(s.Layout)) * keyAspect)
	s.BoardH = min(len(rows(s.Layout))*rowH, s.H*55/100)
	s.BoardX = s.W - s.BoardW
	// A phone keyboard belongs at the bottom, under the thumb. On a computer
	// there is no thumb and nothing below it, so it is centred instead — which
	// puts it on the same centre line as the card beside it, and stops the two
	// columns from drifting apart.
	s.BoardY = s.H - s.BoardH
	if s.Layout == AZERTY {
		s.BoardY = (s.H - s.BoardH) / 2
	}

	// What is left above the keyboard — the whole width in portrait, the left
	// column in landscape — holds the card and the word beneath it.
	colW, colH := s.W, s.H-s.BoardH
	if s.Layout == AZERTY {
		colW, colH = s.W-s.BoardW, s.H
	}
	header := colH * 12 / 100
	s.WordBandH = min(colH*16/100, 3*48)

	// The largest whole multiple of the card's art that leaves its band a
	// tenth of itself as margin.
	available := colH - header - s.WordBandH
	scale := max(1, min((colW*80/100)/cardArtW, (available*88/100)/cardArtH))
	s.CardW, s.CardH = cardArtW*scale, cardArtH*scale
	// The card and the word beneath it are ONE block, centred in what the
	// keyboard leaves. A card pinned under the header and a word pinned above
	// the keyboard put all the slack in one place, which reads as a hole; split
	// evenly above and below, the same pixels read as margin.
	s.CardBandH = s.CardH
	block := s.CardH + s.WordBandH
	top := header + max(0, (colH-header-block)/2)
	s.CardY = top
	s.CardX = (colW - s.CardW) / 2
	s.EntryY = top + s.CardH + s.WordBandH/2

	return s
}
