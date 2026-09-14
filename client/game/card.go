package game

import (
	"time"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/engine"
)

// CardState names one of the states brief/ENCRE_02 §7's table draws the card
// in. It names the child's relationship to the word, not its finish —
// [engine.Shine] (holo, polychrome) is orthogonal and carried separately, since
// a shining word is still gold, tarnished or ordinary underneath.
type CardState int

// The states of brief/ENCRE_02 §7, minus Holo and Polychrome (see
// [CardState]'s own doc).
const (
	// CardNormal is an ordinary word, met before, neither gold nor cursed.
	CardNormal CardState = iota
	// CardFaceDown is the back of the card: in the draw pile, or a Rencontre
	// before it turns.
	CardFaceDown
	// CardRencontre is a word this child has never met, shown face up so it
	// can be copied (brief/ENCRE_01 §4).
	CardRencontre
	// CardGold is a mastered word, kept in the Garde.
	CardGold
	// CardTarnished is a gold word that went unplayed too long
	// (brief/ENCRE_04 §4's TarnishWeeks): still worth restoring, not worth
	// the Garde slot it once held.
	CardTarnished
	// CardCursed is a word failed enough to need three plays in a row to tame.
	CardCursed
)

// DeriveCardState reports how a card carrying st draws, given whether it is
// currently showing its back.
//
// faceDown wins over every other state: what is on top of the draw pile, or a
// Rencontre not yet turned, shows the verso regardless of what is under it.
// Past that, the precedence follows how hard a state is to change back:
// cursed (three correct answers in a row to tame) outranks a first meeting,
// which outranks gold and tarnished — themselves mutually exclusive, since
// tarnishing is a gold word losing that status rather than gaining a second
// one (see [engine.WordState]).
func DeriveCardState(st *engine.WordState, faceDown bool) CardState {
	if faceDown {
		return CardFaceDown
	}
	switch {
	case st.Cursed:
		return CardCursed
	case !st.Seen:
		return CardRencontre
	case st.Gold:
		return CardGold
	case st.Tarnished:
		return CardTarnished
	default:
		return CardNormal
	}
}

// CardFlip is the Rencontre's retournement (brief/ENCRE_06 §5): the verso
// turns to the recto over [anim.Juice.CardFlip]'s duration, scaleX going from
// 1 down to [anim.Juice.CardFlipMinScaleX] and back up, symmetric about the
// midpoint — where the card is thinnest and its edge, not either face, is
// what shows.
//
// It holds only the duration and curve to play by; it owns no clock, the same
// discipline [anim.Juice] itself keeps (see the package doc): a caller drives
// it with elapsed time of its own choosing, which is what lets it be replayed
// at any speed, or not at all, without this type changing.
type CardFlip struct {
	juice anim.Juice
}

// NewCardFlip returns a CardFlip playing by j's CardFlip timing.
func NewCardFlip(j anim.Juice) CardFlip {
	return CardFlip{juice: j}
}

// Duration is how long the whole retournement takes.
func (f CardFlip) Duration() time.Duration {
	return f.juice.CardFlip.Duration.Duration()
}

// ScaleX returns the card's horizontal scale at elapsed into the flip: 1
// before it starts, 1 once it has finished, and eased down to
// [anim.Juice.CardFlipMinScaleX] and back at its midpoint in between — a
// mirrored curve, since the card turns and turns back the same way.
//
// A zero or negative [anim.Juice.CardFlip] curve is never reached: [anim.
// Juice.Validate] refuses one at load time, so the only error left to
// [Curve.Easing] here is a programming one, and ScaleX holds at 1 rather than
// panic on it — a card that fails to flip is a visible bug, not a crashed one.
func (f CardFlip) ScaleX(elapsed time.Duration) float64 {
	total := f.Duration()
	if elapsed <= 0 || elapsed >= total {
		return 1
	}

	easing, err := f.juice.CardFlip.Curve.Easing()
	if err != nil {
		return 1
	}

	// The first half shrinks toward CardFlipMinScaleX, the second grows back:
	// folding elapsed onto [0, half] and mirroring the eased value is what
	// makes the two halves the same curve run forward then backward, rather
	// than two independent easings that would not meet smoothly at the
	// midpoint.
	half := total / 2
	t := float64(elapsed) / float64(half)
	if elapsed > half {
		t = float64(total-elapsed) / float64(half)
	}
	eased := easing.Y(min(max(t, 0), 1))
	return 1 - eased*(1-f.juice.CardFlipMinScaleX)
}

// EdgeVisible reports whether the card is thin enough, at elapsed into the
// flip, that its edge — [anim.Juice.CardFlipEdgeColor] — is what should draw,
// rather than either face. brief/ENCRE_06 §5 calls this "visible à
// mi-course"; here that is any scaleX at or under halfway between 1 and the
// flip's own minimum, which is symmetric around the exact midpoint by
// construction (see [CardFlip.ScaleX]) rather than a single frame of it.
func (f CardFlip) EdgeVisible(elapsed time.Duration) bool {
	threshold := (1 + f.juice.CardFlipMinScaleX) / 2
	return f.ScaleX(elapsed) <= threshold
}
