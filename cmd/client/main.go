// Command client is the ENCRE game client: an Ebitengine v2 program built both
// natively (linux/darwin/windows) and for the browser with GOOS=js GOARCH=wasm.
//
// It currently holds the Étape 0 prototype of brief/ENCRE_05 ticket T00: the
// drawn keyboard, a card with a word to copy, and the word as it is typed. The
// question it exists to answer is not whether the code works but whether a
// seven-year-old can type "cœur" on a real tablet without fighting the
// interface — so it measures its own touch targets and shows the number rather
// than asserting they are big enough.
//
// Everything here is composition: the geometry lives in client/ui and the typed
// word in client/game, both tested. When the scene graph of T23 arrives, the
// loop moves to client/game with tests of its own.
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/assets"
	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/ui"
)

// The light theme of ENCRE_06 §2 and §3, which overrides the night background
// of ENCRE_02: a phone held at arm's length in daylight, read by a seven-year-
// old, needs parchment and not ink.
var (
	parchemin      = color.RGBA{R: 0xEA, G: 0xD9, B: 0xB4, A: 0xFF} // fond d'écran
	parcheminClair = color.RGBA{R: 0xFB, G: 0xF3, B: 0xDE, A: 0xFF} // surfaces, touches
	parcheminVieux = color.RGBA{R: 0xCD, G: 0xB4, B: 0x8A, A: 0xFF} // zone clavier, bordures
	cuirClair      = color.RGBA{R: 0xA8, G: 0x8C, B: 0x63, A: 0xFF} // ourlet de touche
	cuir           = color.RGBA{R: 0x6E, G: 0x57, B: 0x38, A: 0xFF} // texte secondaire
	encre          = color.RGBA{R: 0x24, G: 0x23, B: 0x42, A: 0xFF} // texte principal
	brique         = color.RGBA{R: 0x8F, G: 0x2F, B: 0x1E, A: 0xFF} // texte chaud
	braise         = color.RGBA{R: 0xF5, G: 0xA7, B: 0x42, A: 0xFF} // formes chaudes
	flamme         = color.RGBA{R: 0xFF, G: 0xE0, B: 0x8A, A: 0xFF} // liseré des arêtes tournées vers la bougie
	ambreBrule     = color.RGBA{R: 0xD9, G: 0x62, B: 0x2B, A: 0xFF} // lumière du sceau
	sangSeche      = color.RGBA{R: 0x5A, G: 0x0F, B: 0x14, A: 0xFF} // creux du sceau
)

// Text comes from one 12-pixel bitmap face enlarged by whole factors, never
// re-rasterised at another size: any other factor lands the glyph off the pixel
// grid (ENCRE_04 §2). That quantises the type scale of ENCRE_06 to 12, 24, 36
// and 48 — its 13, 14, 22, 26 and 30 px steps assume a scalable font.
const (
	textLabel = 1 // 12 px — labels and diagnostics
	textKey   = 2 // 24 px — the letters on the keys (ENCRE_06 asks 26)
	textWord  = 4 // 48 px — the word, the biggest character on screen (§4)
)

// Words carrying the accents ENCRE_03 §4 teaches at CE1, taken from its own
// examples, plus the two the always-visible row cannot reach: cœur and flûte
// are typed by holding o and u.
var words = []string{
	"école", "bébé", "mère", "forêt", "garçon",
	"français", "tête", "cœur", "flûte", "hôpital",
}

const (
	// The light of ENCRE_02 §4: a candle at the top left. Shadows fall to the
	// bottom right, one pixel; the edges turned towards the candle carry a rim
	// of Flamme; nothing is lit from underneath. It is the only relief the
	// charter allows — gradients are forbidden — so losing it flattens
	// everything into the paper.
	cornerRadius = 6 // ENCRE_06 §4
	shadowDepth  = 1
	rimWidth     = 1

	keyInset  = 2                      // the drawn key sits inside its touch cell
	hemHeight = 3                      // ourlet bas (ENCRE_06 §4)
	pressSink = 2                      // translateY on press (ENCRE_06 §4)
	holdDelay = 350 * time.Millisecond // before the accents of a key open
)

