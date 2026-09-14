package game_test

import (
	"testing"

	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

func TestDoorsForMancheReturnsTheMatchingPair(t *testing.T) {
	d := engine.Deck{Rooms: [2][2]engine.RoomID{
		{engine.Echoppe, engine.Repos},
		{engine.Encrier, engine.Echoppe},
	}}

	got, err := game.DoorsForManche(d, 0)
	if err != nil {
		t.Fatalf("DoorsForManche(d, 0): %v", err)
	}
	if want := [2]engine.RoomID{engine.Echoppe, engine.Repos}; got != want {
		t.Errorf("DoorsForManche(d, 0) = %v, want %v", got, want)
	}

	got, err = game.DoorsForManche(d, 1)
	if err != nil {
		t.Fatalf("DoorsForManche(d, 1): %v", err)
	}
	if want := [2]engine.RoomID{engine.Encrier, engine.Echoppe}; got != want {
		t.Errorf("DoorsForManche(d, 1) = %v, want %v", got, want)
	}
}

func TestDoorsForMancheRejectsAnythingButZeroOrOne(t *testing.T) {
	d := engine.Deck{}
	for _, manche := range []int{-1, 2, 3} {
		if _, err := game.DoorsForManche(d, manche); err == nil {
			t.Errorf("DoorsForManche(d, %d) = nil error, want one: the boss manche opens no Salle", manche)
		}
	}
}

func TestOtherDoorFindsTheNonÉchoppeDoor(t *testing.T) {
	tests := []struct {
		doors [2]engine.RoomID
		want  engine.RoomID
	}{
		{[2]engine.RoomID{engine.Echoppe, engine.Repos}, engine.Repos},
		{[2]engine.RoomID{engine.Encrier, engine.Echoppe}, engine.Encrier},
	}
	for _, tt := range tests {
		got, ok := game.OtherDoor(tt.doors)
		if !ok {
			t.Fatalf("OtherDoor(%v): no other door found", tt.doors)
		}
		if got != tt.want {
			t.Errorf("OtherDoor(%v) = %v, want %v", tt.doors, got, tt.want)
		}
	}
}

func TestOtherDoorReportsNoneWhenBothAreTheÉchoppe(t *testing.T) {
	if _, ok := game.OtherDoor([2]engine.RoomID{engine.Echoppe, engine.Echoppe}); ok {
		t.Error("OtherDoor(Échoppe, Échoppe) = ok, want not ok: this deck is broken")
	}
}
