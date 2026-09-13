// Package engine holds the rules of ENCRE: scoring, the state a word moves
// through, ranks, decks and the replay the server scores a run with.
//
// It is pure Go with no dependency outside the standard library, and the server
// is the authority: the client computes a score to show it, the server
// recomputes it from the attempts and that is the one that counts
// (brief/ENCRE_04 §1). Everything here is therefore deterministic — given the
// same deck seed and the same attempts, twice the same outcome.
package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Config carries every number the rules turn on (brief/ENCRE_04 §4).
//
// The values are not arbitrary: the balance simulation ran 100 children over 36
// weeks through nine versions to reach them, and the first eight killed every
// child before week ten. Treat a change here as a change to whether a
// seven-year-old is still playing in June, and re-run sim/ before keeping it.
type Config struct {
	// WordsPerManche is how many words a manche holds. Six, lowered from eight
	// for a seven-year-old — the change that took retention from 61% to 71%.
	WordsPerManche int
	// K scales the three targets of a run against the deck's value.
	K [3]float64
	// RankMult raises the targets with the rank, Blanc through Diamant.
	RankMult [6]float64
	// RankUpWins is the number of boss wins a rank-up asks for.
	RankUpWins int
	// RankUpMinWeeks is the floor in weeks under the same rank, so a good
	// fortnight cannot carry a child past what they can spell.
	RankUpMinWeeks int
	// RankDownFails is how many failed bosses drop a rank. Without a way down,
	// the simulation had everyone plateau and leave out of boredom.
	RankDownFails int
	// KindnessStep lowers the targets after a run lost early, and
	// KindnessFloor is as far as that goes.
	KindnessStep, KindnessFloor float64
	// BlindTokens is how many words a run may be played blind, and BlindMult
	// what that pays.
	BlindTokens int
	BlindMult   float64
	// ChipPerTrap is the chips a single trap of a Couleur is worth.
	ChipPerTrap float64
	// XPPerLevel is the XP a level of a Couleur costs, multiplied by the level.
	XPPerLevel float64
	// LevelCooldownW is the weeks between two level-ups of one Couleur.
	LevelCooldownW int
	// LevelMax is as high as a Couleur goes.
	LevelMax int
	// GoldDays is the distinct days of success a word needs to turn gold, and
	// GoldMinSpanDays the span they must cover — so gold means remembered, not
	// drilled in one sitting.
	GoldDays, GoldMinSpanDays int
	// TarnishWeeks is how long a gold word may go unplayed before it tarnishes.
	TarnishWeeks int
	// CurseFails and CurseWeeks are the failures, and the weeks they fall in,
	// that curse a word.
	CurseFails, CurseWeeks int
	// HoloOdds and PolyOdds are the chances a word comes back shining.
	HoloOdds, PolyOdds float64
	// RevancheWindow is the share of the target a lost manche must have reached
	// to be worth replaying once.
	RevancheWindow float64
	// RecycleOld is how many older words a week's deck brings back.
	RecycleOld int
	// GardeSlots is the base size of the Garde, before the rank widens it.
	GardeSlots int
	// RunMoney and MancheMoney are what a run and a manche pay.
	RunMoney, MancheMoney int
	// RerollCost is what the Échoppe charges to redraw.
	RerollCost int
	// ChronoSeconds and PresseSeconds are the two clocks of the bosses.
	ChronoSeconds, PresseSeconds float64
	// CahierBonus is the chips the Cahier stores for the next run.
	CahierBonus float64
}

// DefaultConfig returns the configuration of brief/ENCRE_04 §4.
func DefaultConfig() Config {
	return Config{
		WordsPerManche:  6,
		K:               [3]float64{2.2, 5.5, 14},
		RankMult:        [6]float64{1, 1.3, 1.7, 2.2, 2.8, 3.5},
		RankUpWins:      8,
		RankUpMinWeeks:  3,
		RankDownFails:   3,
		KindnessStep:    0.03,
		KindnessFloor:   0.85,
		BlindTokens:     2,
		BlindMult:       3,
		ChipPerTrap:     10,
		XPPerLevel:      12,
		LevelCooldownW:  3,
		LevelMax:        10,
		GoldDays:        3,
		GoldMinSpanDays: 7,
		TarnishWeeks:    4,
		CurseFails:      3,
		CurseWeeks:      3,
		HoloOdds:        1.0 / 8,
		PolyOdds:        1.0 / 40,
		RevancheWindow:  0.85,
		RecycleOld:      4,
		GardeSlots:      3,
		RunMoney:        4,
		MancheMoney:     4,
		RerollCost:      3,
		ChronoSeconds:   10,
		PresseSeconds:   15,
		CahierBonus:     50,
	}
}

// Validate reports the first field left at zero.
//
// Every number here is load-bearing, so none of them has a meaningful zero: a
// zeroed K makes every target unreachable, a zeroed GoldDays turns every word
// gold on its first success. Rather than list the fields — a list that rots the
// day one is added — it walks them, so a field introduced without a default is
// caught the first time this runs.
func (c Config) Validate() error {
	v := reflect.ValueOf(c)
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		if err := noZero(v.Field(i), name); err != nil {
			return err
		}
	}
	return nil
}

// noZero reports an error when f, or any element of it when f is an array, is
// the zero value.
func noZero(f reflect.Value, name string) error {
	if f.Kind() == reflect.Array {
		for i := range f.Len() {
			if f.Index(i).IsZero() {
				return fmt.Errorf("config: %s[%d] is zero, and every value of the config is load-bearing", name, i)
			}
		}
		return nil
	}
	if f.IsZero() {
		return fmt.Errorf("config: %s is zero, and every value of the config is load-bearing", name)
	}
	return nil
}

// LoadConfig reads a configuration from JSON and refuses one that is not whole.
//
// A partial document would silently zero whatever it leaves out, which is the
// failure Validate exists to catch — so loading and validating are one step and
// callers cannot skip the second.
func LoadConfig(raw []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("reading the config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
