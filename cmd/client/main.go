// Command client is the ENCRE game client: an Ebitengine v2 program built both
// natively (linux/darwin/windows) and for the browser with GOOS=js GOARCH=wasm.
//
// It currently holds the Étape 0 prototype of brief/ENCRE_05 ticket T00: a grey
// card, the drawn AZERTY keyboard with its accent row, and a word to copy. The
// question it exists to answer is not whether the code works but whether a
// seven-year-old can type "garçon" on a real tablet without fighting the
// interface — so it measures its own touch targets and shows the number, rather
// than asserting they are large enough.
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

// Logical resolutions from brief/ENCRE_04_spec_technique.md §2. The layout
// follows the window's orientation instead of forcing a rotation, so the client
// chooses between the two on every Layout call.
const (
	portraitWidth, portraitHeight   = 390, 844
	landscapeWidth, landscapeHeight = 1280, 720
)

// The palette of brief/ENCRE_02 §3. No pure black, no pure white; Or never
// touches the interface, so none of it appears here.
var (
	nuit           = color.RGBA{R: 0x0B, G: 0x0A, B: 0x14, A: 0xFF} // Nuit
	encreProfonde  = color.RGBA{R: 0x16, G: 0x15, B: 0x2A, A: 0xFF} // Encre profonde
	encre          = color.RGBA{R: 0x24, G: 0x23, B: 0x42, A: 0xFF} // Encre
	encreDiluee    = color.RGBA{R: 0x3A, G: 0x38, B: 0x66, A: 0xFF} // Encre diluée
	parcheminClair = color.RGBA{R: 0xFB, G: 0xF3, B: 0xDE, A: 0xFF} // Parchemin clair
	parchemin      = color.RGBA{R: 0xEA, G: 0xD9, B: 0xB4, A: 0xFF} // Parchemin
	braise         = color.RGBA{R: 0xF5, G: 0xA7, B: 0x42, A: 0xFF} // Braise
	vertDeGris     = color.RGBA{R: 0x7F, G: 0xA6, B: 0x9A, A: 0xFF} // Vert-de-gris
)

// Words carrying the accents ENCRE_03 §4 teaches at CE1, taken from its own
// examples: é (école, bébé, été), è (mère, père), ê (forêt, fête, tête) and
// ç (garçon, français).
var words = []string{
	"école", "bébé", "été", "mère", "père",
	"forêt", "fête", "tête", "garçon", "français",
}

// keyInset is how far the drawn key sits inside its touch area. The target is
// deliberately larger than the picture of it: at seven the finger lands wide of
// where the eye aimed (ENCRE_02 §11).
const keyInset = 2

// pressDepth is the 3 logical pixels a key sinks when touched (ENCRE_02 §11).
const pressDepth = 3

// maxKeyAspect caps how much taller than wide a DRAWN key may be. The touch
// cell keeps the whole height it was given — a bigger target is the point — but
// a key drawn 39 wide and 95 tall reads as a column, not a key.
const maxKeyAspect = 1.6

// Text is drawn from one 12-pixel bitmap face scaled by whole numbers, never
// re-rasterised at another size: any other factor lands the glyph off the pixel
// grid and turns pixel art into mush (ENCRE_04 §2).
const (
	baseTextPx = 12
	textSmall  = 1 // 12 px — diagnostics
	textKey    = 2 // 24 px — the letters on the keys
	textWord   = 3 // 36 px — the word to copy and the word being typed
)

type client struct {
	kb    *ui.Keyboard
	entry *game.Entry

	plume *audio.Player
	// audioUnlocked records the browser's one-gesture rule: the audio context
	// does not start until the player has touched something (ENCRE_04 §2).
	audioUnlocked bool

	word int // index into words

	// Geometry recomputed on every Layout, because the browser can rotate or
	// resize the canvas at any moment.
	screenW, screenH     int
	outsideW             int
	boardX, boardY       int
	cardY, cardH, entryY int
	// cellW is the width of an ordinary letter key. The two special keys are
	// wider, so capping each key's height against its own width would make them
	// tower over the letters; the whole row is capped against this instead.
	cellW      int
	pressed    ui.Key
	hasPressed bool
	pressTicks int
	face       *text.GoXFace
}

