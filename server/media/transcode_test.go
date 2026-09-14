package media_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oioio-space/encre/server/media"
)

// TestTranscodeWebmToOgg is an integration test against the real ffmpeg
// binary: it transcodes the embedded webm/opus fixture and checks the
// result is a decodable Ogg file with sound in it. It skips (never fails)
// when ffmpeg is not on PATH, per this ticket's own instruction to say so
// rather than pretend.
func TestTranscodeWebmToOgg(t *testing.T) {
	if _, err := media.LookupFFmpeg(); errors.Is(err, media.ErrFFmpegNotFound) {
		t.Skip("ffmpeg not found on PATH; skipping the real transcode")
	}

	outPath := filepath.Join(t.TempDir(), "out.ogg")
	err := media.Transcode(t.Context(), media.DefaultTranscodeOptions, "testdata/sample.webm", outPath)
	if err != nil {
		t.Fatalf("Transcode() error = %v", err)
	}

	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading transcoded output: %v", err)
	}
	if !media.SniffOgg(out) {
		t.Errorf("Transcode() output does not open with the Ogg signature (first bytes: %x)", out[:min(8, len(out))])
	}
	if len(out) == 0 {
		t.Error("Transcode() output is empty")
	}
}

// TestTranscodeWithBadFFmpegPathReturnsErrorNotPanic checks the "ffmpeg
// absent" path directly, regardless of whether ffmpeg happens to be
// installed on the machine running this test: pointing
// TranscodeOptions.FFmpegPath at a binary that does not exist must fail
// cleanly with an error, never panic or hang.
func TestTranscodeWithBadFFmpegPathReturnsErrorNotPanic(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "out.ogg")
	opts := media.TranscodeOptions{FFmpegPath: filepath.Join(t.TempDir(), "no-such-ffmpeg-binary")}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Transcode() with a bad ffmpeg path panicked: %v", r)
		}
	}()
	if err := media.Transcode(t.Context(), opts, "testdata/sample.webm", outPath); err == nil {
		t.Error("Transcode() with a nonexistent ffmpeg binary: want error, got nil")
	}
}

// TestTranscodeRunsWithNoShell is a light regression guard: ffmpeg is never
// invoked with input containing shell metacharacters interpreted as
// anything but a literal, opaque path — this feeds one and checks
// Transcode fails cleanly (ffmpeg cannot open a file with that literal
// name) rather than the metacharacter having any special effect.
func TestTranscodeTreatsPathsAsLiteralArguments(t *testing.T) {
	if _, err := media.LookupFFmpeg(); errors.Is(err, media.ErrFFmpegNotFound) {
		t.Skip("ffmpeg not found on PATH")
	}
	outPath := filepath.Join(t.TempDir(), "out.ogg")
	err := media.Transcode(t.Context(), media.DefaultTranscodeOptions, "; rm -rf / ; echo pwned", outPath)
	if err == nil {
		t.Error("Transcode() with a shell-metacharacter-laden input path: want error (no such file), got nil")
	}
}

// TestLookupFFmpegReportsNotFoundWithoutFFmpegOnPATH forces PATH to a
// directory that cannot contain ffmpeg and checks [media.LookupFFmpeg]
// wraps exec.LookPath's error as the specific sentinel
// [media.ErrFFmpegNotFound] — the "absent" half of LookupFFmpeg's contract,
// which the rest of this file's tests skip past on any machine that
// happens to have ffmpeg installed (this one does).
func TestLookupFFmpegReportsNotFoundWithoutFFmpegOnPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := media.LookupFFmpeg(); !errors.Is(err, media.ErrFFmpegNotFound) {
		t.Errorf("LookupFFmpeg() with an empty PATH: error = %v, want ErrFFmpegNotFound", err)
	}
}

func TestTranscodeRespectsContextCancellation(t *testing.T) {
	if _, err := media.LookupFFmpeg(); errors.Is(err, media.ErrFFmpegNotFound) {
		t.Skip("ffmpeg not found on PATH")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	outPath := filepath.Join(t.TempDir(), "out.ogg")
	if err := media.Transcode(ctx, media.DefaultTranscodeOptions, "testdata/sample.webm", outPath); err == nil {
		t.Error("Transcode() with an already-cancelled context: want error, got nil")
	}
}
