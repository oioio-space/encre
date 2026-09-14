package main

import (
	"testing"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/ui"
	"github.com/oioio-space/encre/engine"
)

// newTestClient returns a client with just enough state for run.go's own
// state machine to run: no faces, no audio, no ebiten.Image — none of which
// [client.onLetterTyped] or [client.tick] touch.
func newTestClient() *client {
	kb := ui.NewKeyboard(ui.Phone, 390, 318)
	c := &client{
		kb:    kb,
		entry: game.NewEntry(kb),
		juice: anim.DefaultJuice(),
		cfg:   engine.DefaultConfig(),
	}
	c.cards = []runCard{{word: engine.Word{ID: "chat", Text: "chat", Letters: 4}}}
	c.runScore = game.NewRunScore(c.cfg)
	c.runScore.StartManche(0, engine.NoBoss, false)
	c.phase = phaseWriting
	return c
}

// TestOnLetterTypedClearsCorrectionOnceTheChildRetypesCorrectly is a
// regression test for a bug caught by eye (CLAUDE.md's "vérifier avec les
// yeux") rather than by a test: a [game.Correction] set on a wrong letter
// stayed on screen — a blinking "the last letter was wrong" — even after
// the child retyped it correctly and the word went on to complete.
func TestOnLetterTypedClearsCorrectionOnceTheChildRetypesCorrectly(t *testing.T) {
	c := newTestClient()

	for _, r := range "cha" {
		if !c.entry.Type(r) {
			t.Fatalf("entry.Type(%q) = false, want true", r)
		}
		c.onLetterTyped()
	}

	// A wrong letter: the keyboard has no digits, so 'x' is a plain trap.
	c.entry.Type('x')
	c.onLetterTyped()
	if c.correction == nil {
		t.Fatal("onLetterTyped() after a wrong letter: correction = nil, want set")
	}
	if got := c.entry.Text(); got != "cha" {
		t.Errorf("entry.Text() after the wrong letter = %q, want %q (the wrong tail dropped)", got, "cha")
	}

	// The child retypes the letter actually asked for.
	c.entry.Type('t')
	c.onLetterTyped()
	if c.correction != nil {
		t.Errorf("onLetterTyped() after the retype matches = %+v, want nil", *c.correction)
	}
	if c.phase != phaseSilence {
		t.Errorf("phase = %v, want phaseSilence (the word is now complete)", c.phase)
	}
}

// TestOnLetterTypedCompletesAnAlreadyCorrectWord documents the ordinary path
// with no mismatch at all: no correction is ever set, and the word
// completes the instant the last letter lands.
func TestOnLetterTypedCompletesAnAlreadyCorrectWord(t *testing.T) {
	c := newTestClient()

	for _, r := range "chat" {
		c.entry.Type(r)
		c.onLetterTyped()
	}

	if c.correction != nil {
		t.Errorf("correction = %+v, want nil (no letter was ever wrong)", *c.correction)
	}
	if c.phase != phaseSilence {
		t.Errorf("phase = %v, want phaseSilence", c.phase)
	}
}
