package ui

// TrackingRatio is how much extra space encre-tfy.1 opens between letters, as
// a fraction of the face's own size. Zorzi, Barbiero, Facoetti et al. (PNAS
// 2012, DOI 10.1073/pnas.1209921109; replicated in Annals of Dyslexia, 2020)
// measured +2.5 pt on a 14 pt body — about +18% — as what doubles a child's
// reading accuracy and gains over 20% of reading speed, immediately and
// without training. It sits at the middle of brief/ENCRE_07 §4bis's +15% to
// +20% window: the word being typed and the letters of every key both carry
// it (brief/ENCRE_06 §4, §5).
const TrackingRatio = 0.175

// Tracked lays s out letter by letter with [TrackingRatio] of size opened
// between each pair, given advance(r): the face's own natural width of r in
// logical pixels. It returns the left edge of every rune's own advance and
// the width of the whole string, letters and the gaps between them, but with
// no tracking trailing the last letter — trailing space would throw off
// anything centred on this width, and there is no letter after it to space
// away from.
//
// This takes advance as a function rather than a [text.Face] so the layout
// arithmetic is tested on its own, without a face or a graphics context: any
// later caller measuring with a real font hands its own Advance-shaped
// closure through.
func Tracked(s string, size float64, advance func(r rune) float64) (positions []float64, width float64) {
	extra := size * TrackingRatio
	x := 0.0
	for _, r := range s {
		positions = append(positions, x)
		x += advance(r) + extra
	}
	if len(positions) > 0 {
		x -= extra
	}
	return positions, x
}
