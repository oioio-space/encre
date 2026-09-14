package game_test

import (
	"testing"
	"time"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

func TestDeriveCardStateFaceDownWinsOverEverythingElse(t *testing.T) {
	st := &engine.WordState{Cursed: true, Gold: true}
	if got := game.DeriveCardState(st, true); got != game.CardFaceDown {
		t.Errorf("DeriveCardState(cursed+gold, faceDown=true) = %v, want CardFaceDown", got)
	}
}

func TestDeriveCardStatePrecedence(t *testing.T) {
	tests := []struct {
		name string
		st   *engine.WordState
		want game.CardState
	}{
		{"never met", &engine.WordState{}, game.CardRencontre},
		{"met, nothing special", &engine.WordState{Seen: true}, game.CardNormal},
		{"gold", &engine.WordState{Seen: true, Gold: true}, game.CardGold},
		{"tarnished", &engine.WordState{Seen: true, Tarnished: true}, game.CardTarnished},
		{"cursed outranks having been met", &engine.WordState{Seen: true, Cursed: true}, game.CardCursed},
		{
			"cursed outranks gold too",
			&engine.WordState{Seen: true, Gold: true, Cursed: true},
			game.CardCursed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := game.DeriveCardState(tt.st, false); got != tt.want {
				t.Errorf("DeriveCardState(%+v, false) = %v, want %v", tt.st, got, tt.want)
			}
		})
	}
}

func TestCardFlipScaleXStartsAndEndsAtOne(t *testing.T) {
	f := game.NewCardFlip(anim.DefaultJuice())

	if got := f.ScaleX(0); got != 1 {
		t.Errorf("ScaleX(0) = %v, want 1", got)
	}
	if got := f.ScaleX(f.Duration()); got != 1 {
		t.Errorf("ScaleX(Duration()) = %v, want 1", got)
	}
	if got := f.ScaleX(f.Duration() * 2); got != 1 {
		t.Errorf("ScaleX(past the end) = %v, want 1", got)
	}
}

func TestCardFlipScaleXIsThinnestAtTheMidpoint(t *testing.T) {
	j := anim.DefaultJuice()
	f := game.NewCardFlip(j)
	mid := f.Duration() / 2

	got := f.ScaleX(mid)
	if got >= 1 {
		t.Fatalf("ScaleX(midpoint) = %v, want under 1", got)
	}
	// The linear easing this uses (ease-in-out, symmetric) reaches exactly
	// its minimum at the midpoint: a quarter and three-quarters through
	// should both be wider than the midpoint itself.
	if q := f.ScaleX(mid / 2); q <= got {
		t.Errorf("ScaleX(quarter) = %v, want more than ScaleX(midpoint) = %v", q, got)
	}
	if q := f.ScaleX(mid + mid/2); q <= got {
		t.Errorf("ScaleX(three-quarters) = %v, want more than ScaleX(midpoint) = %v", q, got)
	}
	wantMin := j.CardFlipMinScaleX
	if diff := got - wantMin; diff < -1e-9 || diff > 1e-9 {
		t.Errorf("ScaleX(midpoint) = %v, want CardFlipMinScaleX = %v", got, wantMin)
	}
}

func TestCardFlipScaleXIsSymmetric(t *testing.T) {
	f := game.NewCardFlip(anim.DefaultJuice())
	total := f.Duration()

	for _, elapsed := range []time.Duration{10 * time.Millisecond, 100 * time.Millisecond, 140 * time.Millisecond} {
		mirrored := total - elapsed
		got, want := f.ScaleX(elapsed), f.ScaleX(mirrored)
		if got != want {
			t.Errorf("ScaleX(%v) = %v, ScaleX(%v) = %v, want equal (symmetric flip)", elapsed, got, mirrored, want)
		}
	}
}

func TestCardFlipEdgeVisibleOnlyNearTheMidpoint(t *testing.T) {
	f := game.NewCardFlip(anim.DefaultJuice())
	total := f.Duration()

	if f.EdgeVisible(0) {
		t.Error("EdgeVisible(0) = true, want false: the card starts on a full face")
	}
	if !f.EdgeVisible(total / 2) {
		t.Error("EdgeVisible(midpoint) = false, want true: the card is thinnest here")
	}
	if f.EdgeVisible(total) {
		t.Error("EdgeVisible(Duration()) = true, want false: the card ends on a full face")
	}
}
