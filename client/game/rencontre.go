package game

import "github.com/oioio-space/encre/engine"

// IsRencontre reports whether st marks a word this child has never met: the
// card turns face up and is copied rather than recalled (brief/ENCRE_01 §4).
// A nil st — no record at all — is the same as one never played.
func IsRencontre(st *engine.WordState) bool {
	return st == nil || !st.Seen
}

// NewRencontreAttempt builds the [engine.Attempt] a Rencontre records, for
// wordID answered in manche in millis milliseconds.
//
// It always reports Correct: true, because brief/ENCRE_01 §4 is explicit that
// a Rencontre admits no failure — the child is copying a word shown face up,
// not recalling one — and it always sets Copy: true, which is what
// [engine.Record] reads to teach at a fraction of a full success, and what
// [RunScore.Apply] reads to leave the combo untouched rather than advance it.
func NewRencontreAttempt(wordID string, manche int, millis int) engine.Attempt {
	return engine.Attempt{
		WordID:  wordID,
		Manche:  manche,
		Copy:    true,
		Correct: true,
		Millis:  millis,
	}
}
