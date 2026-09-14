// Package e2e holds ENCRE's slow, whole-system tests: real HTTP handlers, a
// real (in-memory) SQLite store, and — where the ticket calls for it — a real
// browser. Each test in here is the only place a particular seam between
// packages is checked; see the package-level doc comment in doc_test.go for
// which test guards which seam and why it does not belong next to the code
// it exercises instead.
package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/api"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// testPepper returns a [auth.Pepper] with a fixed, test-only key — good
// enough to exercise peppering and TOTP-encryption paths, never anything a
// production deployment would load.
func testPepper(t *testing.T) *auth.Pepper {
	t.Helper()
	key := bytes.Repeat([]byte("k"), 32)
	pep, err := auth.NewPepper("test", key)
	if err != nil {
		t.Fatalf("auth.NewPepper() error = %v", err)
	}
	return pep
}

// openTestDB opens a fresh in-memory store, closed automatically when t ends.
func openTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// testServer bundles an httptest.Server built from a real api.Server, a
// cookie-aware client, the store underneath, and a clock the test controls —
// the same shape server/api's own tests use, duplicated here rather than
// imported because it lives in an unexported _test.go file in that package
// and this package deliberately never imports server/api's test helpers, to
// avoid coupling this slow suite's compile to that package's internal test
// changes.
type testServer struct {
	*httptest.Server
	Client *http.Client
	DB     *store.Store
	now    *time.Time
}

// newTestServer builds a testServer with its own in-memory store, starting
// the clock at a fixed instant so every test controls it explicitly rather
// than racing time.Now.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	srv := api.New(db, engine.DefaultConfig(), func() time.Time { return now }, testPepper(t))
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	return &testServer{Server: hs, Client: &http.Client{Jar: jar}, DB: db, now: &now}
}

// Advance moves the test server's clock forward by d.
func (ts *testServer) Advance(d time.Duration) { *ts.now = ts.now.Add(d) }

// Now returns the test server's current clock reading.
func (ts *testServer) Now() time.Time { return *ts.now }

// post issues a POST to path with a JSON-encoded body, using ts's
// cookie-aware client, and returns the raw response for the caller to
// inspect.
func (ts *testServer) post(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling request body: %v", err)
	}
	resp, err := ts.Client.Post(ts.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// decodeBody decodes resp's JSON body into a fresh T, failing the test on
// any error.
func decodeBody[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decoding response body %q: %v", raw, err)
	}
	return v
}

// seedParent inserts a parent with the given ID, a derived email, and a
// derived family code so tests can drive the family-scoped child login
// endpoints without a separate signup step.
func seedParent(t *testing.T, db *store.Store, id string) {
	t.Helper()
	p := &store.Parent{
		ID: id, Email: id + "@example.com", PassHash: []byte("h"),
		FamilyCode: id + "-family", CreatedAt: time.Now(),
	}
	if err := db.CreateParent(t.Context(), p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}
}

// seedChild inserts a parent and a child under it, with pattern as the
// child's login pattern, so tests can log in as the child they just seeded.
func seedChild(t *testing.T, db *store.Store, id, pseudo, pattern string) {
	t.Helper()
	seedParent(t, db, id+"-parent")
	hash, err := auth.HashPattern(pattern, testPepper(t))
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	c := &store.Child{ID: id, ParentID: id + "-parent", Pseudo: pseudo, PatternHash: []byte(hash), CreatedAt: time.Now()}
	if err := db.CreateChild(t.Context(), c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}
}

// seedValidatedList creates one list per (id, text) pair under childID, each
// holding a single enabled, validated item — the shape [Server.wordCatalog]
// (via POST /run/start) reads, and the closest server/store gets today to
// "a parent pastes a list and validates it": server/parent has no HTTP
// endpoint for that flow yet (encre-018 is still in progress), so this goes
// through the store directly rather than a parent handler that does not
// exist. See doc_test.go for the gap this leaves in TestFullWeekJourney.
func seedValidatedList(t *testing.T, db *store.Store, childID string, words ...string) {
	t.Helper()
	for _, w := range words {
		l := &store.WordList{ID: childID + "-" + w + "-list", ChildID: childID, Label: w, CreatedAt: time.Now()}
		if err := db.CreateList(t.Context(), l); err != nil {
			t.Fatalf("CreateList(%q) error = %v", w, err)
		}
		item := &store.Item{
			ID: childID + "-" + w, ListID: l.ID, Kind: engine.KindWord, Text: w,
			Enabled: true, AudioPath: w + ".ogg",
		}
		if err := db.SaveItem(t.Context(), item); err != nil {
			t.Fatalf("SaveItem(%q) error = %v", w, err)
		}
		if err := db.ValidateList(t.Context(), l.ID); err != nil {
			t.Fatalf("ValidateList(%q) error = %v", w, err)
		}
	}
}

// loginChild logs in as pseudo/pattern through the real /child/login
// endpoint, leaving the session cookie on ts's client for subsequent calls.
func loginChild(t *testing.T, ts *testServer, pseudo, pattern string) {
	t.Helper()
	var childID, familyCode string
	err := ts.DB.DB().QueryRowContext(t.Context(), `
		SELECT children.id, parents.family_code
		FROM children JOIN parents ON parents.id = children.parent_id
		WHERE children.pseudo = ?`, pseudo).Scan(&childID, &familyCode)
	if err != nil {
		t.Fatalf("looking up seeded child %q: %v", pseudo, err)
	}

	resp := ts.post(t, "/api/v1/child/login", map[string]string{
		"familyCode": familyCode, "childID": childID, "pattern": pattern,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("child/login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// runStartResponse mirrors enough of the real POST /run/start response for
// tests to read out of it without importing server/api's unexported type.
type runStartResponse struct {
	RunID   string
	Deck    engine.Deck
	Rank    int
	Targets [3]float64
	Levels  map[engine.Color]int
	Audio   map[string]string
}

// startRun drives the real POST /run/start endpoint on ts's logged-in
// client.
func startRun(t *testing.T, ts *testServer, gardeIDs []string) (*http.Response, runStartResponse) {
	t.Helper()
	resp := ts.post(t, "/api/v1/run/start", map[string]any{"gardeIDs": gardeIDs})
	if resp.StatusCode != http.StatusOK {
		return resp, runStartResponse{}
	}
	return resp, decodeBody[runStartResponse](t, resp)
}

// finishBody is the JSON body of a POST /run/{id}/finish request.
func finishBody(attempts []engine.Attempt, revanche [3]bool) map[string]any {
	return map[string]any{
		"attempts":  attempts,
		"talismans": []engine.TalismanID{},
		"revanche":  revanche,
		"cahier":    false,
	}
}

// finishRunResponse mirrors enough of the real POST /run/{id}/finish
// response for tests to read out of it.
type finishRunResponse struct {
	Outcome engine.Outcome
	Events  []engine.Event
}
