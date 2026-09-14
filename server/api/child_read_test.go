package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/oioio-space/encre/content"
	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

// TestChildRulesReportsLevelsPerCouleur checks that GET /child/rules answers
// the fioles: the child's level in every Couleur, straight from the engine
// standing rather than some second copy of it.
func TestChildRulesReportsLevelsPerCouleur(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	setChildLevel(t, ts.DB, "child1", engine.Muettes, 4)
	loginChild(t, ts, "Mia", "1379")

	resp := ts.get(t, "/api/v1/child/rules")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("child/rules status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type rulesResponse struct {
		Levels map[string]int
	}
	got := decodeBody[rulesResponse](t, resp)
	if got.Levels[fmt.Sprint(int(engine.Muettes))] != 4 {
		t.Errorf("Levels[Muettes] = %d, want 4: %+v", got.Levels[fmt.Sprint(int(engine.Muettes))], got.Levels)
	}
}

func TestChildRulesRequiresSession(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.get(t, "/api/v1/child/rules")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("child/rules with no session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestChildExploitsHidesUnachievedHiddenOnes is the ticket's core rule for
// the gallery: a hidden Exploit the child has not done yet shows as "???",
// never its real name or reward, while an achieved one — hidden or not —
// shows in full.
func TestChildExploitsHidesUnachievedHiddenOnes(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginChild(t, ts, "Mia", "1379")

	hidden := firstHiddenExploitID(t)
	setChildExploits(t, ts.DB, "child1", []string{})

	resp := ts.get(t, "/api/v1/child/exploits")
	type exploitView struct {
		ID, Name, Reward string
		Hidden, Achieved bool
	}
	type exploitsResponse struct{ Exploits []exploitView }
	got := decodeBody[exploitsResponse](t, resp)

	var found bool
	for _, e := range got.Exploits {
		if e.ID != hidden {
			continue
		}
		found = true
		if e.Achieved {
			t.Errorf("exploit %q Achieved = true, want false", hidden)
		}
		if e.Name != "???" || e.Reward != "" {
			t.Errorf("unachieved hidden exploit %+v, want Name=??? Reward=empty", e)
		}
	}
	if !found {
		t.Fatalf("hidden exploit %q not present in response", hidden)
	}
}

// TestChildExploitsRevealsAchievedHiddenOnes is the flip side: once the
// child's record lists a hidden Exploit's ID, it shows in full.
func TestChildExploitsRevealsAchievedHiddenOnes(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginChild(t, ts, "Mia", "1379")

	hidden := firstHiddenExploitID(t)
	setChildExploits(t, ts.DB, "child1", []string{hidden})

	resp := ts.get(t, "/api/v1/child/exploits")
	type exploitView struct {
		ID, Name, Reward string
		Hidden, Achieved bool
	}
	type exploitsResponse struct{ Exploits []exploitView }
	got := decodeBody[exploitsResponse](t, resp)

	for _, e := range got.Exploits {
		if e.ID == hidden {
			if !e.Achieved || e.Name == "???" {
				t.Errorf("achieved hidden exploit %+v, want Achieved=true and its real name", e)
			}
			return
		}
	}
	t.Fatalf("hidden exploit %q not present in response", hidden)
}

// TestChildBestiaryReportsWordState checks that the bestiary answers every
// validated, enabled word the child owns with the state its WordState
// implies: missing (never seen), known (seen, not gold) or gold.
func TestChildBestiaryReportsWordState(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	seedValidatedWord(t, ts.DB, "child1", "gomme", "gomme")
	loginChild(t, ts, "Mia", "1379")

	if err := ts.DB.SaveWordStates(t.Context(), "child1", map[string]*engine.WordState{
		"chat": {Gold: true, Seen: true},
	}); err != nil {
		t.Fatalf("SaveWordStates() error = %v", err)
	}

	resp := ts.get(t, "/api/v1/child/bestiary")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("child/bestiary status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type bestiaryWord struct {
		ItemID, State string
	}
	type bestiaryResponse struct{ Words []bestiaryWord }
	got := decodeBody[bestiaryResponse](t, resp)

	states := map[string]string{}
	for _, w := range got.Words {
		states[w.ItemID] = w.State
	}
	if states["chat"] != "gold" {
		t.Errorf("chat state = %q, want gold", states["chat"])
	}
	if states["gomme"] != "missing" {
		t.Errorf("gomme state = %q, want missing", states["gomme"])
	}
}

// TestChildResultCardServesAValidPNG proves the acceptance criterion: the
// endpoint answers an actual PNG image ([image/png] can decode it back), not
// a placeholder or an error dressed as one.
func TestChildResultCardServesAValidPNG(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")

	_, run := startRun(t, ts, nil)

	resp := ts.get(t, "/api/v1/child/result-card/"+run.RunID+".png")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("result-card status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	body, err := readAll(resp)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		t.Errorf("decoded image has empty bounds: %v", img.Bounds())
	}
}

// TestChildResultCardRejectsAnotherChildsRun proves the run-ownership rule
// the whole package holds elsewhere: this endpoint must not become the one
// place a child can read another child's data.
func TestChildResultCardRejectsAnotherChildsRun(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Théo", "2468")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	ts.Client.Jar = nil // drop child1's cookies before logging in as child2
	jar := newCookieJar(t)
	ts.Client.Jar = jar
	loginChild(t, ts, "Théo", "2468")

	resp := ts.get(t, "/api/v1/child/result-card/"+run.RunID+".png")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("result-card for another child's run: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestChildResultCardRequiresSession(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.get(t, "/api/v1/child/result-card/whatever.png")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("result-card with no session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestChildResultCardNotFoundForUnknownRun(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	loginChild(t, ts, "Mia", "1379")

	resp := ts.get(t, "/api/v1/child/result-card/nope.png")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("result-card for unknown run: status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// firstHiddenExploitID returns the ID of the first hidden Exploit the
// embedded content pack carries, failing the test if it carries none — the
// content this ticket must never recopy.
func firstHiddenExploitID(t *testing.T) string {
	t.Helper()
	for _, e := range content.Embedded().Exploits {
		if e.Hidden {
			return e.ID
		}
	}
	t.Fatal("content.Embedded().Exploits carries no hidden exploit to test with")
	return ""
}

// setChildLevel writes childID's level in one Couleur through the same
// [store.Child.SetEngine] / [store.Store.SaveChild] path a real handler
// would use, rather than reaching for the opaque JSON column directly.
func setChildLevel(t *testing.T, db *store.Store, childID string, c engine.Color, level int) {
	t.Helper()
	child, err := db.ChildByID(t.Context(), childID)
	if err != nil {
		t.Fatalf("ChildByID(%q) error = %v", childID, err)
	}
	ec := child.Engine()
	if ec.Level == nil {
		ec.Level = map[engine.Color]int{}
	}
	ec.Level[c] = level
	child.SetEngine(ec)
	if err := db.SaveChild(t.Context(), child); err != nil {
		t.Fatalf("SaveChild() error = %v", err)
	}
}

// setChildExploits writes childID's achieved-exploit IDs directly: the
// column is opaque JSON server/store keeps no engine type for, the same gap
// [dailyLimitMinutes] fills for the daily-limit column.
func setChildExploits(t *testing.T, db *store.Store, childID string, achieved []string) {
	t.Helper()
	raw, err := json.Marshal(map[string][]string{"achieved": achieved})
	if err != nil {
		t.Fatalf("marshaling achieved exploits: %v", err)
	}
	_, err = db.DB().ExecContext(t.Context(),
		`UPDATE children SET exploits_json = ? WHERE id = ?`, raw, childID)
	if err != nil {
		t.Fatalf("setting exploits_json: %v", err)
	}
}

// newCookieJar builds a fresh, empty cookie jar.
func newCookieJar(t *testing.T) http.CookieJar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	return jar
}

// readAll drains resp's body.
func readAll(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
