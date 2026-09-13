package ui

// usableFraction is how much of the monitor a window may occupy. The rest is
// left to the title bar and whatever panel the desktop keeps on screen: a
// window sized to the full height loses its own title bar behind them, and on
// some desktops cannot be dragged back out.
const usableFraction = 0.9

// WindowScale returns the largest whole number of screen pixels per logical
// pixel that still fits on the monitor.
//
// It is whole on purpose (ENCRE_04 §2). A fractional scale resamples every
// glyph and every sprite off the pixel grid, which is what turns pixel art into
// mush; at ×2 or ×3 each logical pixel stays a clean square of screen pixels.
// It never returns less than 1, including for the 0×0 monitor Ebitengine
// reports before the window exists.
func WindowScale(logicalW, logicalH, monitorW, monitorH int) int {
	if logicalW <= 0 || logicalH <= 0 {
		return 1
	}
	scale := min(
		int(float64(monitorW)*usableFraction)/logicalW,
		int(float64(monitorH)*usableFraction)/logicalH,
	)
	return max(scale, 1)
}

// PhysicalPx converts a length in logical pixels to physical device pixels.
//
// Ebitengine stretches the screenLen logical pixels the game draws across the
// outsideLen device-independent pixels the window occupies, and the device then
// renders each of those with deviceScale physical pixels. Both steps count.
//
// This is a reporting figure, not the touch-target test. The 48 pixels of
// ENCRE_02 §11 are density-independent units, so a key is measured against them
// in the logical pixels of the 390-wide screen; scaling by the device factor
// first makes every modern phone pass and measures nothing.
//
// It returns 0 when screenLen is 0, which is what Ebitengine passes before the
// window exists.
func PhysicalPx(logical, screenLen, outsideLen int, deviceScale float64) float64 {
	if screenLen == 0 {
		return 0
	}
	return float64(logical) * float64(outsideLen) / float64(screenLen) * deviceScale
}
