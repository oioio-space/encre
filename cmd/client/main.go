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
	"os"
	"time"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

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
	keyInset  = 2                      // the drawn key sits inside its touch cell
	hemHeight = 3                      // ourlet bas (ENCRE_06 §4)
	pressSink = 2                      // translateY on press (ENCRE_06 §4)
	holdDelay = 350 * time.Millisecond // before the accents of a key open
)

type client struct {
	kb    *ui.Keyboard
	entry *game.Entry
	face  *text.GoXFace

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
	// La Plume and Le Greffe are Design deliverables (ENCRE_02 §16) that do not
	// exist yet. bitmapfont stands in: a real pixel font that — checked here
	// rather than assumed — already carries every accent the charter asks for.
	required := ui.RequiredRunes(ui.Phone) + ui.RequiredRunes(ui.AZERTY) + "…·×→"
	if missing := ui.MissingGlyphsInFace(bitmapfont.Face, required); len(missing) > 0 {
		return nil, fmt.Errorf("the font cannot draw %d required rune(s): %q", len(missing), string(missing))
	}

	c := &client{face: text.NewGoXFace(bitmapfont.Face)}

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

func (c *client) Update() error {
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

func (c *client) Draw(screen *ebiten.Image) {
	screen.Fill(parchemin)
	c.drawDiagnostics(screen)
	c.drawCard(screen)
	c.drawEntry(screen)
	c.drawKeyboard(screen)
	c.drawOpenVariants(screen)
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
	x, y := float64(c.cardX), float64(c.cardY)
	w, h := float64(c.cardW), float64(c.cardH)
	vector.FillRect(screen, float32(x), float32(y), float32(w), float32(h), parcheminClair, false)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 1, parcheminVieux, false)

	c.drawText(screen, "le mot à écrire", x+w/2, y+22, textLabel, cuir)
	c.drawText(screen, words[c.word], x+w/2, y+h/2, textWord, encre)
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
		c.drawKey(screen, k, float64(c.boardX+k.X), float64(c.boardY+k.Y), float64(k.W), float64(k.H), pressed)
	}
}

// drawKey draws one key with the hem of ENCRE_06 §4: a three-pixel lip of Cuir
// clair under the face, brought down to one and the face sunk by two when it is
// pressed. That lip is the whole of the key's relief — the charter allows no
// gradient — so losing it loses the affordance.
func (c *client) drawKey(dst *ebiten.Image, k ui.Key, x, y, w, h float64, pressed bool) {
	x, y, w, h = x+keyInset, y+keyInset, w-2*keyInset, h-2*keyInset

	hem := float64(hemHeight)
	if pressed {
		hem = 1
		y += pressSink
		h -= pressSink
	}
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), cuirClair, false)
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h-hem), parcheminClair, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h-hem), 1, parcheminVieux, false)

	label, col, scale := k.Label(), encre, textKey
	switch k.Kind {
	case ui.KeyErase:
		col, scale = cuir, textLabel
	case ui.KeyValidate:
		col, scale = brique, textLabel
	}
	c.drawText(dst, label, x+w/2, y+(h-hem)/2, scale, col)

	// A dot marks the keys that hold more under a long press.
	if k.Kind == ui.KeyRune && len(ui.Variants(k.Rune)) > 0 {
		vector.FillRect(dst, float32(x+w-5), float32(y+3), 2, 2, cuirClair, false)
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
	if err := ebiten.RunGame(c); err != nil {
		log.Fatal(err)
	}
}
