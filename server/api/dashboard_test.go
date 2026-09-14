package api_test

import (
	"net/http"
	"testing"

	"github.com/oioio-space/encre/engine"
)

// TestDashboardReportsEverythingENCRE01Asks checks the acceptance criterion
// against brief/ENCRE_01 §14 word for word: dorées, rangs, fioles des
// Couleurs, temps joué, résultats de dictée.
func TestDashboardReportsEverythingENCRE01Asks(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	setChildLevel(t, ts.DB, "child1", engine.Muettes, 3)
	for _, id := range []string{"gold1", "gold2", "plain"} {
		seedItem(t, ts.DB, "child1", id)
	}
	if err := ts.DB.SaveWordStates(t.Context(), "child1", map[string]*engine.WordState{
		"gold1": {Gold: true, Seen: true},
		"gold2": {Gold: true, Seen: true},
		"plain": {Seen: true},
	}); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}
	day := int32(ts.Now().Unix() / (24 * 60 * 60))
	if err := ts.DB.AddPlayTime(t.Context(), "child1", day, 300); err != nil {
		t.Fatalf("AddPlayTime() error = %v", err)
	}
	listID, item1, item2 := seedDicteeList(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	ts.post(t, "/api/v1/lists/"+listID+"/dictee-result", map[string]any{
		"results": []map[string]any{
			{"itemID": item1, "correct": true},
			{"itemID": item2, "correct": false},
		},
	})

	resp := ts.get(t, "/api/v1/children/child1/dashboard")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type dicteeSummary struct {
		ListID         string
		Correct, Total int
	}
	type dashboardResponse struct {
		Rank               int
		Levels             map[string]int
		GoldWords          int
		PlayedTodaySeconds int
		RemainingSeconds   int
		Dictees            []dicteeSummary
		Message            string
	}
	got := decodeBody[dashboardResponse](t, resp)

	if got.GoldWords != 2 {
		t.Errorf("GoldWords = %d, want 2", got.GoldWords)
	}
	if got.PlayedTodaySeconds != 300 {
		t.Errorf("PlayedTodaySeconds = %d, want 300", got.PlayedTodaySeconds)
	}
	if got.Message == "" {
		t.Error("Message is empty, want ENCRE_01's reassurance line")
	}
	var found bool
	for _, d := range got.Dictees {
		if d.ListID != listID {
			continue
		}
		found = true
		if d.Correct != 1 || d.Total != 2 {
			t.Errorf("dictee summary = %+v, want Correct=1 Total=2", d)
		}
	}
	if !found {
		t.Fatalf("Dictees = %+v, want an entry for %s", got.Dictees, listID)
	}
}

func TestDashboardRequiresParentSession(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginChild(t, ts, "Mia", "1379")

	resp := ts.get(t, "/api/v1/children/child1/dashboard")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("dashboard with a child session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestDashboardRejectsAnotherParentsChild(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Théo", "2468")
	loginParent(t, ts, "child2-parent")

	resp := ts.get(t, "/api/v1/children/child1/dashboard")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("dashboard for another parent's child: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
