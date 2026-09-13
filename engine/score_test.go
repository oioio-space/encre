package engine_test

import (
	"testing"

	"github.com/oioio-space/encre/engine"
)

// levels1 is a child at level 1 in every Couleur, the state of week one.
func levels1() map[engine.Color]int {
	l := map[engine.Color]int{}
	for _, c := range engine.Colors() {
		l[c] = 1
	}
	return l
}

// plain is a five-letter word with no trap: chips are its letters and nothing
// else, which makes it the baseline every other case is measured against.
var plain = engine.Word{ID: "w", Text: "aller", Letters: 5}

func TestChipsStartFromTheLettersOfTheWord(t *testing.T) {
	got, _ := engine.Score(engine.Attempt{Correct: true}, plain, &engine.WordState{},
		nil, 1, engine.Ctx{Levels: levels1()}, engine.DefaultConfig())

	if want := 5.0; got != want {
		t.Errorf("chips for a five-letter word with no trap = %v, want %v", got, want)
	}
}

func TestEachTrapPaysItsCouleurAtTheChildsLevel(t *testing.T) {
	cfg := engine.DefaultConfig()

	for _, colour := range engine.Colors() {
		for _, level := range []int{1, 10} {
			w := plain
			w.Traps = map[engine.Color]int{colour: 2}
			levels := levels1()
			levels[colour] = level

			got, _ := engine.Score(engine.Attempt{Correct: true}, w, &engine.WordState{},
				nil, 1, engine.Ctx{Levels: levels}, cfg)

			want := 5 + 2*cfg.ChipPerTrap*float64(level)
			if got != want {
				t.Errorf("%v at level %d: chips = %v, want %v", colour, level, got, want)
			}
		}
	}
}

func TestTheAccentThiefTakesTheAccentuatedTrapsToNothing(t *testing.T) {
	// The boss of ENCRE_01: for one manche the Accentuées pay nothing, and the
	// child has to earn the target on the other Couleurs.
	cfg := engine.DefaultConfig()
	w := plain
	w.Traps = map[engine.Color]int{engine.Accentuees: 3, engine.Muettes: 1}

	got, _ := engine.Score(engine.Attempt{Correct: true}, w, &engine.WordState{},
		nil, 1, engine.Ctx{Levels: levels1(), Boss: engine.VoleurDAccents}, cfg)

	// Only the single Muette trap survives.
	if want := 5 + cfg.ChipPerTrap; got != want {
		t.Errorf("chips under the Voleur d'accents = %v, want %v", got, want)
	}
}

