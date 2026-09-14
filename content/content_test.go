package content_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/encre/content"
	"github.com/oioio-space/encre/engine"
)

// maxWords is the ceiling ENCRE_03 §4 and §6 put on anything a seven-year-old
// is asked to read on a card or hear from Phalène.
const maxWords = 8

// maxUIWords is the tighter ceiling on an interface label (ENCRE_05 T09).
const maxUIWords = 6

func TestPhaleneSaysSixtyShortLines(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	total := 0
	for context, lines := range p.Phalene {
		if len(lines) == 0 {
			t.Errorf("le contexte %q n'a aucune ligne", context)
		}
		for _, line := range lines {
			total++
			if n := len(strings.Fields(line)); n > maxWords {
				t.Errorf("Phalène/%s : %d mots, maximum %d — %q", context, n, maxWords, line)
			}
		}
	}
	if total != 60 {
		t.Errorf("%d lignes de Phalène, ENCRE_03 §6 en donne 60", total)
	}
}

func TestTalismansAreShortAndComplete(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	if len(p.Talismans) != 20 {
		t.Fatalf("%d Talismans, ENCRE_03 §4 en donne 20", len(p.Talismans))
	}
	for _, ta := range p.Talismans {
		if n := len(strings.Fields(ta.Line)); n > maxWords {
			t.Errorf("%s : %d mots, maximum %d — %q", ta.Name, n, maxWords, ta.Line)
		}
		if n := len(strings.Fields(ta.Flavor)); n > maxWords {
			t.Errorf("%s (saveur) : %d mots, maximum %d — %q", ta.Name, n, maxWords, ta.Flavor)
		}
		if ta.Name == "" || ta.Icon == "" {
			t.Errorf("%q n'a pas de nom ou pas d'icône", ta.ID)
		}
	}
}

// TestEveryEngineTalismanHasItsText is the seam that matters: the engine scores
// twenty Talismans and the child must be able to read all twenty.
func TestEveryEngineTalismanHasItsText(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	for id := engine.Perroquet; id <= engine.Tambour; id++ {
		ta, ok := p.Talisman(id)
		if !ok {
			t.Errorf("le Talisman %d que l'engine sait compter n'a pas de texte", id)
			continue
		}
		if ta.Line == "" {
			t.Errorf("%s n'a pas de ligne", ta.Name)
		}
	}
}

func TestEightTalismansAreOpenAtTheStart(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	open := 0
	for _, ta := range p.Talismans {
		if !ta.Locked {
			open++
		}
	}
	if open != 8 {
		t.Errorf("%d Talismans ouverts, ENCRE_03 §4 en ouvre 8 en V1", open)
	}
}

func TestLockedTalismansNameAnExploitThatExists(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	ids := make([]string, 0, len(p.Exploits))
	for _, e := range p.Exploits {
		ids = append(ids, e.ID)
	}
	for _, ta := range p.Talismans {
		if !ta.Locked {
			continue
		}
		if !slices.Contains(ids, ta.Exploit) {
			t.Errorf("%s se débloque par %q, qui n'est pas un exploit", ta.Name, ta.Exploit)
		}
	}
}

func TestBossesSayTheirThreeLines(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	if len(p.Bosses) != 5 {
		t.Fatalf("%d boss, ENCRE_03 §5 en donne 5", len(p.Bosses))
	}
	open := 0
	for _, b := range p.Bosses {
		if !b.Locked {
			open++
		}
		for label, line := range map[string]string{
			"arrivée": b.Arrival, "défaite": b.Defeat, "victoire": b.Victory,
		} {
			if line == "" {
				t.Errorf("%s n'a pas de ligne d'%s", b.Name, label)
			}
			if n := len(strings.Fields(line)); n > maxWords {
				t.Errorf("%s (%s) : %d mots, maximum %d — %q", b.Name, label, n, maxWords, line)
			}
		}
	}
	if open != 2 {
		t.Errorf("%d boss ouverts, ENCRE_03 §5 en ouvre 2 en V1", open)
	}
}

