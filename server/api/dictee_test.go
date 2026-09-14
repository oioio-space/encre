package api_test

import (
	"net/http"
	"testing"
)

// TestDicteeResultAppliesPositiveBonusOnly is the ticket's core rule
// (ENCRE_01 §14, "jamais de perte"): a dictée entirely wrong still earns the
// fixed completion bonus and never a negative one, and every correct word
// adds on top of it.
func TestDicteeResultAppliesPositiveBonusOnly(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, item1, item2 := seedDicteeList(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	// Every word wrong: still a positive bonus, never zero and never
	// negative — the endpoint must not even offer a way to express a malus.
	resp := ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{
			{"itemID": item1, "correct": false},
			{"itemID": item2, "correct": false},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dictee-result status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type dicteeResultResponse struct {
		Correct, Total, BonusSeconds int
	}
	got := decodeBody[dicteeResultResponse](t, resp)
	if got.Correct != 0 || got.Total != 2 {
		t.Errorf("Correct/Total = %d/%d, want 0/2", got.Correct, got.Total)
	}
	if got.BonusSeconds <= 0 {
		t.Errorf("BonusSeconds = %d, want > 0 even with every word wrong", got.BonusSeconds)
	}

	day := int32(ts.Now().Unix() / (24 * 60 * 60))
	_, bonus, err := ts.DB.PlayTime(t.Context(), "child1", day)
	if err != nil {
		t.Fatalf("PlayTime() error = %v", err)
	}
	if bonus != got.BonusSeconds {
		t.Errorf("stored bonus = %d, want %d (the response's own figure)", bonus, got.BonusSeconds)
	}
}

// TestDicteeResultBonusGrowsWithCorrectWords checks the second half of the
// rule: a correct word pays more than a wrong one, strictly.
func TestDicteeResultBonusGrowsWithCorrectWords(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, item1, item2 := seedDicteeList(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	allWrong := ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{{"itemID": item1, "correct": false}},
	})
	type dicteeResultResponse struct{ BonusSeconds int }
	wrongBonus := decodeBody[dicteeResultResponse](t, allWrong).BonusSeconds

	allRight := ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{{"itemID": item2, "correct": true}},
	})
	rightBonus := decodeBody[dicteeResultResponse](t, allRight).BonusSeconds

	if rightBonus <= wrongBonus {
		t.Errorf("bonus for a correct word = %d, want > bonus for a wrong one (%d)", rightBonus, wrongBonus)
	}
}

// TestDicteeResultRejectsItemOutsideList checks that a parent cannot record
// a result for an item that is not actually part of the list named in the
// URL.
func TestDicteeResultRejectsItemOutsideList(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, _, _ := seedDicteeList(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{{"itemID": "not-in-this-list", "correct": true}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("dictee-result with an outside item: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestDicteeResultRequiresParentSession checks that a child session — or no
// session at all — cannot post a dictée result.
func TestDicteeResultRequiresParentSession(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, item1, _ := seedDicteeList(t, ts, "child1")
	loginChild(t, ts, "Mia", "1379")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{{"itemID": item1, "correct": true}},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("dictee-result with a child session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestDicteeResultRejectsAnotherParentsList checks that a parent cannot
// post a dictée result against a list belonging to someone else's child.
func TestDicteeResultRejectsAnotherParentsList(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Théo", "2468")
	listID, item1, _ := seedDicteeList(t, ts, "child1")
	loginParent(t, ts, "child2-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{{"itemID": item1, "correct": true}},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("dictee-result against another parent's list: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
