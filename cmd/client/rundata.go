package main

import (
	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/engine"
)

// runCard is one card of the demo manche this prototype plays: a word to
// spell, the Couleur its dominant trap belongs to (what the ribbon, the
// glyph and the score label draw with), and the [game.CardState] it starts
// the manche in. It stands in for the deck client/net will deal from a real
// run — wiring that is a later ticket's — so the run screen has something
// true to draw while it waits: real French CE1 words, real traps, real
// Couleurs, only the source is this slice rather than the server.
type runCard struct {
	word  engine.Word
	state game.CardState
	// pHat is a stand-in for [engine.PHat], which needs a whole *engine.Child
	// this prototype does not carry. It is fixed per card rather than
	// computed, and documented as such: wiring the real estimate is part of
	// the deck-loading ticket, not this drawing one.
	pHat float64
}

// dominantColor returns the Couleur runCard's own word carries the most
// traps of, the one the ribbon, the glyph and the score label all draw with
// — brief/ENCRE_02 §7's "ruban de Couleur, teinte de la Couleur dominante".
// A word with no trap at all draws as Muettes, so every card always has a
// Couleur to show.
func (c runCard) dominantColor() engine.Color {
	best, bestN := engine.Muettes, -1
	for _, col := range engine.Colors() {
		if n := c.word.Traps[col]; n > bestN {
			best, bestN = col, n
		}
	}
	return best
}

// tokens is the jetons the card's footer shows (brief/ENCRE_02 §7): one per
// letter, plus a few for every trap the word carries, since a trickier word
// is worth more to have in the Garde.
func (c runCard) tokens() int {
	return c.word.Letters + 4*c.word.TrapCount()
}

// demoManche is the six words of a demo manche (engine.DefaultConfig's own
// WordsPerManche), real CE1 vocabulary carrying the traps ENCRE_03 §4
// teaches — the same words brief/ENCRE_03 gives as examples of each Couleur,
// so the run screen has something true to spell while client/net's real
// deck is wired by a later ticket.
func demoManche() []runCard {
	word := func(id string, letters int, traps map[engine.Color]int, family string) engine.Word {
		return engine.Word{ID: id, Text: id, Letters: letters, Traps: traps, Family: family}
	}
	return []runCard{
		{word: word("chat", 4, map[engine.Color]int{engine.Muettes: 1}, "Fantômes"), state: game.CardNormal, pHat: 0.82},
		{word: word("pomme", 5, map[engine.Color]int{engine.Jumelles: 1}, "Jumeaux"), state: game.CardNormal, pHat: 0.71},
		{word: word("école", 5, map[engine.Color]int{engine.Accentuees: 1}, "Couronnés"), state: game.CardGold, pHat: 0.93},
		{word: word("eau", 3, map[engine.Color]int{engine.Masquees: 1}, "Masqués"), state: game.CardRencontre, pHat: 0.35},
		{word: word("verre", 5, map[engine.Color]int{engine.Sosies: 1}, "Métamorphes"), state: game.CardTarnished, pHat: 0.58},
		{word: word("chats", 5, map[engine.Color]int{engine.Accordees: 1}, "Meutes"), state: game.CardNormal, pHat: 0.64},
	}
}

// demoTarget is the chips this demo manche's cards must reach, computed by
// [engine.Targets] itself against an ordinary Child (rank zero, no rout
// behind them to lower the bar) rather than a number picked to look right on
// the seal. Wiring the child a real run carries — level, rank, kindness — is
// the deck-loading ticket's; this asks the same formula the server scores
// against for what an ordinary child would be asked, so the seal in this
// prototype cracks against a real target rather than an invented one.
func demoTarget(cards []runCard, cfg engine.Config) float64 {
	words := make([]engine.Word, len(cards))
	for i, c := range cards {
		words[i] = c.word
	}
	child := &engine.Child{Kindness: 1}
	deck := engine.Deck{Week: words}
	targets := engine.Targets(child, deck, map[string]*engine.WordState{}, cfg)
	return targets[0]
}
