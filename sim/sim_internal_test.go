package sim

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/oioio-space/encre/engine"
)

// TestShopBuysAV1TalismanWhenTheEchoppeIsOffered is encre-00q.2: sim never set
// run.Talismans, so the shop's whole effect on scoring was never simulated.
// Given money and an Échoppe on the table, the child should leave holding at
// least one Talisman from V1's eight.
func TestShopBuysAV1TalismanWhenTheEchoppeIsOffered(t *testing.T) {
	c := NewChild(rand.New(rand.NewPCG(1, 1)))
	c.Money = 100
	rooms := [2][2]engine.RoomID{
		{engine.Echoppe, engine.Repos},
		{engine.Echoppe, engine.Encrier},
	}
	rng := rand.New(rand.NewPCG(2, 2))

	got := c.shop(rooms, rng)

	if len(got) == 0 {
		t.Fatal("shop returned no Talismans with money and two Échoppe visits, want at least one")
	}
	for _, id := range got {
		if !slices.Contains(v1Talismans, id) {
			t.Errorf("shop bought %v, which is not one of V1's eight", id)
		}
	}
	if c.Money >= 100 {
		t.Errorf("Money = %d after buying, want less than 100", c.Money)
	}
}

// TestShopBuysNothingWithoutMoney checks a child with no money leaves a run
// with nothing, rather than the shop lending on credit.
func TestShopBuysNothingWithoutMoney(t *testing.T) {
	c := NewChild(rand.New(rand.NewPCG(1, 1)))
	c.Money = 0
	rooms := [2][2]engine.RoomID{
		{engine.Echoppe, engine.Repos},
		{engine.Echoppe, engine.Encrier},
	}
	rng := rand.New(rand.NewPCG(2, 2))

	if got := c.shop(rooms, rng); len(got) != 0 {
		t.Errorf("shop = %v with no money, want nothing bought", got)
	}
}

// TestAWordSFirstMeetingIsACopyAttempt is ENCRE_01 §4: the first time a child
// meets a word it is a Rencontre, shown rather than asked, and always
// succeeds. sim never played it before encre-00q.2.
func TestAWordSFirstMeetingIsACopyAttempt(t *testing.T) {
	c := NewChild(rand.New(rand.NewPCG(1, 1)))
	rng := rand.New(rand.NewPCG(3, 3))
	words := newWords(0, rng)

	c.playRun(words, 0, rng, engine.DefaultConfig())

	var sawCopy bool
	for _, a := range c.lastAttempts {
		if a.Copy {
			sawCopy = true
			if !a.Correct {
				t.Errorf("attempt on %q was a Copy but not Correct, want a Rencontre to never fail", a.WordID)
			}
		}
	}
	if !sawCopy {
		t.Error("no attempt was a Copy on a child's very first run, want the new words to be Rencontres")
	}
}
