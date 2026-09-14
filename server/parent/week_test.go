package parent_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/parent"
	"github.com/oioio-space/encre/server/store"
)

// weekTestServer is a logged-in [parent.Server] with a temp media root, for
// this file's week-flow tests: paste → validation → voix → date.
type weekTestServer struct {
	url      string
	client   *http.Client
	db       *store.Store
	parentID string
	// requests counts every request made through client, for the
	// interaction-budget test — see [TestSemaineTenWordsUnderInteractionBudget].
	requests int
}

func newWeekTestServer(t *testing.T) *weekTestServer {
	t.Helper()

	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	srv, err := parent.NewServer(db, testPepper(t))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.SetMediaRoot(t.TempDir())

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	wts := &weekTestServer{url: ts.URL, db: db}
	wts.client = &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     countingTransport{t: wts, base: http.DefaultTransport},
	}

	tp := seedParent(t, db)
	wts.parentID = tp.id
	if resp := login(t, ts, wts.client, tp); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: got status %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	// Logging in itself is not one of the parent's ten-word-list
	// interactions the budget test counts; only requests made after this
	// point are.
	wts.requests = 0
	return wts
}

// countingTransport increments srv.requests on every round trip, so the
// budget test can count exactly how many requests the flow it drives makes
// — the same number of taps a real parent's browser would send.
type countingTransport struct {
	t    *weekTestServer
	base http.RoundTripper
}

func (c countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.t.requests++
	return c.base.RoundTrip(r)
}

func (ts *weekTestServer) get(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	return get(t, ts.client, ts.url+path)
}

func (ts *weekTestServer) post(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	return postForm(t, ts.client, ts.url+path, form, true)
}

// postFollow posts form to path and, if the response is a redirect (every
// test client in this package disables automatic redirect-following so
// status codes stay inspectable — see [newWeekTestServer]), follows it with
// a GET, returning the final response and body.
func (ts *weekTestServer) postFollow(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	resp, body := ts.post(t, path, form)
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return resp, body
	}
	return ts.get(t, resp.Header.Get("Location"))
}

func (ts *weekTestServer) patch(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, ts.url+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest PATCH: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := ts.client.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	_ = resp.Body.Close()
	return resp, string(body)
}

func (ts *weekTestServer) postAudio(t *testing.T, path, csrfToken string) (*http.Response, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("csrf_token", csrfToken); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	fw, err := mw.CreateFormFile("audio", "voix.webm")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte("not really audio, bytes are enough for this test")); err != nil {
		t.Fatalf("writing form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, ts.url+path, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := ts.client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	_ = resp.Body.Close()
	return resp, string(body)
}

// createChild drives the "add a child" form to completion and returns the
// new child's ID.
func (ts *weekTestServer) createChild(t *testing.T, pseudo, pattern string) string {
	t.Helper()
	_, body := ts.get(t, "/parent/children")
	csrfToken := extractCSRF(t, body)
	form := url.Values{"pseudo": {pseudo}, "pattern": {pattern}, "csrf_token": {csrfToken}}
	resp, respBody := ts.post(t, "/parent/children", form)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}
	children, err := ts.db.ChildrenOfParent(t.Context(), ts.parentID)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	for _, c := range children {
		if c.Pseudo == pseudo {
			return c.ID
		}
	}
	t.Fatalf("created child %q not found among %+v", pseudo, children)
	return ""
}

// extractListID reads the list ID [handleListsPost] sent the browser to,
// whether by redirect (the happy path) or by re-rendering the validation
// page directly at its own URL (an error path that never left
// /parent/children/.../lists).
func extractListID(t *testing.T, resp *http.Response) string {
	t.Helper()
	loc := resp.Header.Get("Location")
	if loc == "" {
		loc = resp.Request.URL.Path
	}
	parts := strings.Split(strings.Trim(loc, "/"), "/")
	if len(parts) < 2 {
		t.Fatalf("cannot extract list ID from %q", loc)
	}
	return parts[len(parts)-1]
}

func TestListsPostAnalysesWordsAndFillsColors(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{
		"label": {"Semaine 1"}, "kind": {"mot"}, "raw_text": {"chat, chien"},
		"csrf_token": {extractCSRF(t, newBody)},
	}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("posting list: got status %d; body: %s", resp.StatusCode, listBody)
	}
	if !strings.Contains(listBody, "chat") || !strings.Contains(listBody, "chien") {
		t.Errorf("validation page does not list both words: %s", listBody)
	}
	if !strings.Contains(listBody, "Muettes") {
		t.Errorf("validation page does not show chat's Muettes trap: %s", listBody)
	}
}

