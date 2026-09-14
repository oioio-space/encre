package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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
// everything a test needs to drive the actual handler chain end to end
// rather than calling engine or store functions directly.
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
	srv := api.New(db, engine.DefaultConfig(), func() time.Time { return now })
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	ts := &testServer{Server: hs, Client: &http.Client{Jar: jar}, DB: db, now: &now}
	// api.Server closes over `now` by reference to the local variable inside
	// this function's own scope, which would freeze it at whatever value it
	// held when New was called; storing that variable's address on ts and
	// having Advance write through it keeps them the same variable.
	return ts
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

// postRaw issues a POST with an arbitrary raw body, for tests that need to
// send something [json.Marshal] would never produce (a duplicate key,
// invalid UTF-8).
func (ts *testServer) postRaw(t *testing.T, path, body string) *http.Response {
	t.Helper()
	resp, err := ts.Client.Post(ts.URL+path, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// get issues a GET using ts's cookie-aware client.
func (ts *testServer) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := ts.Client.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
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

// seedParent inserts a parent with the given ID and a derived email.
func seedParent(t *testing.T, db *store.Store, id string) {
	t.Helper()
	p := &store.Parent{ID: id, Email: id + "@example.com", PassHash: []byte("h"), CreatedAt: time.Now()}
	if err := db.CreateParent(t.Context(), p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}
}

// seedChild inserts a parent and a child under it, with pattern as the
// child's login pattern, so tests can log in as the child they just seeded.
func seedChild(t *testing.T, db *store.Store, id, pseudo, pattern string) {
	t.Helper()
	seedParent(t, db, id+"-parent")
	hash, err := auth.HashPattern(pattern)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	c := &store.Child{ID: id, ParentID: id + "-parent", Pseudo: pseudo, PatternHash: []byte(hash), CreatedAt: time.Now()}
	if err := db.CreateChild(t.Context(), c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}
}

// seedItem adds one bare item under a fresh, unvalidated list for childID —
// enough to satisfy word_states' foreign key on item_id, for tests that
// exercise word state without needing the item to show up in a deck.
func seedItem(t *testing.T, db *store.Store, childID, itemID string) {
	t.Helper()
	l := &store.WordList{ID: itemID + "-list", ChildID: childID, Label: "l", CreatedAt: time.Now()}
	if err := db.CreateList(t.Context(), l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	item := &store.Item{ID: itemID, ListID: l.ID, Kind: engine.KindWord, Text: itemID}
	if err := db.SaveItem(t.Context(), item); err != nil {
		t.Fatalf("SaveItem() error = %v", err)
	}
}

// seedValidatedWord adds one enabled item to a freshly validated list for
// childID, so [Server.wordCatalog] (via /run/start) picks it up.
func seedValidatedWord(t *testing.T, db *store.Store, childID, itemID, text string) {
	t.Helper()
	l := &store.WordList{ID: itemID + "-list", ChildID: childID, Label: "l", CreatedAt: time.Now()}
	if err := db.CreateList(t.Context(), l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	item := &store.Item{ID: itemID, ListID: l.ID, Kind: engine.KindWord, Text: text, Enabled: true, AudioPath: itemID + ".ogg"}
	if err := db.SaveItem(t.Context(), item); err != nil {
		t.Fatalf("SaveItem() error = %v", err)
	}
	if err := db.ValidateList(t.Context(), l.ID); err != nil {
		t.Fatalf("ValidateList() error = %v", err)
	}
}

// loginChild logs in as pseudo/pattern through the real /child/login
// endpoint, leaving the session cookie on ts's client for subsequent calls.
func loginChild(t *testing.T, ts *testServer, pseudo, pattern string) {
	t.Helper()
	resp := ts.post(t, "/api/v1/child/login", map[string]string{"pseudo": pseudo, "pattern": pattern})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("child/login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// setDailyLimitMinutes overwrites childID's daily_limit_json directly: the
// column is opaque JSON server/store keeps no engine type for, so there is
// no Store setter to call through instead.
func setDailyLimitMinutes(t *testing.T, db *store.Store, childID string, minutes int) {
	t.Helper()
	_, err := db.DB().ExecContext(t.Context(),
		`UPDATE children SET daily_limit_json = ? WHERE id = ?`,
		fmt.Sprintf(`{"minutes":%d}`, minutes), childID)
	if err != nil {
		t.Fatalf("setting daily limit: %v", err)
	}
}
