package api_test

import (
	"net/http"
	"testing"

	"github.com/oioio-space/encre/engine"
)

func TestChildLoginSetsSessionCookieAndAllowsMe(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")

	loginChild(t, ts, "Mia", "1379")

	resp := ts.get(t, "/api/v1/child/me")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("child/me status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type meResponse struct {
		ChildID string
		Pseudo  string
	}
	got := decodeBody[meResponse](t, resp)
	if got.ChildID != "child1" || got.Pseudo != "Mia" {
		t.Errorf("child/me = %+v, want ChildID=child1 Pseudo=Mia", got)
	}
}

// TestChildLoginRejectsWrongPattern is the regression test for a handler
// that would trust auth.LoginChild's error text or forget to return early:
// a wrong pattern must answer 401 with no session cookie set, not a session
// for the wrong child.
func TestChildLoginRejectsWrongPattern(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")

	resp := ts.post(t, "/api/v1/child/login", map[string]string{"pseudo": "Mia", "pattern": "0000"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("child/login with wrong pattern: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "encre_child" && c.Value != "" {
			t.Errorf("child/login with wrong pattern set a session cookie: %+v", c)
		}
	}
}

func TestChildMeRequiresSession(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.get(t, "/api/v1/child/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("child/me with no session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestChildMeSortsGardeCandidatesTarnishedFirst checks the ordering
// contract the handler promises: a client that stopped sorting client-side
// would only notice this test, since the JSON shape looks identical either
// way.
func TestChildMeSortsGardeCandidatesTarnishedFirst(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginChild(t, ts, "Mia", "1379")

	for _, id := range []string{"plain-gold", "tarnished", "cursed-gold", "not-gold"} {
		seedItem(t, ts.DB, "child1", id)
	}
	states := map[string]*engine.WordState{
		"plain-gold":  {Gold: true},
		"tarnished":   {Gold: true, Tarnished: true},
		"cursed-gold": {Gold: true, Cursed: true}, // must be excluded
		"not-gold":    {Seen: true},               // must be excluded
	}
	if err := ts.DB.SaveWordStates(t.Context(), "child1", states); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}

	resp := ts.get(t, "/api/v1/child/me")
	type meResponse struct {
		Garde []struct {
			ItemID    string
			Tarnished bool
		}
	}
	got := decodeBody[meResponse](t, resp)
	if len(got.Garde) != 2 {
		t.Fatalf("len(Garde) = %d, want 2 (gold, non-cursed only): %+v", len(got.Garde), got.Garde)
	}
	if !got.Garde[0].Tarnished || got.Garde[0].ItemID != "tarnished" {
		t.Errorf("Garde[0] = %+v, want the tarnished candidate first", got.Garde[0])
	}
	if got.Garde[1].ItemID != "plain-gold" {
		t.Errorf("Garde[1] = %+v, want plain-gold", got.Garde[1])
	}
}

func TestChildMeReportsRemainingTime(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	setDailyLimitMinutes(t, ts.DB, "child1", 20)
	loginChild(t, ts, "Mia", "1379")

	resp := ts.get(t, "/api/v1/child/me")
	type meResponse struct{ RemainingSeconds int }
	got := decodeBody[meResponse](t, resp)
	if got.RemainingSeconds != 20*60 {
		t.Errorf("RemainingSeconds = %d, want %d before any play today", got.RemainingSeconds, 20*60)
	}
}