// TestEngineBossesHaveTheirLines checks the other seam: every modifier the
// engine can put on a manche has a boss that announces it out loud.
func TestEngineBossesHaveTheirLines(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	for _, b := range []engine.Boss{engine.Chuchoteur, engine.Brouillon, engine.VoleurDAccents, engine.Presse} {
		if _, ok := p.Boss(b); !ok {
			t.Errorf("le boss %d n'a pas de texte", b)
		}
	}
	if _, ok := p.Boss(engine.NoBoss); ok {
		t.Error("une manche ordinaire ne devrait annoncer personne")
	}
}

func TestInterfaceLabelsStayUnderSixWords(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	if len(p.UI) == 0 {
		t.Fatal("aucun texte d'interface")
	}
	for key, label := range p.UI {
		if n := len(strings.Fields(label)); n > maxUIWords {
			t.Errorf("%s : %d mots, maximum %d — %q", key, n, maxUIWords, label)
		}
	}
}

func TestSayFallsBackToTheKey(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	if got := p.Say("jouer"); got != "Jouer" {
		t.Errorf("Say(jouer) = %q, want %q", got, "Jouer")
	}
	if got := p.Say("clé_absente"); got != "clé_absente" {
		t.Errorf("Say sur une clé absente = %q, want la clé elle-même", got)
	}
}

func TestLinesReturnsAContextInOrder(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	lines := p.Lines("accueil")
	if len(lines) != 5 || lines[0] != "Bonsoir. La bougie est allumée." {
		t.Errorf("Lines(accueil) = %v, want the five lines of ENCRE_03 §6 in order", lines)
	}
	if got := p.Lines("contexte_absent"); got != nil {
		t.Errorf("Lines sur un contexte absent = %v, want nil", got)
	}
}

func TestUnknownTalismanHasNoText(t *testing.T) {
	t.Parallel()

	if _, ok := content.Embedded().Talisman(engine.TalismanID(99)); ok {
		t.Error("un Talisman que l'engine ne connaît pas a trouvé un texte")
	}
}

func TestExploitsCoverTheTwelveVisibleOnes(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	visible := 0
	for _, e := range p.Exploits {
		if e.Name == "" || e.Reward == "" {
			t.Errorf("l'exploit %q n'a pas de nom ou pas de récompense", e.ID)
		}
		if !e.Hidden {
			visible++
		}
	}
	if visible != 12 {
		t.Errorf("%d exploits visibles, ENCRE_03 §8 en montre 12 en V1", visible)
	}
}

// TestTheTwoCursedExploitsAreDistinct pins down a reading of the brief.
//
// ENCRE_03 §8 lists « dompter une maudite » among the hidden feats, while §4
// unlocks Le Tambour on « dompter 3 maudites ». Read as one feat the two
// contradict each other; read as two they do not, and the child gets a first
// reward for taming one and the Talisman for taming three. The data takes the
// second reading, and this test is where that decision is written down.
func TestTheTwoCursedExploitsAreDistinct(t *testing.T) {
	t.Parallel()

	p := content.Embedded()
	byID := map[string]content.Exploit{}
	for _, e := range p.Exploits {
		byID[e.ID] = e
	}

	one, ok := byID["une_maudite"]
	if !ok || !one.Hidden {
		t.Errorf("une_maudite = %+v, want a hidden exploit", one)
	}
	three, ok := byID["trois_maudites"]
	if !ok || three.Reward != "Le Tambour" {
		t.Errorf("trois_maudites = %+v, want Le Tambour as its reward", three)
	}
	tambour, ok := p.Talisman(engine.Tambour)
	if !ok || tambour.Exploit != "trois_maudites" {
		t.Errorf("Le Tambour opens on %q, want trois_maudites", tambour.Exploit)
	}
}