func newClient() (*client, error) {
	// La Plume and Le Greffe are Design deliverables (ENCRE_02 §16) that do not
	// exist yet. bitmapfont stands in: a real pixel font, 12 px, and — checked
	// here rather than assumed — one that already carries every accent the
	// charter asks for, which the charter expects to have to draw by hand.
	required := ui.RequiredRunes(ui.AZERTY) + "…×→"
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

// drawText draws s centred on (cx, cy), enlarged by the whole factor scale.
//
// The enlargement is a nearest-neighbour blit of the 12-pixel glyphs, not a
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

// Layout reports the logical resolution for the current window: portrait when
// the window is at least as tall as it is wide, landscape otherwise, and lays
// the screen bands out for it (ENCRE_02 §14).
func (c *client) Layout(outsideWidth, outsideHeight int) (int, int) {
	c.outsideW = outsideWidth
	if outsideHeight >= outsideWidth {
		c.screenW, c.screenH = portraitWidth, portraitHeight
		// The portrait bands of ENCRE_02 §14, as fractions of the height:
		// counter 12%, magnifier and flame 6%, the card 28%, the word being
		// written 9%, the keyboard 45%.
		counter := c.screenH * 12 / 100
		flame := c.screenH * 6 / 100
		c.cardH = c.screenH * 28 / 100
		word := c.screenH * 9 / 100
		c.cardY = counter + flame
		c.entryY = c.cardY + c.cardH + word/2
		c.boardX, c.boardY = 0, c.cardY+c.cardH+word
		c.kb = ui.NewKeyboard(ui.AZERTY, c.screenW, c.screenH-c.boardY)
	} else {
		c.screenW, c.screenH = landscapeWidth, landscapeHeight
		// Landscape (ENCRE_02 §14): keyboard on the right 55% of the width with
		// the counter above it, card on the left with the word beneath.
		boardW := c.screenW * 55 / 100
		counter := c.screenH * 10 / 100
		c.boardX, c.boardY = c.screenW-boardW, counter
		c.cardH = c.screenH * 45 / 100
		c.cardY = counter + c.screenH*5/100
		c.entryY = c.cardY + c.cardH + c.screenH*12/100
		c.kb = ui.NewKeyboard(ui.AZERTY, boardW, c.screenH-counter)
	}
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
	if c.pressTicks > 0 {
		c.pressTicks--
		if c.pressTicks == 0 {
			c.hasPressed = false
		}
	}

	// Touch and mouse: the drawn keyboard, which is the only one the game uses.
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		c.press(x, y)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		c.press(ebiten.CursorPosition())
	}

	// The physical keyboard of the computer. Accented characters arrive here
	// and nowhere else (ENCRE_04 §2), so this is the path that must not filter
	// them out: game.Entry accepts exactly what the drawn keyboard can produce.
	for _, r := range ebiten.AppendInputChars(nil) {
		if c.entry.Type(r) {
			c.echo(r)
			c.playPlume()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		c.entry.Erase()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		c.validate()
	}
	return nil
}

// press routes a tap at a screen point to the key under it.
func (c *client) press(x, y int) {
	key, ok := c.kb.KeyAt(x-c.boardX, y-c.boardY)
	if !ok {
		return
	}
	c.pressed, c.hasPressed, c.pressTicks = key, true, 6

	switch key.Kind {
	case ui.KeyErase:
		c.entry.Erase()
	case ui.KeyValidate:
		c.validate()
	case ui.KeyRune:
		c.entry.Type(key.Rune)
	}
	c.playPlume()
}

// echo lights the drawn key matching a character typed on the physical
// keyboard, so the computer and the tablet show the same thing (ENCRE_02 §11).
func (c *client) echo(r rune) {
	for _, k := range c.kb.Keys() {
		if k.Kind == ui.KeyRune && k.Rune == r {
			c.pressed, c.hasPressed, c.pressTicks = k, true, 6
			return
		}
	}
}

func (c *client) validate() {
	c.word = (c.word + 1) % len(words)
	c.entry.Clear()
}

// playPlume restarts the scratch. The first call is also what unlocks the
// browser's audio context, which is why it hangs off the first tap rather than
// off startup.
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
	screen.Fill(nuit)
	c.drawDiagnostics(screen)
	c.drawCard(screen)
	c.drawEntry(screen)
	c.drawKeyboard(screen)
}

