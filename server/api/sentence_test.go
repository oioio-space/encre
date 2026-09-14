package api_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/oioio-space/encre/server/store"
)

// fakeGenClient is a [gen.Client] under this package's own control, so
// these tests never call the real Anthropic API.
type fakeGenClient struct {
	reply string
	err   error
}

func (f fakeGenClient) Complete(context.Context, string) (string, error) {
	return f.reply, f.err
}

// seedSentenceItem creates a validated list with one item for childID and
// returns (listID, itemID).
func seedSentenceItem(t *testing.T, ts *testServer, childID string) (listID, itemID string) {
	t.Helper()
	listID, itemID = childID+"-slist", childID+"-sitem"
	l := &store.WordList{ID: listID, ChildID: childID, Label: "l"}
	if err := ts.DB.CreateList(t.Context(), l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	item := &store.Item{ID: itemID, ListID: listID, Text: "chat", Enabled: true}
	if err := ts.DB.SaveItem(t.Context(), item); err != nil {
		t.Fatalf("SaveItem() error = %v", err)
	}
	return listID, itemID
}

const validGenReply = `{"mot": "chat", "phrases": [
	{"texte": "Le chat dort sur le lit.", "cible": "chat", "forme": "chat"},
	{"texte": "Le chat court dans le jardin.", "cible": "chat", "forme": "chat"},
	{"texte": "Le chat mange sa soupe.", "cible": "chat", "forme": "chat"}
]}`

func TestGenerateSentencesStoresAcceptedOnes(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	ts.APIServer.SetGenClient(fakeGenClient{reply: validGenReply})
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/items/"+itemID+"/sentences/generate", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("generate status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type sentenceView struct{ ID, Text, TargetForm string }
	type generateResponse struct {
		Sentences []sentenceView
		Rejected  int
	}
	got := decodeBody[generateResponse](t, resp)
	if len(got.Sentences) != 3 {
		t.Fatalf("len(Sentences) = %d, want 3: %+v", len(got.Sentences), got.Sentences)
	}

	stored, err := ts.DB.SentencesOfItem(t.Context(), itemID)
	if err != nil {
		t.Fatalf("SentencesOfItem() error = %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("len(SentencesOfItem()) = %d, want 3", len(stored))
	}
	for _, s := range stored {
		if s.Approved {
			t.Errorf("sentence %s Approved = true, want false before the parent taps it", s.ID)
		}
	}
}

func TestGenerateSentencesWithNoClientAnswers503(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/items/"+itemID+"/sentences/generate", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("generate with no client configured: status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestGenerateSentencesOnAPIFailureAnswers503NotServerError(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	ts.APIServer.SetGenClient(fakeGenClient{err: errors.New("connection refused")})
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/items/"+itemID+"/sentences/generate", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("generate on api failure: status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestManualSentenceIsStoredApprovedImmediately(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/items/"+itemID+"/sentences/manual",
		map[string]string{"texte": "Le chat dort sur le lit.", "forme": "chat"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manual status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	stored, err := ts.DB.SentencesOfItem(t.Context(), itemID)
	if err != nil {
		t.Fatalf("SentencesOfItem() error = %v", err)
	}
	if len(stored) != 1 || !stored[0].Approved {
		t.Fatalf("SentencesOfItem() = %+v, want one Approved=true sentence", stored)
	}
}

func TestManualSentenceRejectsEmptyText(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/items/"+itemID+"/sentences/manual",
		map[string]string{"texte": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("manual with blank text: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestApproveSentence(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	sentence := &store.Sentence{ID: "s1", ItemID: itemID, Text: "Le chat dort.", TargetForm: "chat"}
	if err := ts.DB.SaveSentence(t.Context(), sentence); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}

	resp := ts.post(t, "/api/v1/lists/"+listID+"/sentences/s1/approve", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got, err := ts.DB.SentenceByID(t.Context(), "s1")
	if err != nil {
		t.Fatalf("SentenceByID() error = %v", err)
	}
	if !got.Approved {
		t.Error("Approved = false after approve")
	}
}

func TestRegenerateSentenceReplacesTextAndResetsApproval(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	ts.APIServer.SetGenClient(fakeGenClient{reply: validGenReply})
	loginParent(t, ts, "child1-parent")

	original := &store.Sentence{
		ID: "s1", ItemID: itemID, Text: "Une mauvaise phrase.", TargetForm: "chat",
		AudioPath: "old.ogg", Approved: true,
	}
	if err := ts.DB.SaveSentence(t.Context(), original); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}

	resp := ts.post(t, "/api/v1/lists/"+listID+"/sentences/s1/regenerate", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	got, err := ts.DB.SentenceByID(t.Context(), "s1")
	if err != nil {
		t.Fatalf("SentenceByID() error = %v", err)
	}
	if got.Text == original.Text {
		t.Error("Text unchanged after regenerate")
	}
	if got.Approved {
		t.Error("Approved = true after regenerate, want false (unreviewed new text)")
	}
	if got.AudioPath != "" {
		t.Errorf("AudioPath = %q after regenerate, want empty (audio recorded against the old text)", got.AudioPath)
	}
}

func TestSentenceEndpointsRequireParentSession(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginChild(t, ts, "Mia", "1379")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/items/"+itemID+"/sentences/manual",
		map[string]string{"texte": "Le chat dort."})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("manual with a child session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestApproveSentenceRejectsAnotherParentsList(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Théo", "2468")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	sentence := &store.Sentence{ID: "s1", ItemID: itemID, Text: "Le chat dort.", TargetForm: "chat"}
	if err := ts.DB.SaveSentence(t.Context(), sentence); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}
	loginParent(t, ts, "child2-parent")

	resp := ts.post(t, "/api/v1/lists/"+listID+"/sentences/s1/approve", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("approve against another parent's list: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
