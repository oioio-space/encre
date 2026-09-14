package ui_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/oioio-space/encre/client/ui"
)

// A seven-year-old misses a 9 mm target about one tap in six (Anthony et al.,
// "Physical dimensions of children's touchscreen interactions", IJHCS 2019 —
// 116 children, 55 000 taps). Seven columns on a 390 pt screen cannot be made
// bigger: the geometry caps a key at about 9 mm however the padding is spent.
// What can be fixed is the space *between* the keys, which the same work names
// as the main cause of hitting the neighbour. So the drawn key keeps its size,
// and the key that answers is the nearest one — the gaps stop being dead.

// TestATapInsideAKeyAlwaysReturnsThatKey is the promise the drawing makes. It
// must survive any forgiveness added around the edges: what is under the finger
// wins over what is near it.
func TestATapInsideAKeyAlwaysReturnsThatKey(t *testing.T) {
	t.Parallel()

	kb := ui.NewKeyboard(ui.Phone, 390, 334)
	rapid.Check(t, func(rt *rapid.T) {
		i := rapid.IntRange(0, len(kb.Keys())-1).Draw(rt, "key")
		key := kb.Keys()[i]
		x := rapid.IntRange(key.X, key.X+key.W-1).Draw(rt, "x")
		y := rapid.IntRange(key.Y, key.Y+key.H-1).Draw(rt, "y")

		got, ok := kb.KeyAt(x, y)
		if !ok {
			rt.Fatalf("KeyAt(%d, %d) found nothing inside key %q", x, y, key.Label())
		}
		if got.X != key.X || got.Y != key.Y {
			rt.Errorf("KeyAt(%d, %d) = %q, want the key drawn there, %q", x, y, got.Label(), key.Label())
		}
	})
}

// TestNoDeadSpaceBetweenTheKeys is the change itself: anywhere inside the block
// of keys, a tap lands on a key. The gaps are decoration, not holes.
func TestNoDeadSpaceBetweenTheKeys(t *testing.T) {
	t.Parallel()

	kb := ui.NewKeyboard(ui.Phone, 390, 334)
	minX, minY, maxX, maxY := bounds(kb)

	rapid.Check(t, func(rt *rapid.T) {
		x := rapid.IntRange(minX, maxX).Draw(rt, "x")
		y := rapid.IntRange(minY, maxY).Draw(rt, "y")
		if _, ok := kb.KeyAt(x, y); !ok {
			rt.Errorf("KeyAt(%d, %d) found nothing, inside the block of keys", x, y)
		}
	})
}

// TestTheNearestKeyWins keeps the forgiveness honest: it hands the tap to the
// key it actually fell closest to, never to a luckier neighbour.
func TestTheNearestKeyWins(t *testing.T) {
	t.Parallel()

	kb := ui.NewKeyboard(ui.Phone, 390, 334)
	minX, minY, maxX, maxY := bounds(kb)

	rapid.Check(t, func(rt *rapid.T) {
		x := rapid.IntRange(minX-20, maxX+20).Draw(rt, "x")
		y := rapid.IntRange(minY-20, maxY+20).Draw(rt, "y")

		got, ok := kb.KeyAt(x, y)
		if !ok {
			return // out of reach, which the next test covers
		}
		want := distance(got, x, y)
		for _, other := range kb.Keys() {
			if d := distance(other, x, y); d < want {
				rt.Errorf("KeyAt(%d, %d) = %q at %d, but %q is nearer at %d",
					x, y, got.Label(), want, other.Label(), d)
			}
		}
	})
}

// TestAFarTapIsIgnored stops the forgiveness from swallowing the whole screen:
// a tap on the card is not a tap on the keyboard.
func TestAFarTapIsIgnored(t *testing.T) {
	t.Parallel()

	kb := ui.NewKeyboard(ui.Phone, 390, 334)
	minX, minY, _, maxY := bounds(kb)
	for _, p := range []struct{ x, y int }{
		{x: minX, y: minY - 200},
		{x: minX, y: maxY + 200},
		{x: -200, y: minY},
	} {
		if key, ok := kb.KeyAt(p.x, p.y); ok {
			t.Errorf("KeyAt(%d, %d) = %q, want nothing that far from the keyboard", p.x, p.y, key.Label())
		}
	}
}

func bounds(kb *ui.Keyboard) (minX, minY, maxX, maxY int) {
	keys := kb.Keys()
	minX, minY = keys[0].X, keys[0].Y
	maxX, maxY = keys[0].X+keys[0].W-1, keys[0].Y+keys[0].H-1
	for _, k := range keys[1:] {
		minX, minY = min(minX, k.X), min(minY, k.Y)
		maxX, maxY = max(maxX, k.X+k.W-1), max(maxY, k.Y+k.H-1)
	}
	return minX, minY, maxX, maxY
}

// distance is how far the point lies outside the key's drawn rectangle, squared
// so it stays in integers. Zero means inside.
func distance(k ui.Key, x, y int) int {
	dx := max(k.X-x, 0, x-(k.X+k.W-1))
	dy := max(k.Y-y, 0, y-(k.Y+k.H-1))
	return dx*dx + dy*dy
}
