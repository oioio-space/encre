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

// The portrait phone's five bands, fixed pixel heights from brief/ENCRE_06
// §4's table, corrected by ENCRE_07 §4.1: the card grows from a fractional
// ×1.875 (180×240, forbidden by ENCRE_02 §15) to a whole ×2 (192×256), and the
// keyboard band gives back the 16 px the card gained, from 334 to 318. They
// sum to exactly PortraitHeight (844), which is why the keyboard's is
// subtracted rather than derived from its own rows: a row-based height cannot
// land on 318, since it is not a multiple of the keyboard's five rows.
//
// cardBandMargin is the slack ENCRE_06 §4's own numbers leave around the card
// inside its band — 268-240 before the correction, 284-256 after it — kept as
// the rule rather than the two numbers it happens to produce, so a future
// change to the art's own size still lands the card centred with the same
// breathing room.
const (
	phoneHeaderH    = 104
	phoneTargetBarH = 12
	phoneCardBandH  = 284
	phoneWordBandH  = 126
	phoneBoardH     = 318
	cardBandMargin  = 28
)

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
	if outsideH >= outsideW {
		return newPhoneScreen()
	}
	return newAZERTYScreen()
}

// newPhoneScreen lays out the portrait phone at the fixed logical resolution
// of PortraitWidth by PortraitHeight, in the five bands of brief/ENCRE_06 §4
// as corrected by ENCRE_07 §4.1 (see the phone… constants): a header, a
// target bar, the card and the word beneath it as one centred block, and the
// keyboard taking whatever height the other four leave.
func newPhoneScreen() Screen {
	s := Screen{W: PortraitWidth, H: PortraitHeight, Layout: Phone}

	s.BoardW, s.BoardH = s.W, phoneBoardH
	s.BoardX, s.BoardY = 0, s.H-s.BoardH

	s.WordBandH = phoneWordBandH
	s.CardBandH = phoneCardBandH
	// The largest whole multiple of the card's art that leaves cardBandMargin
	// of slack around it — the margin ENCRE_06 §4's own band and card sizes
	// imply, kept as a rule so it survives the art's own size changing.
	scale := 1
	for cand := (phoneCardBandH - cardBandMargin) / cardArtH; cand >= 1; cand-- {
		if cardArtW*cand <= s.W*80/100 {
			scale = cand
			break
		}
	}
	s.CardW, s.CardH = cardArtW*scale, cardArtH*scale

	top := phoneHeaderH + phoneTargetBarH
	s.CardX = (s.W - s.CardW) / 2
	s.CardY = top + (phoneCardBandH-s.CardH)/2
	s.EntryY = top + phoneCardBandH + s.WordBandH/2

	return s
}

// newAZERTYScreen lays out the tablet and the computer, at the fixed logical
// resolution of LandscapeWidth by LandscapeHeight: the AZERTY keyboard in a
// column of its own, and the card and the word beside it, sized by what each
// holds rather than by a fixed band — brief/ENCRE_06 §4's fixed table is
// telephone-only.
func newAZERTYScreen() Screen {
	s := Screen{W: LandscapeWidth, H: LandscapeHeight, Layout: AZERTY}

	// The keyboard takes only the height its rows need. Given the width it has
	// and the columns of its layout, a row is as tall as a key may be — so the
	// keys tile their area instead of floating in it.
	s.BoardW = s.W * 55 / 100
	rowH := int(float64(s.BoardW/LetterColumns(s.Layout)) * keyAspect)
	s.BoardH = min(len(rows(s.Layout))*rowH, s.H*55/100)
	s.BoardX = s.W - s.BoardW
	// There is no thumb to put the keyboard under and nothing below it, so it
	// is centred instead — which puts it on the same centre line as the card
	// beside it, and stops the two columns from drifting apart.
	s.BoardY = (s.H - s.BoardH) / 2

	// What is left beside the keyboard holds the card and the word beneath it.
	colW, colH := s.W-s.BoardW, s.H
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
