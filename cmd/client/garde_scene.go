package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/ui"
)

// gardeScene is the minimal étui-de-cuir screen of brief/ENCRE_02 §10 between
// Boot and a run: just enough to give [game.Director] somewhere real to
// leave from before Run, and a single sceau *Jouer* to tap — the Atelier
// itself, the three-card fan and the rank are a later ticket's, not this
// drawing one's.
type gardeScene struct {
	c *client
}

// Update implements game.Scene: a tap on the sceau of playButtonRect starts
// the demo manche ([client.startManche]) and replaces Garde with Run on the
// stack — the run's own sequence never steps back to it (see
// [game.Director.Replace]'s own doc).
func (g *gardeScene) Update(gm *game.Game) error {
	x, y, _, justDown, _ := g.c.pointer()
	if !justDown {
		return nil
	}
	sx, sy, sw, sh := g.playButtonRect()
	if x >= sx && x < sx+sw && y >= sy && y < sy+sh {
		g.c.startManche()
		return gm.Replace(game.Run)
	}
	return nil
}

// playButtonRect is the touch area of the single sceau *Jouer*
// (ENCRE_02 §14: "un seul bouton : un sceau Jouer"), centred low enough for
// a one-handed thumb (Hoober 2013, the same reasoning [ui.Screen.
// BottomThirdY] already carries for the run screen's own speaker and wager
// buttons).
func (g *gardeScene) playButtonRect() (x, y, w, h int) {
	side := 96
	return g.c.screen.W/2 - side/2, g.c.screen.H*2/3 - side/2, side, side
}

// Draw implements game.Scene.
func (g *gardeScene) Draw(dst *ebiten.Image, _ ui.Screen) {
	dst.Fill(parchemin)
	c := g.c
	cx := float64(c.screen.W) / 2

	drawFace(dst, "l'atelier", c.faces.plumeWord, ui.DefaultPlumeSize, cx, float64(c.screen.H)/4, cuir)
	drawFace(dst, "trois cartes t'attendent dans la garde", c.faces.greffeLabel, greffeLabelSize,
		cx, float64(c.screen.H)/4+40, cuir)

	sx, sy, sw, sh := g.playButtonRect()
	drawSeal(dst, float32(sx)+float32(sw)/2, float32(sy)+float32(sh)/2, float32(sw)/2)
	drawFace(dst, "jouer", c.faces.greffeLabel, greffeLabelSize, cx, float64(sy+sh)+24, brique)
}
