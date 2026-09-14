package api

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"slices"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

// resultCardWidth and resultCardHeight size the card GET
// /child/result-card/{runID}.png draws.
const (
	resultCardWidth  = 480
	resultCardHeight = 320
)

// parchment is the card's background (ENCRE_02 §3, "Parchemin").
var parchment = color.RGBA{0xEA, 0xD9, 0xB4, 0xFF}

// resultBandWon and resultBandLost are the banner colours across the top of
// the card: gold (ENCRE_02 §3, "Or") for a won run, sang (§3, "Sang") for a
// lost one — never a colour that also names a Couleur, so the banner cannot
// be mistaken for one.
var (
	resultBandWon  = color.RGBA{0xF2, 0xC1, 0x4E, 0xFF}
	resultBandLost = color.RGBA{0xB3, 0x20, 0x2A, 0xFF}
)

// talismanDot is the colour of one dot in the talisman row: encre profonde
// (ENCRE_02 §3), a neutral that reads against the parchment without
// borrowing a Couleur's own colour.
var talismanDot = color.RGBA{0x24, 0x23, 0x42, 0xFF}

// couleurTeinte is the six Couleurs' teinte (ENCRE_02 §3), in [engine.Colors] order.
var couleurTeinte = map[engine.Color]color.RGBA{
	engine.Muettes:    {0x9F, 0xC4, 0xE8, 0xFF},
	engine.Jumelles:   {0xB0, 0x7C, 0xE0, 0xFF},
	engine.Accentuees: {0xD9, 0x52, 0x5C, 0xFF},
	engine.Masquees:   {0x6F, 0xB5, 0x7A, 0xFF},
	engine.Sosies:     {0x5B, 0xC8, 0xC4, 0xFF},
	engine.Accordees:  {0xE8, 0xA9, 0x4C, 0xFF},
}

// resultCardMaxLevel is the level a fiole shows completely full at
// (engine/rank.go's own level cap).
const resultCardMaxLevel = 10

const (
	bandHeight  = 40
	fioleWidth  = 48
	fioleGap    = 16
	fioleTop    = bandHeight + 24
	fioleHeight = 180
	fioleMargin = 24
	dotSize     = 10
	dotGap      = 8
	dotsTop     = fioleTop + fioleHeight + 24
)

// buildResultCard draws run's result card: a band announcing win or loss, a
// fiole per Couleur filled to the child's level in it, and one dot per
// Talisman the run carried — everything drawn with rectangles and
// image/png, no font and no dependency, matching ENCRE_02 §3's rule that a
// Couleur is a fill, never a gradient or a label.
func buildResultCard(run *store.Run, levels map[engine.Color]int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, resultCardWidth, resultCardHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(parchment), image.Point{}, draw.Src)

	band := resultBandLost
	if run.FailedAt < 0 {
		band = resultBandWon
	}
	fillRect(img, 0, 0, resultCardWidth, bandHeight, band)

	for i, c := range engine.Colors() {
		x := fioleMargin + i*(fioleWidth+fioleGap)
		level := min(max(levels[c], 0), resultCardMaxLevel)
		filled := fioleHeight * level / resultCardMaxLevel
		fillRect(img, x, fioleTop+(fioleHeight-filled), x+fioleWidth, fioleTop+fioleHeight, couleurTeinte[c])
	}

	talismans := slices.Clone(run.Talismans)
	slices.Sort(talismans)
	for i := range talismans {
		x := fioleMargin + i*(dotSize+dotGap)
		if x+dotSize > resultCardWidth {
			break
		}
		fillRect(img, x, dotsTop, x+dotSize, dotsTop+dotSize, talismanDot)
	}

	return img
}

// fillRect fills [x0,x1) × [y0,y1) with c, clipped to img's bounds.
func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	b := img.Bounds()
	x0, y0 = max(x0, b.Min.X), max(y0, b.Min.Y)
	x1, y1 = min(x1, b.Max.X), min(y1, b.Max.Y)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// handleChildResultCard draws and serves the result card for one of the
// child's own runs (ENCRE_04 §7). See [resultCardRunID]'s doc comment for
// why the route captures "{runID}.png" as one path value rather than a bare
// {runID}.
func (s *Server) handleChildResultCard(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	run, ok := s.ownRunByID(w, r, sess, resultCardRunID(r))
	if !ok {
		return
	}
	child, err := s.db.ChildByID(r.Context(), run.ChildID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}

	img := buildResultCard(run, child.Engine().Level)
	w.Header().Set("Content-Type", "image/png")
	// A run's own card never changes once played, so the browser can hold
	// on to it — but it is one child's data, never a shared static asset,
	// so this stays private rather than a CDN-cacheable public max-age.
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	// The header is already written by the time png.Encode could fail; see
	// writeJSON's own comment on the same tradeoff.
	_ = png.Encode(w, img)
}
