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
