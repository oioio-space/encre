package game

import (
	"slices"

	"github.com/oioio-space/encre/engine"
)

// gardeSlotsMax is as wide as the Garde ever gets (brief/ENCRE_01 §10:
// "+1 emplacement tous les 2 rangs, max 5").
//
// It mirrors engine's own unexported maxGardeSlots rather than importing it —
// [engine.BuildDeck] does not export the cap, and this package may only touch
// client/ — so [TestGardeSlotsMatchesWhatBuildDeckActuallyGrants] cross-checks
// the two against a live [engine.BuildDeck] call: this constant drifting from
// engine's own would fail there rather than silently offer a slot the deck
// then refuses.
const gardeSlotsMax = 5

// GardeSlots returns how many cards the Garde holds at rank, given cfg's base
// size: one more every two ranks, capped at [gardeSlotsMax].
func GardeSlots(cfg engine.Config, rank int) int {
	return min(cfg.GardeSlots+rank/2, gardeSlotsMax)
}

// GardeFan orders candidates the way the Garde screen offers them for
// choosing (brief/ENCRE_01 §10): the ternies first, with their ×2, since they
// are both the deck-building and the spaced repetition this screen is
// for — a word about to be lost is worth noticing before one already safe.
//
// It sorts a copy; candidates itself is never modified. Within "ternie" and
// "not ternie" the original order is kept, so a caller that already sorted
// candidates some other way (alphabetically, by rarity) keeps that order
// inside each of the two groups.
func GardeFan(candidates []engine.Word, states map[string]*engine.WordState) []engine.Word {
	fan := slices.Clone(candidates)
	tarnished := func(w engine.Word) bool {
		st := states[w.ID]
		return st != nil && st.Tarnished
	}
	slices.SortStableFunc(fan, func(a, b engine.Word) int {
		at, bt := tarnished(a), tarnished(b)
		switch {
		case at && !bt:
			return -1
		case bt && !at:
			return 1
		default:
			return 0
		}
	})
	return fan
}