type client struct {
	kb    *ui.Keyboard
	entry *game.Entry
	face  *text.GoXFace

	// juice is the timings of brief/ENCRE_02 §12, loaded once at startup and
	// handed to the game.Game this scene runs under (see main). This
	// prototype does not yet animate anything they govern, but main.go wires
	// them through the same game.Game every later scene will read them from.
	juice anim.Juice

	// screen is the layout of the current frame, computed by Layout and
	// handed to Draw by the game.Stage that runs this scene.
	screen ui.Screen

	plume *audio.Player
	// audioUnlocked records the browser's one-gesture rule: the audio context
	// does not start until the player has touched something (ENCRE_04 §2).
	audioUnlocked bool

	word int

	// Geometry, recomputed on every Layout because the browser can rotate or
	// resize the canvas at any moment.
	screenW, screenH                   int
	outsideW                           int
	boardX, boardY                     int
	boardW, boardH                     int
	cardX, cardY, cardW, cardH, entryY int
	// cellW is the width of an ordinary letter key. The two special keys are
	// wider, so capping each key's height against its own width would make them
	// tower over the letters; the whole row is capped against this instead.
	cellW int

	// held is the key under the finger. When it has accents and the finger
	// stays past holdDelay, they open above it and the release picks one.
	held      ui.Key
	holding   bool
	heldSince time.Time
	pointerX  int
	pointerY  int
	echoKey   ui.Key
	echoTicks int
}

func newClient() (*client, error) {
	// La Plume, Le Greffe and La Cursive are the real fonts ticket encre-amh
	// chose (ENCRE_02 §5), checked here against every rune the drawn keyboard
	// can produce rather than assumed. The scene still draws through the
	// bitmap placeholder below (c.face): wiring this registry's faces into
	// Draw is a later ticket's redraw, not this one's.
	required := ui.RequiredRunes(ui.Phone) + ui.RequiredRunes(ui.AZERTY) + "…·×→"
	if _, err := ui.NewDefaultRegistry(required); err != nil {
		return nil, fmt.Errorf("loading fonts: %w", err)
	}

	juice, err := anim.LoadJuice(assets.JuiceJSON)
	if err != nil {
		return nil, fmt.Errorf("loading juice.json: %w", err)
	}

	c := &client{face: text.NewGoXFace(bitmapfont.Face), juice: juice}

	ctx := audio.NewContext(48000)
	stream, err := vorbis.DecodeF32(bytes.NewReader(assets.PlumeOgg))
	if err != nil {
		return nil, fmt.Errorf("decoding plume.ogg: %w", err)
	}
	if c.plume, err = ctx.NewPlayerF32(stream); err != nil {
		return nil, fmt.Errorf("preparing the plume sound: %w", err)
	}
	return c, nil
}

// Layout reports the logical resolution for the current window and lays out
// the bands inside it. The arithmetic lives in client/ui, where a test sweeps
// it across every size a browser or a pocket can produce.
func (c *client) Layout(outsideWidth, outsideHeight int) (int, int) {
	c.outsideW = outsideWidth
	s := ui.NewScreen(outsideWidth, outsideHeight)
	c.screen = s
	c.screenW, c.screenH = s.W, s.H
	c.cardX, c.cardY = s.CardX, s.CardY
	c.cardW, c.cardH, c.entryY = s.CardW, s.CardH, s.EntryY
	c.boardX, c.boardY, c.boardW, c.boardH = s.BoardX, s.BoardY, s.BoardW, s.BoardH

	c.kb = ui.NewKeyboard(s.Layout, c.boardW, c.boardH)
	c.cellW = 0
	for _, k := range c.kb.Keys() {
		if k.Kind == ui.KeyRune && (c.cellW == 0 || k.W < c.cellW) {
			c.cellW = k.W
		}
	}
	if c.entry == nil {
		c.entry = game.NewEntry(c.kb)
	}
	return c.screenW, c.screenH
}

