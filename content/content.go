// Package content holds the words the game says: the Talismans, the boss lines,
// the sixty lines of Phalène, the exploits and the interface labels of
// ENCRE_03 §4 to §8.
//
// It is data, embedded as JSON and read once. Nothing here decides anything —
// the rules live in engine, and what is written here is only ever shown, said
// or read aloud. Keeping the two apart is what lets the texts be rewritten, and
// the voices re-synthesised, without touching a rule.
package content

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/oioio-space/encre/engine"
)

//go:embed data/*.json
var files embed.FS

// Talisman is one of the twenty of ENCRE_03 §4.
//
// Line is what the child reads on the card, eight words at most. Flavor is
// never written: Phalène says it when the speaker is tapped (ENCRE_03 §11).
type Talisman struct {
	// ID is the written name the game keys the Talisman by, and what
	// Pack.Talisman matches against the engine's own id.
	ID string `json:"id"`
	// Icon names the sprite drawn on the card.
	Icon string `json:"icone"`
	// Name is what the child reads at the top of the card.
	Name string `json:"nom"`
	// Line is the effect, in eight words at most.
	Line string `json:"ligne"`
	// Flavor is never written on screen: Phalène says it when the speaker is
	// tapped, so that flavour costs no reading (ENCRE_03 §11).
	Flavor string `json:"saveur"`
	// Locked marks the twelve that ENCRE_03 §4 keeps for V2, each behind the
	// Exploit named here.
	Locked bool `json:"verrouille"`
	// Exploit is the id of the Exploit that opens it, empty when it is open
	// from the start.
	Exploit string `json:"exploit"`
}

// Boss is one of the five of ENCRE_03 §5, with the three lines it says: on
// arriving, on losing, and on winning.
type Boss struct {
	// ID is the written name the game keys the boss by.
	ID string `json:"id"`
	// Name is what is written on the black blotter when it arrives.
	Name string `json:"nom"`
	// Arrival is what it says on arriving.
	Arrival string `json:"arrivee"`
	// Defeat is what it says when the child beats it.
	Defeat string `json:"defaite"`
	// Victory is what it says when it beats the child. It never mocks: at
	// seven, losing has to stay survivable (ENCRE_01, profil du joueur).
	Victory string `json:"victoire"`
	// Locked marks the three ENCRE_03 §5 keeps for V2.
	Locked bool `json:"verrouille"`
}

// Exploit is one of the feats of ENCRE_03 §8. A hidden one shows as ??? in the
// gallery until the child does it.
type Exploit struct {
	// ID is the written name Talisman.Exploit points at.
	ID string `json:"id"`
	// Name is the feat, as the gallery shows it.
	Name string `json:"nom"`
	// Reward is what doing it opens.
	Reward string `json:"recompense"`
	Hidden bool   `json:"cache"`
}

// Pack is everything the game says.
type Pack struct {
	// Talismans are the twenty of ENCRE_03 §4, open ones first.
	Talismans []Talisman
	// Bosses are the five of ENCRE_03 §5.
	Bosses []Boss
	// Exploits are the feats of ENCRE_03 §8, visible ones first.
	Exploits []Exploit
	// Phalene holds her lines by context: accueil, faute, combo_haut…
	Phalene map[string][]string
	// UI holds the interface labels by key, six words at most each.
	UI map[string]string
}

var (
	once   sync.Once
	pack   *Pack
	loaded error
)

// Embedded returns the texts built into the binary, parsed once and shared.
//
// It panics if they cannot be read, because that is a broken build rather than
// a runtime condition: the JSON ships inside the binary, and this package's own
// tests read every line of it.
func Embedded() *Pack {
	once.Do(func() { pack, loaded = load() })
	if loaded != nil {
		panic(fmt.Sprintf("content: embedded texts unreadable: %v", loaded))
	}
	return pack
}

func load() (*Pack, error) {
	p := &Pack{}
	for name, into := range map[string]any{
		"talismans.json": &p.Talismans,
		"boss.json":      &p.Bosses,
		"exploits.json":  &p.Exploits,
		"phalene.json":   &p.Phalene,
		"interface.json": &p.UI,
	} {
		b, err := files.ReadFile("data/" + name)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, into); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	return p, nil
}

// talismanIDs ties a written Talisman to the rule that scores it. It is the one
// place the texts and the engine meet, and this package's tests fail if either
// side gains a Talisman the other has never heard of.
var talismanIDs = map[string]engine.TalismanID{
	"perroquet": engine.Perroquet, "chronometre": engine.Chronometre,
	"jumeau": engine.Jumeau, "loupe": engine.Loupe, "gomme": engine.Gomme,
	"collectionneur": engine.Collectionneur, "aimant": engine.Aimant,
	"sourd": engine.Sourd, "fantome": engine.Fantome, "horloge": engine.Horloge,
	"bibliothecaire": engine.Bibliothecaire, "colosse": engine.Colosse,
	"couronne": engine.Couronne, "meute": engine.Meute, "miroir": engine.Miroir,
	"echo": engine.Echo, "phare": engine.Phare, "banquier": engine.Banquier,
	"alchimiste": engine.Alchimiste, "tambour": engine.Tambour,
}

// bossIDs ties a written boss to the modifier the engine puts on the manche.
// Le Miroir is absent on purpose: it asks a question about Sosies that ENCRE_01
// leaves to V2, so it carries its lines and no rule yet.
var bossIDs = map[string]engine.Boss{
	"chuchoteur": engine.Chuchoteur, "brouillon": engine.Brouillon,
	"voleur_accents": engine.VoleurDAccents, "presse": engine.Presse,
}

// Talisman returns the texts of the Talisman the engine knows by this id.
func (p *Pack) Talisman(id engine.TalismanID) (Talisman, bool) {
	for _, t := range p.Talismans {
		if got, ok := talismanIDs[t.ID]; ok && got == id {
			return t, true
		}
	}
	return Talisman{}, false
}

// Boss returns the texts of the boss that applies this modifier. An ordinary
// manche — engine.NoBoss — announces nobody.
func (p *Pack) Boss(b engine.Boss) (Boss, bool) {
	for _, got := range p.Bosses {
		if id, ok := bossIDs[got.ID]; ok && id == b {
			return got, true
		}
	}
	return Boss{}, false
}

// Lines returns everything Phalène can say in one context, in the order
// ENCRE_03 §6 lists them. The choice is never made here: a run has to stay
// reproducible from its seed, so the caller draws with the run's own RNG.
func (p *Pack) Lines(context string) []string { return p.Phalene[context] }

// Say returns an interface label. An unknown key comes back as itself, which is
// visible on screen and so impossible to miss while testing.
func (p *Pack) Say(key string) string {
	if s, ok := p.UI[key]; ok {
		return s
	}
	return key
}
