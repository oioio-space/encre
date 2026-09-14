package game

// FirstMismatch returns the index, in runes, of the first letter where typed
// diverges from target, or -1 if typed is exactly target.
//
// It compares runes rather than bytes so an accented letter — two bytes in
// UTF-8 — is never split in the middle, the same reason [Entry] keeps runes
// rather than a string. A word that is a correct prefix or a correct-with-one-
// extra-letter of the other reports the mismatch at the point it runs out or
// overruns, rather than -1: "écol" against "école" is not the same word.
func FirstMismatch(target, typed string) int {
	t, y := []rune(target), []rune(typed)
	n := min(len(t), len(y))
	for i := range n {
		if t[i] != y[i] {
			return i
		}
	}
	if len(t) != len(y) {
		return n
	}
	return -1
}

// Correction is bead encre-cs5's "correction sans texte" (ENCRE_01 §6): no
// sentence of explanation, only the word written correctly, letter by letter,
// with the one letter the child missed blinking — and the child retypes from
// there.
//
// The zero value reports a correct attempt needing no retype; use
// [NewCorrection] to build one from an actual pair of words.
type Correction struct {
	target   string
	mismatch int
}

// NewCorrection compares typed against target and records where the two
// first diverge, if at all.
func NewCorrection(target, typed string) Correction {
	return Correction{target: target, mismatch: FirstMismatch(target, typed)}
}

// NeedsRetype reports whether typed missed target at all.
func (c Correction) NeedsRetype() bool { return c.mismatch >= 0 }

// MismatchIndex is the rune index of the first letter that blinks, or -1 on a
// correct attempt.
func (c Correction) MismatchIndex() int { return c.mismatch }

// Kept is the prefix of target the child already had right — what stays on
// screen, unblinking, while the mismatched letter on is retyped. It is the
// whole word on a correct attempt.
func (c Correction) Kept() string {
	if c.mismatch < 0 {
		return c.target
	}
	return string([]rune(c.target)[:c.mismatch])
}
