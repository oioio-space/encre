package media

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrPiperNotFound is returned by [LookupPiper] and [SynthesizePiper] when
// the piper binary is not on PATH.
//
// Piper is not installed on the machine this ticket was built and tested
// on — confirmed by looking, not assumed — so [SynthesizePiper]'s
// exec.CommandContext call path is written against Piper's documented CLI
// but has never actually been run end to end here. It is exercised by
// [TestSynthesizePiperReportsNotFound], which only proves the "absent"
// path; a deployment that installs piper needs its own smoke test before
// this is trusted as more than "the code that would run if piper were there".
var ErrPiperNotFound = errors.New("media: piper not found on PATH")

// LookupPiper resolves the piper binary on PATH, wrapping
// [exec.LookPath]'s error as [ErrPiperNotFound].
func LookupPiper() (string, error) {
	path, err := exec.LookPath("piper")
	if err != nil {
		return "", ErrPiperNotFound
	}
	return path, nil
}

// PiperOptions configures [SynthesizePiper]. Voice must name a Piper voice
// (ENCRE_04 §8: "fr_FR-siwis-medium" for words, a second voice for
// Phalène/boss/interface); PiperPath defaults to [LookupPiper]'s result
// when empty.
type PiperOptions struct {
	PiperPath string
	Voice     string
}

// SynthesizePiper synthesizes text to outPath (WAV, Piper's own output
// format) with the voice named by opts.Voice — ENCRE_04 §8's fallback for a
// word with no parent recording, and for every Phalène/boss/interface line.
//
// It runs piper through [exec.CommandContext] the same way [Transcode] runs
// ffmpeg: every argument its own slice element, text piped to stdin rather
// than passed as a command-line argument (Piper's own documented interface,
// and one fewer place for an unusual character in a generated justification
// or a boss line to matter). It returns [ErrPiperNotFound] cleanly when
// Piper is absent, exactly like [Transcode] does for ffmpeg — never a
// panic, never a hang.
func SynthesizePiper(ctx context.Context, opts PiperOptions, text, outPath string) error {
	piperPath := opts.PiperPath
	if piperPath == "" {
		found, err := LookupPiper()
		if err != nil {
			return err
		}
		piperPath = found
	}
	if opts.Voice == "" {
		return fmt.Errorf("media: PiperOptions.Voice is required")
	}

	// #nosec G204 -- piperPath comes from LookupPiper (exec.LookPath) or an
	// operator-configured PiperOptions, never from a request; text is sent
	// over stdin, never interpolated into argv or a shell.
	cmd := exec.CommandContext(ctx, piperPath, "--model", opts.Voice, "--output_file", outPath)
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("media: piper failed: %w", err)
	}
	return nil
}
