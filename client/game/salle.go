package game

import (
	"fmt"

	"github.com/oioio-space/encre/engine"
)

// DoorsForManche returns the two rooms offered after completedManche — the
// index of the manche a Run just finished, 0 or 1 — matching brief/ENCRE_01
// §3's own table: a Salle sits between manche 1 and manche 2, and between
// manche 2 and manche 3, never after the boss.
//
// [engine.Deck.Rooms] carries the draw itself, already keyed the same way;
// this exists so a Salle scene reads "the room after this manche" rather than
// reaching into the deck by an index it has to get right by hand — the off-
// by-one a raw d.Rooms[completedManche] invites, since nothing else names
// what that index counts.
func DoorsForManche(d engine.Deck, completedManche int) ([2]engine.RoomID, error) {
	if completedManche < 0 || completedManche > 1 {
		return [2]engine.RoomID{}, fmt.Errorf(
			"game: no Salle follows manche %d; only manches 0 and 1 lead to one", completedManche,
		)
	}
	return d.Rooms[completedManche], nil
}

// OtherDoor returns the door in doors that is not the Échoppe, and whether
// one was found. brief/ENCRE_01 §5 guarantees the Échoppe is always one of
// the two — [engine.BuildDeck] is what enforces that — so a doors pair with
// no other door is a broken deck, not a shape a Salle scene should silently
// draw around.
func OtherDoor(doors [2]engine.RoomID) (engine.RoomID, bool) {
	switch {
	case doors[0] != engine.Echoppe:
		return doors[0], true
	case doors[1] != engine.Echoppe:
		return doors[1], true
	default:
		return "", false
	}
}