// Update implements game.Scene. g is unused: this single-scene prototype
// never transitions and reads nothing from g.Juice yet, but the signature is
// what every later scene will share.
func (c *client) Update(_ *game.Game) error {
	if c.echoTicks > 0 {
		c.echoTicks--
	}

	// One pointer, whether it is a finger or a mouse.
	x, y, down, justDown, justUp := c.pointer()
	c.pointerX, c.pointerY = x, y

	switch {
	case justDown:
		if k, ok := c.kb.KeyAt(x-c.boardX, y-c.boardY); ok {
			c.held, c.holding, c.heldSince = k, true, time.Now()
		}
	case justUp && c.holding:
		c.release(x, y)
	case !down && c.holding:
		c.release(x, y)
	}

	// The physical keyboard of the computer. Accented characters arrive here
	// and nowhere else (ENCRE_04 §2).
	for _, r := range ebiten.AppendInputChars(nil) {
		if c.entry.Type(r) {
			c.echo(r)
			c.playPlume()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		c.entry.Erase()
		c.playPlume()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		c.validate()
	}
	return nil
}

// pointer folds touch and mouse into one, because the prototype has to behave
// the same on a tablet and on the machine it is developed on.
func (c *client) pointer() (x, y int, down, justDown, justUp bool) {
	if ids := inpututil.AppendJustPressedTouchIDs(nil); len(ids) > 0 {
		x, y = ebiten.TouchPosition(ids[0])
		return x, y, true, true, false
	}
	if ids := ebiten.AppendTouchIDs(nil); len(ids) > 0 {
		x, y = ebiten.TouchPosition(ids[0])
		return x, y, true, false, false
	}
	x, y = ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return x, y, true, true, false
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		return x, y, false, false, true
	}
	return x, y, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), false, false
}

// release ends a press: on an open accent row it takes the accent under the
// finger, otherwise it does what the key says.
func (c *client) release(x, y int) {
	defer func() { c.holding = false }()

	if vs := c.openVariants(); len(vs) > 0 {
		if r, ok := c.variantAt(x, y); ok {
			c.entry.Type(r)
			c.playPlume()
			return
		}
	}
	switch c.held.Kind {
	case ui.KeyErase:
		c.entry.Erase()
	case ui.KeyValidate:
		c.validate()
	case ui.KeyRune:
		c.entry.Type(c.held.Rune)
	}
	c.playPlume()
}

// openVariants returns the accents currently shown above the held key, if the
// finger has stayed long enough for them to open.
func (c *client) openVariants() []rune {
	if !c.holding || c.held.Kind != ui.KeyRune || time.Since(c.heldSince) < holdDelay {
		return nil
	}
	return ui.Variants(c.held.Rune)
}

// variantRect is where the nth open accent is drawn, in screen coordinates.
func (c *client) variantRect(n int) (x, y, w, h float64) {
	k := c.held
	w, h = float64(k.W), float64(k.H)
	x = float64(c.boardX+k.X) + float64(n)*w
	y = float64(c.boardY+k.Y) - h - 4
	// Keep the row on screen when the held key is near the right edge.
	if over := x + w - float64(c.screenW); over > 0 {
		x -= over
	}
	return x, y, w, h
}

func (c *client) variantAt(x, y int) (rune, bool) {
	for n, r := range c.openVariants() {
		vx, vy, vw, vh := c.variantRect(n)
		if float64(x) >= vx && float64(x) < vx+vw && float64(y) >= vy && float64(y) < vy+vh {
			return r, true
		}
	}
	return 0, false
}

// echo lights the drawn key matching a character typed on the physical
// keyboard, so the computer and the tablet show the same thing (ENCRE_02 §11).
func (c *client) echo(r rune) {
	for _, k := range c.kb.Keys() {
		if k.Kind == ui.KeyRune && k.Rune == r {
			c.echoKey, c.echoTicks = k, 6
			return
		}
	}
}

func (c *client) validate() {
	c.word = (c.word + 1) % len(words)
	c.entry.Clear()
}

// playPlume restarts the scratch. The first call is also what unlocks the
// browser's audio context, which is why it hangs off the first tap.
func (c *client) playPlume() {
	if c.plume == nil {
		return
	}
	c.audioUnlocked = true
	if err := c.plume.Rewind(); err != nil {
		return
	}
	c.plume.Play()
}

// Draw implements game.Scene. screen is unused: this prototype still lays out
// from the fields Layout copied out of it, not to change a pixel of what T00
// validated.
func (c *client) Draw(dst *ebiten.Image, _ ui.Screen) {
	dst.Fill(parchemin)
	c.drawDiagnostics(dst)
	c.drawCard(dst)
	c.drawEntry(dst)
	c.drawKeyboard(dst)
	c.drawOpenVariants(dst)
}

