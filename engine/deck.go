package engine

import (
	"math/rand/v2"
	"slices"
)

// The rooms offered between two manches (ENCRE_01).
const (
	// Echoppe is the shop. It is on the table at every transition, because it
	// is never the thing a child should gamble away.
	Echoppe RoomID = "Échoppe"
	// Repos gives the child a breath and a little money.
	Repos RoomID = "Repos"
	// Encrier offers a word back from the Garde.
	Encrier RoomID = "Encrier"
	// Rencontre shows a word instead of asking for it.
	RencontreRoom RoomID = "Rencontre"
)

// alternatives are what the Échoppe is offered against.
var alternatives = []RoomID{Repos, Encrier, RencontreRoom}

// bosses are the four of V1 and V2, in the order the weeks meet them.
var bosses = []BossID{"Chuchoteur", "Voleur d'accents", "Brouillon", "Presse"}

// maxGardeSlots is as wide as the Garde ever gets (ENCRE_04 §4).
const maxGardeSlots = 5

// BuildDeck assembles what a run is played from: this week's words, the Garde
// the child chose to keep, a few older words brought back, and anything cursed.
//
// It takes the seed rather than a generator, and records it in Deck.Seed: that
// is exactly what Replay needs to rebuild the same deck on the server, and it
// makes the determinism of ENCRE_04 §4 something a caller can see in the
// signature instead of having to trust. The boss, by contrast, comes from the
// week and ignores the seed — a child and a parent should be able to know on
// Monday what Friday holds.
func BuildDeck(c *Child, week, known []Word, states map[string]*WordState, garde []string, weekNo int32, seed uint64, cfg Config) Deck {
	// A deck has to be rebuildable from its seed, which is the one thing
	// crypto/rand cannot do: the server has to draw what the child drew.
	// #nosec G404 -- reproducibility is the requirement; this is not a secret.
	draw := rand.New(rand.NewPCG(seed, 0x5ec0))

	state := func(id string) *WordState {
		if st := states[id]; st != nil {
			return st
		}
		return &WordState{}
	}

	d := Deck{Week: week, Seed: seed, Boss: bosses[int(weekNo)%len(bosses)]}

	// The Garde is the child's own shelf of gold words, capped by the rank: one
	// more slot per two ranks, never past five.
	slots := min(cfg.GardeSlots+c.Rank/2, maxGardeSlots)
	// Indexed once rather than searched per entry: a child's known words run to
	// hundreds by June, and a linear scan for each of the five Garde slots made
	// this the most expensive thing in a run.
	byID := make(map[string]Word, len(known))
	for _, w := range known {
		byID[w.ID] = w
	}
	for _, id := range garde {
		if w, ok := byID[id]; ok {
			d.Garde = append(d.Garde, w)
		}
	}
	// A tarnished word goes to the head: it is the one the child has stopped
	// meeting, and the one worth pulling back.
	slices.SortStableFunc(d.Garde, func(a, b Word) int {
		at, bt := state(a.ID).Tarnished, state(b.ID).Tarnished
		switch {
		case at && !bt:
			return -1
		case bt && !at:
			return 1
		default:
			return 0
		}
	})
	if len(d.Garde) > slots {
		d.Garde = d.Garde[:slots]
	}

	// Older words come back to be practised, so a gold one never takes a slot:
	// it is already remembered, and the place belongs to a word that is not.
	inGarde := make(map[string]bool, len(garde))
	for _, id := range garde {
		inGarde[id] = true
	}
	var pool []Word
	for _, w := range known {
		st := state(w.ID)
		switch {
		case st.Cursed:
			d.Cursed = append(d.Cursed, w)
		case st.Seen && !st.Gold && !inGarde[w.ID]:
			pool = append(pool, w)
		}
	}
	draw.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	d.Old = pool[:min(len(pool), cfg.RecycleOld)]

	for i := range d.Rooms {
		other := alternatives[draw.IntN(len(alternatives))]
		// The Échoppe takes either side, so the child does not learn to tap the
		// same corner without reading.
		if draw.IntN(2) == 0 {
			d.Rooms[i] = [2]RoomID{Echoppe, other}
		} else {
			d.Rooms[i] = [2]RoomID{other, Echoppe}
		}
	}
	return d
}