// drawDiagnostics reports the measurements ticket T00 asks for. They are on
// screen rather than in a log because the device that has to be measured is a
// phone held in someone's hand, with no console to read.
func (c *client) drawDiagnostics(screen *ebiten.Image) {
	side := c.kb.MinTouchSide()
	dsf := ebiten.Monitor().DeviceScaleFactor()
	physical := ui.PhysicalPx(side, c.screenW, c.outsideW, dsf)

	col := vertDeGris
	verdict := "OK"
	if physical < 48 {
		col, verdict = braise, "TROP PETIT"
	}
	audio := "en attente du 1er appui"
	if c.audioUnlocked {
		audio = "débloqué"
	}

	lines := []struct {
		s   string
		col color.Color
	}{
		{fmt.Sprintf("touche %d px logiques → %.0f px physiques · %s (min 48)", side, physical, verdict), col},
		{fmt.Sprintf("écran %d×%d · fenêtre %d dip · échelle ×%.2f", c.screenW, c.screenH, c.outsideW, dsf), encreDiluee},
		{fmt.Sprintf("%.0f img/s · audio %s", ebiten.ActualFPS(), audio), encreDiluee},
	}
	// Keep the readout clear of the keyboard: in landscape the middle of the
	// screen is under the keys.
	cx := float64(c.screenW) / 2
	if c.screenW > c.screenH {
		cx = float64(c.boardX) / 2
	}
	for i, l := range lines {
		c.drawText(screen, l.s, cx, float64(14+i*16), textSmall, l.col)
	}
}

// drawCard draws the grey card holding the word to copy. Grey because the
// colours of ENCRE_02 §3 belong to sprites that do not exist yet, and a
// placeholder wearing the real palette invites being mistaken for the design.
func (c *client) drawCard(screen *ebiten.Image) {
	w := float64(c.screenW) * 0.62
	if c.screenW > c.screenH {
		w = float64(c.boardX) * 0.7
	}
	x := float64(c.cardX()) - w/2
	vector.FillRect(screen, float32(x), float32(c.cardY), float32(w), float32(c.cardH), encre, false)
	vector.StrokeRect(screen, float32(x), float32(c.cardY), float32(w), float32(c.cardH), 2, encreDiluee, false)

	c.drawText(screen, "copie ce mot", float64(c.cardX()), float64(c.cardY+28), textSmall, encreDiluee)
	c.drawText(screen, words[c.word], float64(c.cardX()), float64(c.cardY+c.cardH/2), textWord, parcheminClair)
}

// cardX is the horizontal centre of the card area.
func (c *client) cardX() int {
	if c.screenW > c.screenH {
		return c.boardX / 2
	}
	return c.screenW / 2
}

// drawEntry draws the word as the child types it.
func (c *client) drawEntry(screen *ebiten.Image) {
	shown, col := c.entry.Text(), parchemin
	if shown == "" {
		shown, col = "…", encreDiluee
	}
	c.drawText(screen, shown, float64(c.cardX()), float64(c.entryY), textWord, col)
}

func (c *client) drawKeyboard(screen *ebiten.Image) {
	for _, k := range c.kb.Keys() {
		// The drawn key is centred in its touch cell and capped in height: the
		// cell stays as large as it was laid out, because that is what the
		// finger has to hit.
		w := float64(k.W - 2*keyInset)
		h := math.Min(float64(k.H-2*keyInset), float64(c.cellW)*maxKeyAspect)
		x := float64(c.boardX+k.X) + (float64(k.W)-w)/2
		y := float64(c.boardY+k.Y) + (float64(k.H)-h)/2

		fill, label, labelCol, scale := encreProfonde, k.Label(), parcheminClair, textKey
		switch k.Kind {
		case ui.KeyErase:
			fill, label, scale = encre, "effacer", textSmall
		case ui.KeyValidate:
			fill, label, labelCol, scale = encre, "valider", braise, textSmall
		}
		if c.hasPressed && sameKey(c.pressed, k) {
			fill, y, h = encreDiluee, y+pressDepth, h-pressDepth
		}

		vector.FillRect(screen, float32(x), float32(y), float32(w), float32(h), fill, false)
		c.drawText(screen, label, x+w/2, y+h/2, scale, labelCol)
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
	// Open at the largest whole multiple the monitor allows. At ×1 on a desktop
	// the 390-pixel-wide phone screen is a postage stamp and its 12-pixel text
	// is unreadable — which is not a finding about the design, only about the
	// window it was shown in.
	// On a computer the window opens in landscape, which is the orientation
	// ENCRE_04 §2 gives that device; the 390-wide portrait screen is for the
	// phone and the tablet, and on a desktop it is an unreadable stamp.
	mw, mh := ebiten.Monitor().Size()
	scale := ui.WindowScale(landscapeWidth, landscapeHeight, mw, mh)
	ebiten.SetWindowSize(landscapeWidth*scale, landscapeHeight*scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(c); err != nil {
		log.Fatal(err)
	}
}