// drawText draws s centred on (cx, cy), enlarged by the whole factor scale. The
// enlargement is a nearest-neighbour blit of the 12-pixel glyphs, not a
// re-rasterisation: that is what keeps the edges square instead of grey.
func (c *client) drawText(dst *ebiten.Image, s string, cx, cy float64, scale int, col color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(cx, cy)
	op.Filter = ebiten.FilterNearest
	op.ColorScale.ScaleWithColor(col)
	op.PrimaryAlign, op.SecondaryAlign = text.AlignCenter, text.AlignCenter
	text.Draw(dst, s, c.face, op)
}

// roundedRect fills a rectangle with the 6-pixel corner ENCRE_06 §4 gives every
// surface of the game. Sharp corners are what made the keys read as a grid
// rather than as things to press.
func roundedRect(dst *ebiten.Image, x, y, w, h, r float32, clr color.Color) {
	r = min(r, w/2, h/2)
	var p vector.Path
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.ArcTo(x+w, y, x+w, y+r, r)
	p.LineTo(x+w, y+h-r)
	p.ArcTo(x+w, y+h, x+w-r, y+h, r)
	p.LineTo(x+r, y+h)
	p.ArcTo(x, y+h, x, y+h-r, r)
	p.LineTo(x, y+r)
	p.ArcTo(x, y, x+r, y, r)
	p.Close()

	op := &vector.DrawPathOptions{}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &p, &vector.FillOptions{}, op)
}

// raised draws a surface standing off the page: its shadow cast down and right,
// a body of Cuir clair, the face sitting on it with a hem of that body showing
// along the bottom, and the rim of Flamme on the two edges turned towards the
// candle (ENCRE_02 §4).
//
// Pass hem=0 and rim=false to sink it. A thing pressed has moved away from the
// light, and the charter forbids lighting anything from below — so a pressed
// key loses its rim rather than gaining a darker one.
func raised(dst *ebiten.Image, x, y, w, h, hem float32, face color.Color, rim bool) {
	if hem > 0 {
		roundedRect(dst, x+shadowDepth, y+shadowDepth, w, h, cornerRadius, cuir)
	}
	roundedRect(dst, x, y, w, h, cornerRadius, cuirClair)
	roundedRect(dst, x, y, w, h-hem, cornerRadius, parcheminVieux)
	roundedRect(dst, x+rimWidth, y+rimWidth, w-2*rimWidth, h-hem-2*rimWidth, cornerRadius, face)
	if !rim {
		return
	}
	// One pixel, along the top and the left only. An even outline would read as
	// a border rather than as light falling on one side, and anything thicker
	// reads as a yellow line drawn round the key.
	roundedRect(dst, x+rimWidth, y+rimWidth, w-2*rimWidth, rimWidth, 0, flamme)
	roundedRect(dst, x+rimWidth, y+rimWidth, rimWidth, h-hem-2*rimWidth, 0, flamme)
}

// drawEraser draws the gomme of ENCRE_06 §4: a block leaning back, with the
// darker band a school eraser has round its middle.
//
// The lean is drawn as two stacked slices rather than by rotating anything —
// rotation resamples the pixels off the grid, which is the one thing the pixel
// look cannot survive (ENCRE_02 §15).
func drawEraser(dst *ebiten.Image, cx, cy, w float32) {
	h := w * 0.58
	half := w / 2
	for i := range 2 {
		sx := cx - half + float32(i)*half
		sy := cy - h/2 - float32(1-i)*h*0.16
		roundedRect(dst, sx+shadowDepth, sy+shadowDepth, half+1, h, 2, cuir)
		roundedRect(dst, sx, sy, half+1, h, 2, parchemin)
		roundedRect(dst, sx, sy+h*0.58, half+1, h*0.42, 2, cuirClair)
	}
}

