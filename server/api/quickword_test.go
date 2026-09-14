package api_test

import (
	"net/http"
	"testing"
)

// TestQuickWordAddsToMesMotsAndIsPlayable checks the ticket's acceptance
// criterion end to end: a word added through quick-word shows up in a run's
// deck, the same way a validated list's item does.
func TestQuickWordAddsToMesMotsAndIsPlayable(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": "chat"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("quick-word status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type quickWordResponse struct{ ItemID string }
	got := decodeBody[quickWordResponse](t, resp)
	if got.ItemID == "" {
		t.Fatal("quick-word: ItemID is empty")
	}

	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)
	found := false
	for _, w := range run.Deck.Week {
		if w.ID == got.ItemID {
			found = true
		}
	}
	if !found {
		t.Errorf("run/start deck = %+v, want it to include the quick word %s", run.Deck.Week, got.ItemID)
	}
}

// TestQuickWordRejectsEmptyText checks that an empty or whitespace-only
// text is refused rather than silently added as a blank word.
func TestQuickWordRejectsEmptyText(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("quick-word with blank text: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestQuickWordRejectsTooLongText checks the length bound: this is meant
// for a single word or a short phrase, not an arbitrary paste.
func TestQuickWordRejectsTooLongText(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginParent(t, ts, "child1-parent")

	long := make([]byte, 200)
	for i := range long {
		long[i] = 'a'
	}
	resp := ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": string(long)})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("quick-word with an overlong text: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestQuickWordRequiresParentSession checks that a child (or no) session
// cannot add a quick word.
func TestQuickWordRequiresParentSession(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginChild(t, ts, "Mia", "1379")

	resp := ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": "chat"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("quick-word with a child session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestQuickWordRejectsAnotherParentsChild checks the ownership rule: a
// parent cannot add a word to a child they do not own.
func TestQuickWordRejectsAnotherParentsChild(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Théo", "2468")
	loginParent(t, ts, "child2-parent")

	resp := ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": "chat"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("quick-word against another parent's child: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestQuickWordTwiceReusesTheSameList checks that repeated quick-word calls
// accumulate into one "mes mots" list rather than creating a fresh one each
// time.
func TestQuickWordTwiceReusesTheSameList(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginParent(t, ts, "child1-parent")

	ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": "chat"})
	ts.post(t, "/api/v1/children/child1/quick-word", map[string]string{"text": "gomme"})

	var listCount int
	if err := ts.DB.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM word_lists WHERE child_id = 'child1'`).Scan(&listCount); err != nil {
		t.Fatalf("counting word lists: %v", err)
	}
	if listCount != 1 {
		t.Errorf("word_lists for child1 = %d, want 1 (one shared 'mes mots' list)", listCount)
	}
}
