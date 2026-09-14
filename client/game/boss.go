package game

import (
	"github.com/oioio-space/encre/content"
	"github.com/oioio-space/encre/engine"
)

// ordinaryListens is how many times a word may be heard outside the
// Chuchoteur (brief/ENCRE_03 §5 / [engine.Ctx.Listens]'s own doc: "two
// ordinarily, one under the Chuchoteur").
const ordinaryListens = 2

// ListensAllowed returns how many times the child may hear the word under
// boss: one for Le Chuchoteur ("le mot n'est dit qu'une fois",
// brief/ENCRE_01 §13), two otherwise. The Perroquet and the Phare Talismans
// add a further listen at the scene that scores the attempt, not here — this
// is the boss's own rule alone.
func ListensAllowed(boss engine.Boss) int {
	if boss == engine.Chuchoteur {
		return 1
	}
	return ordinaryListens
}

// BossLine returns the line the boss named by b says on the black blotter,
// and whether the pack knows it — false for [engine.NoBoss], or a boss the
// pack has not shipped text for yet.
//
// which picks arrival, or the line for the manche's own end: content.Boss.
// Defeat is worded as what the boss says when the child beats it, and
// content.Boss.Victory as what it says when it beats the child instead (see
// [content.Boss]'s own doc) — the opposite of what their field names alone
// suggest, which is why this takes childWon rather than leaving a caller to
// pick the field itself.
func BossLine(pack *content.Pack, b engine.Boss, phase BossLinePhase, childWon bool) (string, bool) {
	boss, ok := pack.Boss(b)
	if !ok {
		return "", false
	}
	switch phase {
	case BossArrival:
		return boss.Arrival, true
	case BossEnd:
		if childWon {
			return boss.Defeat, true
		}
		return boss.Victory, true
	default:
		return "", false
	}
}

// BossLinePhase names which of a boss's lines [BossLine] returns.
type BossLinePhase int

const (
	// BossArrival is the line said when the black blotter arrives.
	BossArrival BossLinePhase = iota
	// BossEnd is the line said when the manche ends, won or lost.
	BossEnd
)