// drawSeal draws the wax seal of ENCRE_06 §4, in the three tones ENCRE_06 §3
// gives it: Brique for the wax, Sang séché for the hollow, Ambre brûlé for the
// light on the side facing the candle.
func drawSeal(dst *ebiten.Image, cx, cy, r float32) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	circle := func(x, y, rad float32, clr color.Color) {
		var p vector.Path
		p.Arc(x, y, rad, 0, 2*math.Pi, vector.Clockwise)
		p.Close()
		o := *op
		o.ColorScale.ScaleWithColor(clr)
		vector.FillPath(dst, &p, &vector.FillOptions{}, &o)
	}
	circle(cx+shadowDepth, cy+shadowDepth, r, cuir)
	circle(cx, cy, r, brique)
	circle(cx-r*0.28, cy-r*0.28, r*0.34, ambreBrule)
	circle(cx+r*0.10, cy+r*0.10, r*0.42, sangSeche)
}

// drawDiagnostics reports the measurements ticket T00 asks for, on screen
// because the device to be measured is a phone in someone's hand with no
// console to read.
func (c *client) drawDiagnostics(screen *ebiten.Image) {
	side := c.kb.MinTouchSide()
	letter := c.letterSide()
	dsf := ebiten.Monitor().DeviceScaleFactor()

	// The 48 of ENCRE_02 §11 is a figure in the logical pixels of this screen —
	// the density-independent units a touch guideline is written in. Comparing
	// it against physical device pixels, as a first version of this readout
	// did, makes every modern phone pass and measures nothing.
	col, verdict := brique, "TROP PETIT"
	if letter >= 48 {
		col, verdict = cuir, "OK"
	}
	audioState := "audio en attente"
	if c.audioUnlocked {
		audioState = "audio ok"
	}

	lines := []struct {
		s   string
		col color.Color
	}{
		{fmt.Sprintf("lettre %d px · accent %d px · min 48 → %s", letter, side, verdict), col},
		{fmt.Sprintf("%d×%d · %d dip · ×%.2f · %.0f px phys.", c.screenW, c.screenH, c.outsideW, dsf, ui.PhysicalPx(letter, c.screenW, c.outsideW, dsf)), cuir},
		{fmt.Sprintf("%.0f img/s · %s · appui long : a c e i o u", ebiten.ActualFPS(), audioState), cuir},
	}
	cx := float64(c.cardX) + float64(c.cardW)/2
	for i, l := range lines {
		c.drawText(screen, l.s, cx, float64(18+i*16), textLabel, l.col)
	}
}

// letterSide is the shortest side of an ordinary letter key — the number the
// 48-pixel rule is really about, since the accent row is narrower by design.
func (c *client) letterSide() int {
	side := 0
	for _, k := range c.kb.Keys() {
		if k.Kind != ui.KeyRune || k.Rune < 'a' || k.Rune > 'z' {
			continue
		}
		if s := min(k.W, k.H); side == 0 || s < side {
			side = s
		}
	}
	return side
}

func (c *client) drawCard(screen *ebiten.Image) {
	x, y := float32(c.cardX), float32(c.cardY)
	w, h := float32(c.cardW), float32(c.cardH)
	raised(screen, x, y, w, h, 2, parcheminClair, true)

	c.drawText(screen, "le mot à écrire", float64(x+w/2), float64(y)+26, textLabel, cuir)
	c.drawText(screen, words[c.word], float64(x+w/2), float64(y+h/2), textWord, encre)
}

func (c *client) drawEntry(screen *ebiten.Image) {
	shown, col := c.entry.Text(), encre
	if shown == "" {
		shown, col = "…", cuir
	}
	c.drawText(screen, shown, float64(c.cardX)+float64(c.cardW)/2, float64(c.entryY), textWord, col)
}

func (c *client) drawKeyboard(screen *ebiten.Image) {
	vector.FillRect(screen, float32(c.boardX), float32(c.boardY),
		float32(c.boardW), float32(c.boardH), parcheminVieux, false)

	for _, k := range c.kb.Keys() {
		pressed := (c.holding && sameKey(c.held, k)) || (c.echoTicks > 0 && sameKey(c.echoKey, k))
		drawKey(screen, c, k, float64(c.boardX+k.X), float64(c.boardY+k.Y), float64(k.W), float64(k.H), pressed)
	}
}