func TestValidateRefusesUnconfirmedUnsureItem(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"zorglubaxatron"}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("posting list: got status %d; body: %s", resp.StatusCode, listBody)
	}
	listID := extractListID(t, resp)

	validateForm := url.Values{"csrf_token": {extractCSRF(t, listBody)}}
	resp2, body2 := ts.post(t, "/parent/lists/"+listID+"/validate", validateForm)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("validate with unsure item: got status %d, want 200 (re-rendered with an error); body: %s", resp2.StatusCode, body2)
	}
	if !strings.Contains(body2, "à vérifier") {
		t.Errorf("validate response does not explain the block: %s", body2)
	}

	got, err := ts.db.ListByID(t.Context(), listID)
	if err != nil {
		t.Fatalf("ListByID: %v", err)
	}
	if got.Validated {
		t.Error("ValidateList: list got validated despite an unconfirmed unsure item")
	}
}

func TestConfirmThenValidateSucceeds(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"zorglubaxatron"}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	listID := extractListID(t, resp)

	items, err := ts.db.ItemsOfList(t.Context(), listID)
	if err != nil || len(items) != 1 {
		t.Fatalf("ItemsOfList: %v, %d items", err, len(items))
	}
	itemID := items[0].ID

	confirmForm := url.Values{"csrf_token": {extractCSRF(t, listBody)}}
	confirmResp, confirmBody := ts.post(t, "/parent/lists/"+listID+"/items/"+itemID+"/confirm", confirmForm)
	if confirmResp.StatusCode != http.StatusOK {
		t.Fatalf("confirm: got status %d; body: %s", confirmResp.StatusCode, confirmBody)
	}

	validateForm := url.Values{"due_date": {"2026-09-20"}, "csrf_token": {extractCSRF(t, confirmBody)}}
	validateResp, validateBody := ts.post(t, "/parent/lists/"+listID+"/validate", validateForm)
	if validateResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("validate after confirm: got status %d, want %d; body: %s", validateResp.StatusCode, http.StatusSeeOther, validateBody)
	}

	got, err := ts.db.ListByID(t.Context(), listID)
	if err != nil {
		t.Fatalf("ListByID: %v", err)
	}
	if !got.Validated {
		t.Error("ValidateList: list was not validated after the unsure item was confirmed")
	}
	if got.DueDate.Format("2006-01-02") != "2026-09-20" {
		t.Errorf("DueDate = %v, want 2026-09-20", got.DueDate)
	}
}

func TestItemPatchCorrectsTextAndResetsConfirmation(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"zorglubaxatron"}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	listID := extractListID(t, resp)
	items, err := ts.db.ItemsOfList(t.Context(), listID)
	if err != nil || len(items) != 1 {
		t.Fatalf("ItemsOfList: %v, %d items", err, len(items))
	}
	itemID := items[0].ID

	// Confirm it first, to prove the PATCH resets Confirmed even for a
	// word the lexicon ends up confident about.
	confirmForm := url.Values{"csrf_token": {extractCSRF(t, listBody)}}
	_, confirmBody := ts.post(t, "/parent/lists/"+listID+"/items/"+itemID+"/confirm", confirmForm)

	patchForm := url.Values{"text": {"chat"}, "csrf_token": {extractCSRF(t, confirmBody)}}
	patchResp, _ := ts.patch(t, "/parent/lists/"+listID+"/items/"+itemID, patchForm)
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH item: got status %d, want 200", patchResp.StatusCode)
	}

	got, err := ts.db.ItemByID(t.Context(), itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if got.Text != "chat" {
		t.Errorf("Text = %q, want chat", got.Text)
	}
	if got.Confirmed {
		t.Error("Confirmed = true after a text correction, want false (needs re-review)")
	}
	if got.Colors[engine.Muettes] != 1 {
		t.Errorf("Colors[Muettes] = %d, want 1 after re-analysis", got.Colors[engine.Muettes])
	}
}

func TestItemColorRemoveIsOneTapAndIdempotent(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"chat"}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	listID := extractListID(t, resp)
	items, _ := ts.db.ItemsOfList(t.Context(), listID)
	itemID := items[0].ID

	removePath := "/parent/lists/" + listID + "/items/" + itemID + "/colors/muettes/remove"
	resp1, body1 := ts.post(t, removePath, url.Values{"csrf_token": {extractCSRF(t, listBody)}})
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first remove: got status %d; body: %s", resp1.StatusCode, body1)
	}
	got, err := ts.db.ItemByID(t.Context(), itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if _, ok := got.Colors[engine.Muettes]; ok {
		t.Errorf("Colors still has Muettes after removing its only trap: %v", got.Colors)
	}

	// A second tap must not error even with nothing left to remove.
	resp2, body2 := ts.post(t, removePath, url.Values{"csrf_token": {extractCSRF(t, body1)}})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second (idempotent) remove: got status %d; body: %s", resp2.StatusCode, body2)
	}
}

