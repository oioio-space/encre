package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrFFmpegNotFound is returned by [LookupFFmpeg] and [Transcode] when
// ffmpeg is not on PATH. The caller must answer the parent plainly — no
// voice upload today — rather than let the request hang or fail some other
// way (encre-6z2's acceptance: "ffmpeg peut être absent — le dire
// proprement, pas paniquer").
var ErrFFmpegNotFound = errors.New("media: ffmpeg not found on PATH")

// LookupFFmpeg resolves the ffmpeg binary on PATH, wrapping
// [exec.LookPath]'s error as [ErrFFmpegNotFound] so every caller can check
// for exactly one sentinel regardless of exec's own error text.
func LookupFFmpeg() (string, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", ErrFFmpegNotFound
	}
	return path, nil
}

// loudnessFilter is ENCRE_04 §8's normalization and silence trim in one
// filter chain: loudnorm brings the recording to a consistent level (the
// EBU R128 defaults ffmpeg's own loudnorm filter ships with), and
// silenceremove cuts silence from both the start (start_periods=1) and the
// end (stop_periods=-1, meaning "every trailing silent period, not just the
// first") at -50 dB — quiet enough that a parent's normal speaking pause
// mid-sentence is never mistaken for the recording's own lead-in or
// trailing silence.
const loudnessFilter = "loudnorm," +
	"silenceremove=start_periods=1:start_threshold=-50dB:start_silence=0.1:" +
	"stop_periods=-1:stop_threshold=-50dB:stop_silence=0.1"

// TranscodeOptions configures [Transcode]. Its zero value is
// [DefaultTranscodeOptions].
type TranscodeOptions struct {
	// FFmpegPath is the ffmpeg binary to run; empty means "resolve via
	// [LookupFFmpeg] on every call" (the default — [Transcode] does this
	// for a zero-value TranscodeOptions).
	FFmpegPath string
}

// Transcode converts inPath (webm/opus, ENCRE_04 §8's MediaRecorder output)
// to outPath (Ogg Vorbis, quality 4, 48 kHz), normalized and with silence
// trimmed at both ends.
//
// It runs ffmpeg through [exec.CommandContext] with every argument as its
// own slice element — never through a shell, and never with inPath or
// outPath interpolated into a string ffmpeg or a shell could reinterpret.
// Both paths are expected to already be validated ([Root.Path]'s contract);
// this function does not itself re-validate them, only passes them through
// untouched.
//
// It returns [ErrFFmpegNotFound] if ffmpeg is not on PATH, and otherwise
// wraps ffmpeg's own stderr into the returned error on a non-zero exit, so
// the caller's log carries what actually went wrong (a corrupt upload, an
// unsupported codec) rather than just "exit status 1".
func Transcode(ctx context.Context, opts TranscodeOptions, inPath, outPath string) error {
	ffmpegPath := opts.FFmpegPath
	if ffmpegPath == "" {
		found, err := LookupFFmpeg()
		if err != nil {
			return err
		}
		ffmpegPath = found
	}

	args := []string{
		"-y", "-loglevel", "error",
		"-i", inPath,
		"-vn",
		"-af", loudnessFilter,
		"-c:a", "libvorbis", "-q:a", "4", "-ar", "48000",
		outPath,
	}
	// #nosec G204 -- ffmpegPath comes from LookupFFmpeg (exec.LookPath) or
	// an operator-configured TranscodeOptions, never from a request; args
	// are a fixed slice with inPath/outPath as opaque elements, never
	// shell-interpreted (no exec.Command("sh", "-c", ...) anywhere in this
	// package).
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("media: ffmpeg failed: %w: %s", err, stderr.String())
	}
	return nil
}

// DefaultTranscodeOptions is the zero [TranscodeOptions] — ffmpeg resolved
// via PATH on every call.
var DefaultTranscodeOptions = TranscodeOptions{}