// drawKey draws one key in the language of ENCRE_06 §4: a rounded face on a
// body of Cuir clair, a three-pixel hem of that body along the bottom, and the
// candle's rim on the top and left. Pressing takes the hem to one pixel, sinks
// the face by two and puts the rim out.
func drawKey(dst *ebiten.Image, c *client, k ui.Key, x, y, w, h float64, pressed bool) {
	hem := float32(hemHeight)
	if pressed {
		hem = 1
		y += pressSink
		h -= pressSink
	}
	face := parcheminClair
	if k.Kind != ui.KeyRune {
		face = parchemin
	}
	raised(dst, float32(x), float32(y), float32(w), float32(h), hem, face, !pressed)

	inner := h - float64(hem)
	switch k.Kind {
	case ui.KeyErase:
		drawEraser(dst, float32(x+w/2), float32(y+inner*0.36), float32(min(w*0.62, inner*0.66)))
		c.drawText(dst, "efface", x+w/2, y+inner*0.78, textLabel, cuir)
	case ui.KeyValidate:
		drawSeal(dst, float32(x+w/2), float32(y+inner*0.38), float32(min(w*0.20, inner*0.24)))
		c.drawText(dst, "valide", x+w/2, y+inner*0.78, textLabel, brique)
	default:
		c.drawText(dst, k.Label(), x+w/2, y+inner/2, textKey, encre)
	}

	// A dot marks the keys that hold more under a long press.
	if k.Kind == ui.KeyRune && len(ui.Variants(k.Rune)) > 0 {
		roundedRect(dst, float32(x+w-6), float32(y+4), 2, 2, 1, cuirClair)
	}
}

// drawOpenVariants draws the accents of a held key above it.
func (c *client) drawOpenVariants(screen *ebiten.Image) {
	vs := c.openVariants()
	if len(vs) == 0 {
		return
	}
	for n, r := range vs {
		x, y, w, h := c.variantRect(n)
		under := float64(c.pointerX) >= x && float64(c.pointerX) < x+w &&
			float64(c.pointerY) >= y && float64(c.pointerY) < y+h
		face, ink := parcheminClair, encre
		if under {
			face, ink = braise, encre
		}
		vector.FillRect(screen, float32(x+keyInset), float32(y+keyInset),
			float32(w-2*keyInset), float32(h-2*keyInset), cuirClair, false)
		vector.FillRect(screen, float32(x+keyInset), float32(y+keyInset),
			float32(w-2*keyInset), float32(h-2*keyInset-hemHeight), face, false)
		c.drawText(screen, string(r), x+w/2, y+h/2-hemHeight/2, textKey, ink)
	}
}

func sameKey(a, b ui.Key) bool {
	return a.Kind == b.Kind && a.Rune == b.Rune && a.X == b.X && a.Y == b.Y
}

func main() {
	c, err := newClient()
	if err != nil {
		// ENCRE_04 §2: fail loudly. A missing accent would otherwise reach the
		// child as an empty box, in a game about spelling that accent.
		fmt.Fprintf(os.Stderr, "encre: %v\n", err)
		os.Exit(1)
	}

	ebiten.SetWindowTitle("ENCRE — prototype clavier (T00)")
	// On a computer the window opens in landscape, the orientation ENCRE_04 §2
	// gives that device; the 390-wide portrait screen is for the phone.
	mw, mh := ebiten.Monitor().Size()
	scale := ui.WindowScale(ui.LandscapeWidth, ui.LandscapeHeight, mw, mh)
	ebiten.SetWindowSize(ui.LandscapeWidth*scale, ui.LandscapeHeight*scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// A single scene for now, the run screen of the T00 prototype, registered
	// under game.Run: the scene graph of T23 exists so later tickets add
	// scenes here rather than growing client into one, but this one does not
	// yet transition anywhere.
	g := game.NewGame(game.Run, c.juice)
	stage := game.NewStage(g, map[game.SceneID]game.Scene{game.Run: c})
	if err := ebiten.RunGame(&runner{client: c, stage: stage}); err != nil {
		log.Fatal(err)
	}
}

// runner adapts a game.Stage to the ebiten.Game interface: Layout is promoted
// straight from client, since the stage does not change how the window is
// laid out, only what draws inside it.
type runner struct {
	*client
	stage *game.Stage
}

func (r *runner) Update() error { return r.stage.Update() }

func (r *runner) Draw(dst *ebiten.Image) { r.stage.Draw(dst, r.screen) }
