package engine

// TalismanID names one of the twenty Talismans. The first eight are V1; the
// rest are unlocked later (ENCRE_03 §8) and are scored here so that unlocking
// one never needs a change to the rules.
type TalismanID int

// The Talismans, in the order ENCRE_03 §8 lists them.
const (
	Perroquet      TalismanID = iota // +1 mult, flat
	Chronometre                      // +4 mult when the answer came fast
	Jumeau                           // +2 mult per Jumelle trap
	Loupe                            // doubles the chips of the Jumelles
	Gomme                            // forgives the first fault of a manche — see Replay, not here
	Collectionneur                   // +1 mult per gold word already played this run
	Aimant                           // half again on a gold word
	Sourd                            // +1 to the blind multiplier
	Fantome                          // +3 mult per Muette trap
	Horloge
	Bibliothecaire // doubles a gold word
	Colosse        // +5 mult, flat
	Couronne       // doubles the chips of the Accentuées
	Meute          // +2 mult per Accordée trap
	Miroir         // doubles the chips of the Sosies
	Echo
	Phare
	Banquier
	Alchimiste
	Tambour // +6 mult on a cursed word
)

// Talismans is the set a child is carrying into a run. A nil map is a child
// carrying none, which is how every run before the first Échoppe starts.
type Talismans map[TalismanID]bool

// Boss is the modifier one manche runs under (ENCRE_01).
type Boss int

const (
	// NoBoss is an ordinary manche.
	NoBoss Boss = iota - 1
	// Chuchoteur gives one listen only.
	Chuchoteur
	// VoleurDAccents takes the Accentuées to nothing for the manche.
	VoleurDAccents
	// Brouillon shows the word for a second, then wipes it.
	Brouillon
	// Presse puts the manche on a clock.
	Presse
)

// Attempt is one word as the child answered it (ENCRE_04 §4).
type Attempt struct {
	WordID string
	Manche int
	// Blind is a word answered without hearing it, worth BlindMult.
	Blind bool
	// Copy marks a Rencontre: the word is shown, so it counts for neither the
	// combo nor the road to gold.
	Copy    bool
	Correct bool
	Typed   string
	Millis  int
}

// Ctx is the condition one attempt was made under.
type Ctx struct {
	// Levels is the child's level in each Couleur, which is what a trap is
	// worth. A missing Couleur reads as level zero and pays nothing.
	Levels map[Color]int
	// Fast says the answer came inside the Chronomètre's window.
	Fast bool
	// Listens is how many times the word may be heard: two ordinarily, one
	// under the Chuchoteur.
	Listens int
	// Sentence says the word is asked inside a sentence rather than alone.
	Sentence bool
	// Blind is the condition PHat weighs when estimating a blind attempt.
	// Score does not read it: there, the Attempt is the record of what
	// happened, and Attempt.Blind is what counts.
	Blind bool
	// Boss is the modifier of the manche, NoBoss outside one.
	Boss Boss
	// GoldPlayed counts the gold words already played this run, for the
	// Collectionneur.
	GoldPlayed int
}

// doubling maps a Couleur to the Talisman that doubles its chips. Keyed by
// Couleur rather than by Talisman so the lookup is one probe per Couleur
// instead of a walk over a map, whose order would not be stable anyway.
var doubling = map[Color]TalismanID{
	Jumelles:   Loupe,
	Accentuees: Couronne,
	Sosies:     Miroir,
}

// Score returns the chips a correct attempt earns and the multiplier they are
// taken at; the caller keeps chips × mult (ENCRE_04 §4).
//
// The two are separate because the game shows them separately — the counter
// fills with chips and the flame carries the multiplier — and because a child
// reading the screen has to be able to see which of the two a Talisman changed.
//
// A wrong answer scores nothing at all, multiplier included: the combo is
// broken, so there is no multiplier left to apply.
func Score(a Attempt, w Word, st *WordState, owned Talismans, combo float64, ctx Ctx, cfg Config) (chips, mult float64) {
	if !a.Correct {
		return 0, 0
	}
	return scoreChips(a, w, st, owned, ctx, cfg), combo + multBonus(w, st, owned, ctx)
}

// scoreChips is the chips half: the letters of the word, plus what its traps
// pay at the child's level, then the state of the word and the way it was
// played.
func scoreChips(a Attempt, w Word, st *WordState, owned Talismans, ctx Ctx, cfg Config) float64 {
	chips := float64(w.Letters)
	// The Couleurs are walked in their fixed order, never in the map's. Floating
	// addition is not associative, so a map's random order would let the server
	// replay one run and find a different score — and ENCRE_04 §1 makes that
	// recomputation the one that counts.
	for _, colour := range Colors() {
		traps := w.Traps[colour]
		if traps == 0 {
			continue
		}
		// The Voleur d'accents is the only boss that touches the chips: for one
		// manche the Accentuées are worth nothing and the target has to be made
		// on everything else.
		if ctx.Boss == VoleurDAccents && colour == Accentuees {
			continue
		}
		v := float64(traps) * cfg.ChipPerTrap * float64(ctx.Levels[colour])
		if tal, ok := doubling[colour]; ok && owned[tal] {
			v *= 2
		}
		chips += v
	}

	if st.Gold {
		if owned[Aimant] {
			chips *= 1.5
		}
		if owned[Bibliothecaire] {
			chips *= 2
		}
		// A tarnished word is a gold one gone unplayed: it pays double because
		// the point is to pull the child back to it.
		if st.Tarnished {
			chips *= 2
		}
	}
	if st.Cursed {
		chips *= 5
	}
	if a.Blind {
		m := cfg.BlindMult
		if owned[Sourd] {
			m++
		}
		chips *= m
	}
	return chips
}

// multBonus is what the Talismans add on top of the combo.
func multBonus(w Word, st *WordState, owned Talismans, ctx Ctx) float64 {
	m := 0.0
	if owned[Perroquet] {
		m++
	}
	if owned[Chronometre] && ctx.Fast {
		m += 4
	}
	if owned[Jumeau] {
		m += 2 * float64(w.Traps[Jumelles])
	}
	if owned[Fantome] {
		m += 3 * float64(w.Traps[Muettes])
	}
	if owned[Meute] {
		m += 2 * float64(w.Traps[Accordees])
	}
	if owned[Collectionneur] {
		m += float64(ctx.GoldPlayed)
	}
	if owned[Colosse] {
		m += 5
	}
	if owned[Tambour] && st.Cursed {
		m += 6
	}
	return m
}
