package parent

import (
	"strings"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
	"github.com/oioio-space/encre/server/store"
)

func TestBuildItemsWord(t *testing.T) {
	lex := lexique.Embedded()
	items, err := buildItems(lex, "list1", kindWord, "chat, chien\nsouris")
	if err != nil {
		t.Fatalf("buildItems() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	for _, it := range items {
		if it.ListID != "list1" {
			t.Errorf("item %q: ListID = %q, want list1", it.Text, it.ListID)
		}
		if it.Kind != engine.KindWord {
			t.Errorf("item %q: Kind = %v, want KindWord", it.Text, it.Kind)
		}
		if it.Source != sourceLexique {
			t.Errorf("item %q: Source = %d, want sourceLexique", it.Text, it.Source)
		}
	}
	chat := findItem(t, items, "chat")
	if chat.Colors[engine.Muettes] != 1 {
		t.Errorf("chat Colors[Muettes] = %d, want 1", chat.Colors[engine.Muettes])
	}
	if chat.Confirmed {
		t.Errorf("chat.Confirmed = true, want false: buildItems must never pre-confirm an item")
	}
	if chat.ID == "" {
		t.Errorf("chat.ID is empty, want a generated ID")
	}
}

func TestBuildItemsWordBlanksAreDropped(t *testing.T) {
	lex := lexique.Embedded()
	items, err := buildItems(lex, "list1", kindWord, "chat\n\n  \nchien")
	if err != nil {
		t.Fatalf("buildItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
}

func TestBuildItemsWordUnknownWordIsUnsure(t *testing.T) {
	lex := lexique.Embedded()
	items, err := buildItems(lex, "list1", kindWord, "zorglubaxatron")
	if err != nil {
		t.Fatalf("buildItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Confidence >= lexique.UnsureConfidence {
		t.Errorf("Confidence = %v, want < %v (unsure)", items[0].Confidence, lexique.UnsureConfidence)
	}
}

func TestBuildItemsSentenceTargetMarkedWithAsterisks(t *testing.T) {
	lex := lexique.Embedded()
	items, err := buildItems(lex, "list1", kindSentence, "Le *chat* dort sur le lit.")
	if err != nil {
		t.Fatalf("buildItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	got := items[0]
	if strings.Contains(got.Text, "*") {
		t.Errorf("Text = %q, still contains the target markers", got.Text)
	}
	if !strings.Contains(got.Text, "chat") {
		t.Errorf("Text = %q, want it to contain chat", got.Text)
	}
	if len(got.Targets) != 1 {
		t.Fatalf("len(Targets) = %d, want 1", len(got.Targets))
	}
	wantStart := strings.Index(got.Text, "chat")
	if got.Targets[0].Start != wantStart || got.Targets[0].End != wantStart+4 {
		t.Errorf("Targets[0] = %+v, want {%d %d}", got.Targets[0], wantStart, wantStart+4)
	}
	if got.Kind != engine.KindSentence {
		t.Errorf("Kind = %v, want KindSentence", got.Kind)
	}
}

func TestBuildItemsSentenceRequiresATarget(t *testing.T) {
	lex := lexique.Embedded()
	if _, err := buildItems(lex, "list1", kindSentence, "Le chat dort."); err == nil {
		t.Error("buildItems() with no *target*: want error, got nil")
	}
}

func TestBuildItemsDictationSplitsOnPunctuation(t *testing.T) {
	lex := lexique.Embedded()
	items, err := buildItems(lex, "list1", kindDictation, "Le chat dort. Il ronronne fort !")
	if err != nil {
		t.Fatalf("buildItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2: %+v", len(items), items)
	}
	if items[0].Text != "Le chat dort." {
		t.Errorf("items[0].Text = %q, want %q", items[0].Text, "Le chat dort.")
	}
	if items[1].Text != "Il ronronne fort !" {
		t.Errorf("items[1].Text = %q, want %q", items[1].Text, "Il ronronne fort !")
	}
	for _, it := range items {
		if it.Kind != engine.KindDictation {
			t.Errorf("item %q: Kind = %v, want KindDictation", it.Text, it.Kind)
		}
	}
}

func TestBuildItemsRejectsUnknownKind(t *testing.T) {
	lex := lexique.Embedded()
	if _, err := buildItems(lex, "list1", "mystere", "chat"); err == nil {
		t.Error("buildItems() with an unknown kind: want error, got nil")
	}
}

func TestBuildItemsRejectsBlankInput(t *testing.T) {
	lex := lexique.Embedded()
	if _, err := buildItems(lex, "list1", kindWord, "   \n  "); err == nil {
		t.Error("buildItems() with blank rawText: want error, got nil")
	}
}

func findItem(t *testing.T, items []*store.Item, text string) *store.Item {
	t.Helper()
	for _, it := range items {
		if it.Text == text {
			return it
		}
	}
	t.Fatalf("no item with Text = %q in %+v", text, items)
	return nil
}