func TestItemAudioUploadCountsTowardProgress(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"chat"}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	listID := extractListID(t, resp)
	items, _ := ts.db.ItemsOfList(t.Context(), listID)
	itemID := items[0].ID

	if !strings.Contains(listBody, "0 / 1") {
		t.Errorf("validation page does not start at 0 / 1: %s", listBody)
	}

	audioResp, audioBody := ts.postAudio(t, "/parent/lists/"+listID+"/items/"+itemID+"/audio", extractCSRF(t, listBody))
	if audioResp.StatusCode != http.StatusOK {
		t.Fatalf("audio upload: got status %d; body: %s", audioResp.StatusCode, audioBody)
	}
	if !strings.Contains(audioBody, "1 / 1") {
		t.Errorf("validation page did not advance the counter after upload: %s", audioBody)
	}

	got, err := ts.db.ItemByID(t.Context(), itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if got.AudioPath == "" {
		t.Error("AudioPath is empty after uploading audio")
	}
}

// TestSemaineTenWordsUnderInteractionBudget is encre-fov's acceptance
// criterion made concrete: "moins de 2 minutes chronométrées sur une liste
// de 10 mots avec la voix." A test cannot time a human, but it can count
// every request their browser would send driving the flow to completion —
// paste, confirm the words the lexicon does not know outright, record one
// clip per word, and validate with a date — and that count is a direct,
// reproducible proxy for how many taps stand between a parent and a
// playable week.
//
// Budget: 4 fixed requests (open the paste form, submit it, open the
// validation page — already included in the paste response so not
// counted twice — and the final validate) plus at most 2 requests per
// word (one possible "confirmer" tap for a lexicon miss, one audio
// upload) — 4 + 2*10 = 24. This scenario's words are all known to the
// embedded lexicon (no "confirmer" tap needed), so it should land well
// under that ceiling; the ceiling itself is what must never regress.
func TestSemaineTenWordsUnderInteractionBudget(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	const budget = 4 + 2*10

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new") // 1
	words := "chat, chien, souris, maison, jardin, poisson, bateau, gomme, cheval, tableau"
	form := url.Values{"kind": {"mot"}, "raw_text": {words}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form) // 2
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("posting list: got status %d; body: %s", resp.StatusCode, listBody)
	}
	listID := extractListID(t, resp)

	items, err := ts.db.ItemsOfList(t.Context(), listID)
	if err != nil {
		t.Fatalf("ItemsOfList: %v", err)
	}
	if len(items) != 10 {
		t.Fatalf("len(items) = %d, want 10", len(items))
	}

	csrfToken := extractCSRF(t, listBody)
	for _, item := range items {
		if item.Confidence < 0.6 {
			resp, body := ts.post(t, "/parent/lists/"+listID+"/items/"+item.ID+"/confirm", url.Values{"csrf_token": {csrfToken}})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("confirming %s: got status %d", item.Text, resp.StatusCode)
			}
			csrfToken = extractCSRF(t, body)
		}
		resp, body := ts.postAudio(t, "/parent/lists/"+listID+"/items/"+item.ID+"/audio", csrfToken)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("recording %s: got status %d; body: %s", item.Text, resp.StatusCode, body)
		}
		csrfToken = extractCSRF(t, body)
	}

	validateResp, validateBody := ts.post(t, "/parent/lists/"+listID+"/validate",
		url.Values{"due_date": {"2026-09-20"}, "csrf_token": {csrfToken}}) // last
	if validateResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("validate: got status %d, want %d; body: %s", validateResp.StatusCode, http.StatusSeeOther, validateBody)
	}

	got, err := ts.db.ListByID(t.Context(), listID)
	if err != nil {
		t.Fatalf("ListByID: %v", err)
	}
	if !got.Validated {
		t.Fatal("list is not validated at the end of the flow")
	}

	if ts.requests > budget {
		t.Errorf("interaction count = %d, want <= %d (encre-fov's < 2 min budget for a 10-word list)",
			ts.requests, budget)
	}
	t.Logf("semaine flow for 10 words took %d requests (budget %d)", ts.requests, budget)
}

