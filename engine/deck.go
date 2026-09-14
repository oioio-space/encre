package engine

import (
	"cmp"
	"math"
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

// Draw picks a manche's words from the deck, nearest a target chance of
// success, rather than mixing them uniformly (encre-00q.3).
//
// The optimal error rate for learning is measured, not guessed: Wilson,
// Shenhav, Straccia & Cohen (Nature Communications, 2019) put it at 15.87%,
// and FSRS-6, trained on roughly 700 million Anki reviews, converges on the
// same 0.85-0.90 band independently. A deck mixed uniformly instead spends
// most of a manche on words either already safe or still out of reach, and
// neither teaches as much as one sitting near the edge of what the child can
// do.
//
// A word of the week the child has not yet met is pedagogy, not
// optimisation — a teacher's list is followed, not routed around — so it is
// seated by a plain round robin and the sort never touches it. Once it has
// had its Rencontre, though, it plays by the same rule as everything else:
// a week that forced the whole list into every manche, all week, spent most
// of it on words either far too hard or already answered, and left almost
// no room for the targeting below to do anything.
//
// Everything else — the rest of the week, the Garde, the words brought back,
// and the maudites — is ranked by how close PHat puts it to cfg.TargetPHat.
//
// A maudite stays a candidate for every manche, which is what keeps it from
// being quietly filtered out for being too hard — but it is not force-fed
// either: an early version seated every maudite every time, and a child
// whose maudites sit near 40% correct never once strung together the
// consecutive successes [Record] asks for to lift the curse, so the count
// only grew. Ranked by TargetPHat alongside the Garde, a maudite is played
// when it is at a chance worth playing rather than on every single manche.
//
// PHat is evaluated under ctx, the context of the manche actually being
// drawn, and that is not incidental: ranking a boss manche's pool outside the
// Boss it will be played under measures a chance the child will not
// actually have, and sends the boss's own failure rate far past what
// cfg.TargetPHat was meant to hold it to.
func Draw(c *Child, d Deck, states map[string]*WordState, manche int, ctx Ctx, cfg Config) []Word {
	state := func(id string) *WordState {
		if st := states[id]; st != nil {
			return st
		}
		return &WordState{}
	}

	var unmet, met []Word
	for _, w := range d.Week {
		if state(w.ID).Seen {
			met = append(met, w)
		} else {
			unmet = append(unmet, w)
		}
	}

	mandatory := tourniquet(unmet, manche)
	if len(mandatory) > cfg.WordsPerManche {
		mandatory = mandatory[:cfg.WordsPerManche]
	}
	need := cfg.WordsPerManche - len(mandatory)
	if need <= 0 {
		return mandatory
	}

	optional := slices.Concat(met, d.Garde, d.Old, d.Cursed)
	slices.SortFunc(optional, func(a, b Word) int {
		return cmp.Compare(
			math.Abs(PHat(c, a, state(a.ID), ctx)-cfg.TargetPHat),
			math.Abs(PHat(c, b, state(b.ID), ctx)-cfg.TargetPHat),
		)
	})

	// slices.Concat rather than append: appending to mandatory would write into
	// its backing array whenever it has spare capacity, quietly corrupting the
	// caller's slice.
	drawn := slices.Concat(mandatory, tourniquet(optional, manche))
	// A pool too thin to fill the manche once round is topped up from the
	// same ranking rather than left short: a child with barely enough known
	// words to go around still plays a full manche, the way the shuffle it
	// replaces did by wrapping its index.
	for i := 0; len(drawn) < cfg.WordsPerManche && len(optional) > 0; i++ {
		drawn = append(drawn, optional[i%len(optional)])
	}
	if len(drawn) > cfg.WordsPerManche {
		drawn = drawn[:cfg.WordsPerManche]
	}
	return drawn
}

// tourniquet returns every third word of ws starting at manche: the round
// robin that spreads a list evenly across the three manches, so none of them
// is handed either the best of a sorted pool or all of a mandatory one while
// the others get none.
func tourniquet(ws []Word, manche int) []Word {
	var out []Word
	for i := manche; i < len(ws); i += manches {
		out = append(out, ws[i])
	}
	return out
}
