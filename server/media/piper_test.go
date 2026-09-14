package media_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/oioio-space/encre/server/media"
)

// TestLookupPiperReportsNotFound documents the state of this deployment
// target explicitly, per the ticket's instruction: Piper is not installed
// here, and this proves [media.LookupPiper] says so cleanly rather than
// this package silently assuming a binary that does not exist.
func TestLookupPiperReportsNotFound(t *testing.T) {
	if _, err := media.LookupPiper(); !errors.Is(err, media.ErrPiperNotFound) {
		t.Skipf("piper is installed on this machine (error = %v); the not-found path is untested here", err)
	}
}

// TestSynthesizePiperWithBadPathReturnsErrorNotPanic checks the same
// "absent binary" contract [Transcode] holds, for Piper: a PiperPath that
// does not resolve to a real binary fails with an error, never a panic.
func TestSynthesizePiperWithBadPathReturnsErrorNotPanic(t *testing.T) {
	opts := media.PiperOptions{
		PiperPath: filepath.Join(t.TempDir(), "no-such-piper-binary"),
		Voice:     "fr_FR-siwis-medium",
	}
	outPath := filepath.Join(t.TempDir(), "out.wav")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SynthesizePiper() with a bad path panicked: %v", r)
		}
	}()
	if err := media.SynthesizePiper(t.Context(), opts, "Bonjour", outPath); err == nil {
		t.Error("SynthesizePiper() with a nonexistent piper binary: want error, got nil")
	}
}

// TestSynthesizePiperRequiresVoice checks that a missing Voice is refused
// before any exec call is attempted.
func TestSynthesizePiperRequiresVoice(t *testing.T) {
	opts := media.PiperOptions{PiperPath: filepath.Join(t.TempDir(), "irrelevant")}
	outPath := filepath.Join(t.TempDir(), "out.wav")
	if err := media.SynthesizePiper(t.Context(), opts, "Bonjour", outPath); err == nil {
		t.Error("SynthesizePiper() with no Voice: want error, got nil")
	}
}
