// Command client is the ENCRE game client: an Ebitengine v2 program built both
// natively (linux/darwin/windows) and for the browser with GOOS=js GOARCH=wasm.
package main

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

// Logical resolutions from brief/ENCRE_04_spec_technique.md §2. The layout
// follows the window's orientation instead of forcing a rotation, so the client
// chooses between the two on every Layout call.
const (
	portraitWidth, portraitHeight   = 390, 844
	landscapeWidth, landscapeHeight = 1280, 720
)

// nuit is the darkest ink of the palette (#0B0A14, brief/ENCRE_02 §Encres). It
// is the canvas the Kage ink shader will later be drawn onto.
var nuit = color.RGBA{R: 0x0B, G: 0x0A, B: 0x14, A: 0xFF}

// game is the root scene. It carries no state yet: this is the window skeleton
// that the scene graph of brief/ENCRE_04 §3 client/game will grow from.
type game struct{}

// Update advances the simulation by one tick; Ebitengine calls it at 60 TPS.
func (g *game) Update() error { return nil }

// Draw renders one frame onto screen, which Ebitengine has already sized to the
// logical resolution Layout returned.
func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(nuit)
}

// Layout reports the logical resolution for the current window: portrait when
// the window is at least as tall as it is wide, landscape otherwise. Ebitengine
// scales that result to the device, so the game always draws in these units.
func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideHeight >= outsideWidth {
		return portraitWidth, portraitHeight
	}
	return landscapeWidth, landscapeHeight
}

func main() {
	ebiten.SetWindowTitle("ENCRE")
	ebiten.SetWindowSize(portraitWidth, portraitHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(&game{}); err != nil {
		log.Fatal(err)
	}
}