func TestQuickWordAddsToStandingList(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, body := ts.get(t, "/parent/children")
	form := url.Values{"text": {"chaton"}, "csrf_token": {extractCSRF(t, body)}}
	resp, respBody := ts.post(t, "/parent/children/"+childID+"/quick-word", form)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("quick-word: got status %d, want %d; body: %s", resp.StatusCode, http.StatusSeeOther, respBody)
	}

	lists, err := ts.db.ListsOfChild(t.Context(), childID)
	if err != nil || len(lists) != 1 {
		t.Fatalf("ListsOfChild: %v, %d lists", err, len(lists))
	}
	if !lists[0].Validated {
		t.Error("quick word list is not validated: the word would not be playable")
	}
	items, err := ts.db.ItemsOfList(t.Context(), lists[0].ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("ItemsOfList: %v, %d items", err, len(items))
	}
	if items[0].Text != "chaton" || !items[0].Confirmed {
		t.Errorf("quick word item = %+v, want Text=chaton, Confirmed=true", items[0])
	}
}

func TestDicteeResultOnlyRecordsCorrectAnswers(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"chat, chien"}, "csrf_token": {extractCSRF(t, newBody)}}
	resp, listBody := ts.postFollow(t, "/parent/children/"+childID+"/lists", form)
	listID := extractListID(t, resp)
	items, err := ts.db.ItemsOfList(t.Context(), listID)
	if err != nil || len(items) != 2 {
		t.Fatalf("ItemsOfList: %v, %d items", err, len(items))
	}

	resultForm := url.Values{"csrf_token": {extractCSRF(t, listBody)}}
	resultForm.Set("correct."+items[0].ID, "on") // items[1] left unchecked: wrong on paper
	resp2, body2 := ts.post(t, "/parent/lists/"+listID+"/dictee-result", resultForm)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("dictee-result: got status %d; body: %s", resp2.StatusCode, body2)
	}

	results, err := ts.db.DicteeResultsOfList(t.Context(), listID)
	if err != nil {
		t.Fatalf("DicteeResultsOfList: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1 (bonus positif uniquement, jamais de malus)", len(results))
	}
	if results[0].ItemID != items[0].ID || !results[0].Correct {
		t.Errorf("recorded result = %+v, want a Correct=true result for %s", results[0], items[0].ID)
	}
}

func TestExportContainsEverything(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, newBody := ts.get(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{"kind": {"mot"}, "raw_text": {"chat"}, "csrf_token": {extractCSRF(t, newBody)}}
	ts.postFollow(t, "/parent/children/"+childID+"/lists", form)

	resp, body := ts.get(t, "/parent/export")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export: got status %d; body: %s", resp.StatusCode, body)
	}
	for _, want := range []string{"Timéo", "chat", ts.parentID} {
		if !strings.Contains(body, want) {
			t.Errorf("export body does not contain %q: %s", want, body)
		}
	}
}

func TestAccountDeleteRemovesEverythingAndRevokesSessions(t *testing.T) {
	ts := newWeekTestServer(t)
	childID := ts.createChild(t, "Timéo", "13579")

	_, body := ts.get(t, "/parent/children")
	form := url.Values{"confirm": {"SUPPRIMER"}, "csrf_token": {extractCSRF(t, body)}}
	resp, respBody := ts.post(t, "/parent/account/delete", form)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("account delete: got status %d, want %d; body: %s", resp.StatusCode, http.StatusSeeOther, respBody)
	}

	if _, err := ts.db.ParentByID(t.Context(), ts.parentID); err == nil {
		t.Error("parent row still exists after account deletion")
	}
	if _, err := ts.db.ChildByID(t.Context(), childID); err == nil {
		t.Error("child row still exists after account deletion")
	}

	// The session cookie the deletion request itself carried must now be
	// dead: a further request redirects to login rather than succeeding.
	afterResp, _ := ts.get(t, "/parent/children")
	if afterResp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /parent/children after account deletion: got status %d, want %d (redirect to login)",
			afterResp.StatusCode, http.StatusSeeOther)
	}
}

func TestAccountDeleteRequiresConfirmationText(t *testing.T) {
	ts := newWeekTestServer(t)
	_, body := ts.get(t, "/parent/children")
	form := url.Values{"confirm": {"oui"}, "csrf_token": {extractCSRF(t, body)}}
	resp, _ := ts.post(t, "/parent/account/delete", form)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("account delete with wrong confirmation text: got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if _, err := ts.db.ParentByID(t.Context(), ts.parentID); err != nil {
		t.Errorf("ParentByID: %v, want the parent to still exist", err)
	}
}
