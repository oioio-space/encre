package api_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/api"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// testPepper returns a [auth.Pepper] with a fixed, test-only key: good
// enough to exercise every peppering and TOTP-encryption path this package's
// tests go through, never anything a production deployment would load.
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
// everything a test needs to drive the actual handler chain end to end
// rather than calling engine or store functions directly.
type testServer struct {
	*httptest.Server
	Client *http.Client
	DB     *store.Store
	// APIServer is the [api.Server] this testServer wraps, exposed for the
	// handful of tests that configure it beyond [api.New]'s own
	// parameters — [api.Server.SetGenClient], namely.
	APIServer *api.Server
	now       *time.Time
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
	ts := &testServer{Server: hs, Client: &http.Client{Jar: jar}, DB: db, APIServer: srv, now: &now}
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

// seedParent inserts a parent with the given ID, a derived email, and a
// derived family code (encre-qpx.5) so tests can drive the family-scoped
// child login endpoints without a separate signup step.
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
// It looks pseudo's child ID and family code up directly (a test-only
// shortcut standing in for the family-scoped login page picking an avatar
// from [Server.handleFamilyChildren]'s list) so every existing call site
// keeps addressing a child by the pseudo it seeded, rather than needing to
// thread a child ID and family code through as well.
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

// loginParent opens a TOTP-fresh parent session for parentID directly
// through the store, bypassing the password and TOTP flow server/auth and
// server/parent already test on their own: this package's tests exercise
// what a handler does with an already-fresh [auth.RequireParent] session,
// not how one is reached. The cookie is left on ts's client for subsequent
// calls.
func loginParent(t *testing.T, ts *testServer, parentID string) {
	t.Helper()
	token, err := auth.CreateSession(t.Context(), ts.DB, store.SessionParent, parentID, ts.Now())
	if err != nil {
		t.Fatalf("auth.CreateSession() error = %v", err)
	}
	_, err = ts.DB.DB().ExecContext(t.Context(),
		`UPDATE sessions SET totp_ok_until = ? WHERE token = ?`,
		ts.Now().Add(time.Hour).Unix(), hashForTest(token))
	if err != nil {
		t.Fatalf("setting totp_ok_until: %v", err)
	}
	ts.Client.Jar.SetCookies(mustParseURL(t, ts.URL), []*http.Cookie{{
		Name: auth.CookieParent, Value: token,
	}})
}

// hashForTest mirrors server/auth's unexported hashSessionToken so
// [loginParent] can update the session row [auth.CreateSession] just wrote,
// keyed the same way server/store stores it (by the token's hash, never the
// raw token itself).
func hashForTest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// mustParseURL parses rawURL, failing the test on error.
func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawURL, err)
	}
	return u
}

// seedDicteeList creates a word list with two items for childID and returns
// (listID, item1ID, item2ID), ready for a POST .../dictee-result test.
func seedDicteeList(t *testing.T, ts *testServer, childID string) (listID, item1ID, item2ID string) {
	t.Helper()
	listID = childID + "-dictee-list"
	l := &store.WordList{ID: listID, ChildID: childID, Label: "Semaine", CreatedAt: time.Now()}
	if err := ts.DB.CreateList(t.Context(), l); err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}
	item1ID, item2ID = childID+"-item1", childID+"-item2"
	for _, id := range []string{item1ID, item2ID} {
		item := &store.Item{ID: id, ListID: listID, Kind: engine.KindWord, Text: id, Enabled: true}
		if err := ts.DB.SaveItem(t.Context(), item); err != nil {
			t.Fatalf("SaveItem() error = %v", err)
		}
	}
	return listID, item1ID, item2ID
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
