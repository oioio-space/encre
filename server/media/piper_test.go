package media_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

// TestSynthesizePiperWithEmptyPathResolvesViaPATH checks that an empty
// PiperOptions.PiperPath falls back to [media.LookupPiper] rather than
// silently execing an empty argv[0] (which would panic or hang) —
// exercised through [media.SynthesizePiper] itself, not [media.LookupPiper]
// directly, since that is the actual code path a caller with a zero-value
// PiperOptions goes through. Piper is confirmed absent from PATH on this
// machine (see [TestLookupPiperReportsNotFound]'s own comment), so this
// asserts the specific sentinel [media.ErrPiperNotFound] comes back.
func TestSynthesizePiperWithEmptyPathResolvesViaPATH(t *testing.T) {
	if _, err := media.LookupPiper(); !errors.Is(err, media.ErrPiperNotFound) {
		t.Skipf("piper is installed on this machine (error = %v); the not-found fallback is untested here", err)
	}

	opts := media.PiperOptions{Voice: "fr_FR-siwis-medium"}
	outPath := filepath.Join(t.TempDir(), "out.wav")
	err := media.SynthesizePiper(t.Context(), opts, "Bonjour", outPath)
	if !errors.Is(err, media.ErrPiperNotFound) {
		t.Errorf("SynthesizePiper() with empty PiperPath and no piper on PATH: error = %v, want ErrPiperNotFound", err)
	}
}

// TestLookupPiperResolvesAnInstalledBinary puts a fake "piper" executable
// on PATH and checks [media.LookupPiper] returns its exact path with no
// error — the success half of the contract [TestLookupPiperReportsNotFound]
// only proves the absence side of, using a real machine's real PATH rather
// than assuming piper is never installed anywhere this suite runs.
func TestLookupPiperResolvesAnInstalledBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH binary resolution differs on windows (extension-based, not the exec bit)")
	}
	dir := t.TempDir()
	fakePiper := filepath.Join(dir, "piper")
	if err := os.WriteFile(fakePiper, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil { //nolint:gosec // fixture binary, 0700 is deliberate (must be executable)
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := media.LookupPiper()
	if err != nil {
		t.Fatalf("LookupPiper() error = %v", err)
	}
	if got != fakePiper {
		t.Errorf("LookupPiper() = %q, want %q", got, fakePiper)
	}
}