func TestTheStateOfTheWordMultipliesTheChips(t *testing.T) {
	cfg := engine.DefaultConfig()

	tests := []struct {
		name  string
		state engine.WordState
		owned engine.Talismans
		want  float64
	}{
		{name: "an ordinary word", want: 5},
		{
			name:  "a cursed word pays five times",
			state: engine.WordState{Cursed: true},
			want:  25,
		},
		{
			name:  "a gold word tarnished pays twice",
			state: engine.WordState{Gold: true, Tarnished: true},
			want:  10,
		},
		{
			name:  "the Aimant pays half again on gold",
			state: engine.WordState{Gold: true},
			owned: engine.Talismans{engine.Aimant: true},
			want:  7.5,
		},
		{
			name:  "the Bibliothécaire doubles gold",
			state: engine.WordState{Gold: true},
			owned: engine.Talismans{engine.Bibliothecaire: true},
			want:  10,
		},
		{
			name:  "neither touches a word that is not gold",
			owned: engine.Talismans{engine.Aimant: true, engine.Bibliothecaire: true},
			want:  5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := engine.Score(engine.Attempt{Correct: true}, plain, &tt.state,
				tt.owned, 1, engine.Ctx{Levels: levels1()}, cfg)
			if got != tt.want {
				t.Errorf("chips = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlayingBlindTriplesTheChipsAndTheSourdMakesItFour(t *testing.T) {
	cfg := engine.DefaultConfig()

	blind := engine.Attempt{Correct: true, Blind: true}
	got, _ := engine.Score(blind, plain, &engine.WordState{}, nil, 1,
		engine.Ctx{Levels: levels1()}, cfg)
	if want := 5 * cfg.BlindMult; got != want {
		t.Errorf("blind chips = %v, want %v", got, want)
	}

	got, _ = engine.Score(blind, plain, &engine.WordState{},
		engine.Talismans{engine.Sourd: true}, 1, engine.Ctx{Levels: levels1()}, cfg)
	if want := 5 * (cfg.BlindMult + 1); got != want {
		t.Errorf("blind chips with the Sourd = %v, want %v", got, want)
	}
}

func TestTheMultiplierStartsAtTheComboAndEachTalismanAddsToIt(t *testing.T) {
	cfg := engine.DefaultConfig()
	w := plain
	w.Traps = map[engine.Color]int{engine.Jumelles: 2, engine.Muettes: 1, engine.Accordees: 3}

	tests := []struct {
		name  string
		owned engine.Talismans
		ctx   engine.Ctx
		state engine.WordState
		want  float64 // on top of a combo of 1
	}{
		{name: "nothing carried", want: 1},
		{name: "Perroquet", owned: engine.Talismans{engine.Perroquet: true}, want: 2},
		{
			name:  "Chronomètre only when the answer came fast",
			owned: engine.Talismans{engine.Chronometre: true},
			want:  1,
		},
		{
			name:  "Chronomètre inside its window",
			owned: engine.Talismans{engine.Chronometre: true},
			ctx:   engine.Ctx{Fast: true},
			want:  5,
		},
		{name: "Jumeau pays two per Jumelle", owned: engine.Talismans{engine.Jumeau: true}, want: 5},
		{name: "Fantôme pays three per Muette", owned: engine.Talismans{engine.Fantome: true}, want: 4},
		{name: "Meute pays two per Accordée", owned: engine.Talismans{engine.Meute: true}, want: 7},
		{
			name:  "Collectionneur pays one per gold word already played",
			owned: engine.Talismans{engine.Collectionneur: true},
			ctx:   engine.Ctx{GoldPlayed: 3},
			want:  4,
		},
		{name: "Colosse", owned: engine.Talismans{engine.Colosse: true}, want: 6},
		{
			name:  "Tambour only on a cursed word",
			owned: engine.Talismans{engine.Tambour: true},
			state: engine.WordState{Cursed: true},
			want:  7,
		},
		{name: "Tambour on an ordinary word", owned: engine.Talismans{engine.Tambour: true}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			ctx.Levels = levels1()
			_, got := engine.Score(engine.Attempt{Correct: true}, w, &tt.state, tt.owned, 1, ctx, cfg)
			if got != tt.want {
				t.Errorf("mult = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTheComboIsTheFloorOfTheMultiplier(t *testing.T) {
	// score is chips × mult and the combo counts the words already spelled
	// right this manche, so a long run of them is worth more than any talisman.
	for _, combo := range []float64{1, 4, 9} {
		_, got := engine.Score(engine.Attempt{Correct: true}, plain, &engine.WordState{},
			engine.Talismans{engine.Perroquet: true}, combo,
			engine.Ctx{Levels: levels1()}, engine.DefaultConfig())
		if want := combo + 1; got != want {
			t.Errorf("mult at combo %v = %v, want %v", combo, got, want)
		}
	}
}

func TestAWrongAnswerScoresNothing(t *testing.T) {
	chips, mult := engine.Score(engine.Attempt{Correct: false}, plain, &engine.WordState{},
		engine.Talismans{engine.Perroquet: true}, 5, engine.Ctx{Levels: levels1()}, engine.DefaultConfig())

	if chips != 0 || mult != 0 {
		t.Errorf("Score of a wrong answer = (%v, %v), want (0, 0)", chips, mult)
	}
}

func TestTheLoupeCouronneAndMiroirDoubleTheirOwnCouleur(t *testing.T) {
	cfg := engine.DefaultConfig()
	pairs := []struct {
		tal    engine.TalismanID
		colour engine.Color
	}{
		{engine.Loupe, engine.Jumelles},
		{engine.Couronne, engine.Accentuees},
		{engine.Miroir, engine.Sosies},
	}
	for _, p := range pairs {
		w := plain
		w.Traps = map[engine.Color]int{p.colour: 1}

		bare, _ := engine.Score(engine.Attempt{Correct: true}, w, &engine.WordState{}, nil, 1,
			engine.Ctx{Levels: levels1()}, cfg)
		with, _ := engine.Score(engine.Attempt{Correct: true}, w, &engine.WordState{},
			engine.Talismans{p.tal: true}, 1, engine.Ctx{Levels: levels1()}, cfg)

		if want := bare + cfg.ChipPerTrap; with != want {
			t.Errorf("%v on a %v word: chips = %v, want %v", p.tal, p.colour, with, want)
		}
	}
}
