package parent_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/parent"
	"github.com/oioio-space/encre/server/store"
)

// testPepper builds a fixed-key [auth.Pepper], shared by every helper in
// this file so a parent seeded through [seedParentWithID] and the
// [parent.Server] built by [newTestServer] always agree on it.
func testPepper(t *testing.T) *auth.Pepper {
	t.Helper()
	pep, err := auth.NewPepper("test", bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatalf("auth.NewPepper() error = %v", err)
	}
	return pep
}

// testParent is a parent account seeded directly through the store, with
// TOTP already enrolled, so tests can log in without going through a signup
// flow this bead does not own.
type testParent struct {
	id, email, password, totpSecret string
}

func seedParent(t *testing.T, db *store.Store) testParent {
	t.Helper()
	return seedParentWithID(t, db, "parent-1", "parent@example.test")
}

func seedParentWithID(t *testing.T, db *store.Store, id, email string) testParent {
	t.Helper()

	pep := testPepper(t)
	hash, err := auth.HashPassword("correct horse battery staple", pep)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	enrollment, err := auth.EnrollTOTP(email)
	if err != nil {
		t.Fatalf("EnrollTOTP: %v", err)
	}
	sealed, err := pep.Encrypt([]byte(enrollment.Secret))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	p := &store.Parent{
		ID:         id,
		Email:      email,
		PassHash:   []byte(hash),
		TOTPSecret: sealed,
		CreatedAt:  time.Now(),
	}
	if err := db.CreateParent(t.Context(), p); err != nil {
		t.Fatalf("CreateParent: %v", err)
	}
	return testParent{id: p.ID, email: p.Email, password: "correct horse battery staple", totpSecret: enrollment.Secret}
}

func (tp testParent) code(t *testing.T, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(tp.totpSecret, at)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	return code
}

// newTestServer returns a ready [parent.Server], its backing store (closed
// automatically) and an [http.Client] that keeps cookies across requests
// like a browser does.
func newTestServer(t *testing.T) (*httptest.Server, *store.Store, *http.Client) {
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

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return ts, db, &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// login drives the login form to completion and returns the resulting
// response (the redirect to /parent/children, with the session cookie set
// in client's jar).
func login(t *testing.T, ts *httptest.Server, client *http.Client, tp testParent) *http.Response {
	t.Helper()
	form := url.Values{
		"email":    {tp.email},
		"password": {tp.password},
		"totp":     {tp.code(t, time.Now())},
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/parent/login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("login POST: %v", err)
	}
	return resp
}

// get issues a same-origin GET and returns the response with its body read
// fully, so callers can inspect status and body together.
func get(t *testing.T, client *http.Client, rawURL string) (*http.Response, string) {
	t.Helper()
	resp, err := client.Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	_ = resp.Body.Close()
	return resp, string(body)
}

// postForm issues a POST with sameOrigin controlling whether Sec-Fetch-Site
// is set to "same-origin" (mimicking a real form submission from the panel
// itself) or omitted entirely (mimicking a cross-site page, which has no
// legitimate way to set that header to same-origin).
func postForm(t *testing.T, client *http.Client, rawURL string, form url.Values, sameOrigin bool) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sameOrigin {
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", rawURL, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	_ = resp.Body.Close()
	return resp, string(body)
}

var csrfFieldRE = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func extractCSRF(t *testing.T, body string) string {
	t.Helper()
	m := csrfFieldRE.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf_token field found in body: %s", body)
	}
	return m[1]
}

func TestParentPageWithoutSessionIsRejected(t *testing.T) {
	ts, _, client := newTestServer(t)

	resp, _ := get(t, client, ts.URL+"/parent/children")
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /parent/children with no session: got status %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/parent/login" {
		t.Errorf("GET /parent/children with no session: got redirect %q, want /parent/login", loc)
	}
}

func TestParentPageWithChildSessionIsRejected(t *testing.T) {
	ts, db, client := newTestServer(t)

	child := &store.Child{ID: "child-1", ParentID: "parent-1", Pseudo: "Timéo"}
	if err := db.CreateParent(t.Context(), &store.Parent{ID: "parent-1", Email: "p@example.test", PassHash: []byte("x")}); err != nil {
		t.Fatalf("CreateParent: %v", err)
	}
	if err := db.CreateChild(t.Context(), child); err != nil {
		t.Fatalf("CreateChild: %v", err)
	}
	token, err := auth.CreateSession(t.Context(), db, store.SessionChild, child.ID, time.Now())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	client.Jar.SetCookies(u, []*http.Cookie{{Name: auth.CookieParent, Value: token}})

	resp, _ := get(t, client, ts.URL+"/parent/children")
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /parent/children with a child session under the parent cookie: got status %d, want %d (redirect to login)", resp.StatusCode, http.StatusSeeOther)
	}
}

func TestLoginThenChildrenPage(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)

	loginResp := login(t, ts, client, tp)
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: got status %d, want %d", loginResp.StatusCode, http.StatusSeeOther)
	}

	resp, body := get(t, client, ts.URL+"/parent/children")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /parent/children after login: got status %d, want 200; body: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "Ajouter un enfant") {
		t.Errorf("GET /parent/children after login: body missing the add-child form")
	}
}

func TestCreateChildRequiresCSRFToken(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}}
	resp, _ := postForm(t, client, ts.URL+"/parent/children", form, true)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST /parent/children without csrf_token: got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("POST /parent/children without csrf_token created %d children, want 0", len(children))
	}
}

func TestCreateChildRejectsCrossSiteRequest(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)

	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	resp, _ := postForm(t, client, ts.URL+"/parent/children", form, false)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST /parent/children with no same-origin signal: got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestCreateChildWithAnotherSessionsCSRFTokenIsRejected(t *testing.T) {
	ts, db, clientA := newTestServer(t)
	tpA := seedParent(t, db)
	login(t, ts, clientA, tpA)
	_, bodyA := get(t, clientA, ts.URL+"/parent/children")
	_ = bodyA // clientA's own CSRF token is not what this test needs.

	// A second parent, in a second session, has a CSRF token of its own —
	// one bound to a session token clientA's requests never carry.
	jarB, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	clientB := &http.Client{Jar: jarB, CheckRedirect: clientA.CheckRedirect}
	tpB := seedParentWithID(t, db, "parent-2", "parent2@example.test")
	login(t, ts, clientB, tpB)
	_, bodyB := get(t, clientB, ts.URL+"/parent/children")
	foreignCSRF := extractCSRF(t, bodyB)

	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {foreignCSRF}}
	resp, _ := postForm(t, clientA, ts.URL+"/parent/children", form, true)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST /parent/children with another session's csrf token: got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	children, err := db.ChildrenOfParent(t.Context(), tpA.id)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("POST with a foreign csrf token created %d children, want 0", len(children))
	}
}

func TestChildCreationDefaultsSosiesOff(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)

	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /parent/children: got status %d, want 200; body: %s", resp.StatusCode, respBody)
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("ChildrenOfParent: got %d children, want 1", len(children))
	}

	var settings struct {
		Rules map[string]bool `json:"rules"`
	}
	if err := json.Unmarshal(children[0].Settings, &settings); err != nil {
		t.Fatalf("unmarshaling settings: %v", err)
	}
	for _, ruleID := range []string{"sosies.a_a", "sosies.et_est", "sosies.on_ont", "sosies.son_sont"} {
		if settings.Rules[ruleID] {
			t.Errorf("newly created child: rule %s is on, want off (Sosies default off)", ruleID)
		}
	}
	if !settings.Rules["masquees.ou"] {
		t.Errorf("newly created child: rule masquees.ou is off, want on")
	}
}

func TestDailyLimitIsRespectedAndModifiable(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)
	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	if resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true); resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil || len(children) != 1 {
		t.Fatalf("ChildrenOfParent: %v, %d children", err, len(children))
	}
	childID := children[0].ID

	var limit struct {
		MinutesPerDay int `json:"minutes_per_day"`
	}
	if err := json.Unmarshal(children[0].DailyLimit, &limit); err != nil {
		t.Fatalf("unmarshaling default limit: %v", err)
	}
	if limit.MinutesPerDay != 20 {
		t.Errorf("default daily limit: got %d minutes, want 20", limit.MinutesPerDay)
	}

	_, settingsBody := get(t, client, ts.URL+"/parent/children/"+childID+"/settings")
	settingsCSRF := extractCSRF(t, settingsBody)

	saveForm := url.Values{
		"minutes_per_day": {"45"},
		"keyboard":        {"abc"},
		"csrf_token":      {settingsCSRF},
	}
	resp, saveBody := postForm(t, client, ts.URL+"/parent/children/"+childID+"/settings", saveForm, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("saving settings: got status %d; body: %s", resp.StatusCode, saveBody)
	}

	updated, err := db.ChildByID(t.Context(), childID)
	if err != nil {
		t.Fatalf("ChildByID: %v", err)
	}
	if err := json.Unmarshal(updated.DailyLimit, &limit); err != nil {
		t.Fatalf("unmarshaling updated limit: %v", err)
	}
	if limit.MinutesPerDay != 45 {
		t.Errorf("after saving settings: got %d minutes, want 45", limit.MinutesPerDay)
	}
}

func TestPseudoWithScriptTagIsEscaped(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)

	const malicious = `<script>x</script>` // 19 runes, under pseudoMaxLen
	form := url.Values{"pseudo": {malicious}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}

	if strings.Contains(respBody, malicious) {
		t.Errorf("rendered page contains the raw <script> tag unescaped")
	}
	if !strings.Contains(respBody, "&lt;script&gt;x&lt;/script&gt;") {
		t.Errorf("rendered page does not contain the escaped pseudo; body: %s", respBody)
	}
}

func TestAZERTYIsNotOfferedPlainlyInPortrait(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)
	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	if resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true); resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil || len(children) != 1 {
		t.Fatalf("ChildrenOfParent: %v, %d children", err, len(children))
	}

	_, settingsBody := get(t, client, ts.URL+"/parent/children/"+children[0].ID+"/settings")
	if !strings.Contains(settingsBody, `value="azerty"`) {
		t.Fatalf("settings page does not mention the azerty option at all; body: %s", settingsBody)
	}

	azertyTag := azertyInputRE.FindString(settingsBody)
	if azertyTag == "" {
		t.Fatalf("no <input ...value=%q...> tag found in settings body", "azerty")
	}
	if !strings.Contains(azertyTag, "disabled") {
		t.Errorf("AZERTY radio is not disabled by default (portrait phone assumed): %s", azertyTag)
	}
}

// azertyInputRE matches the AZERTY radio input's whole tag, across the
// multiple lines the template renders its attributes on. "[^>]" (not
// "(?s).") is what makes it span lines: a negated character class matches a
// newline whether or not the "s" flag is set.
var azertyInputRE = regexp.MustCompile(`<input[^>]*value="azerty"[^>]*>`)

func TestLoginRejectsWrongPassword(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)

	form := url.Values{"email": {tp.email}, "password": {"wrong password"}, "totp": {tp.code(t, time.Now())}}
	resp, body := postForm(t, client, ts.URL+"/parent/login", form, true)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("login with wrong password: got status %d, want 200 (form redisplayed)", resp.StatusCode)
	}
	if !strings.Contains(body, "identifiants ou code invalides") {
		t.Errorf("login with wrong password: body missing the generic rejection message")
	}

	// Rejected: no session cookie was set, so the children page still
	// redirects to login.
	pageResp, _ := get(t, client, ts.URL+"/parent/children")
	if pageResp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /parent/children after a rejected login: got status %d, want %d", pageResp.StatusCode, http.StatusSeeOther)
	}
}

func TestLoginRejectsWrongTOTPCode(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)

	form := url.Values{"email": {tp.email}, "password": {tp.password}, "totp": {"000000"}}
	resp, body := postForm(t, client, ts.URL+"/parent/login", form, true)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("login with wrong TOTP code: got status %d, want 200 (form redisplayed)", resp.StatusCode)
	}
	if !strings.Contains(body, "identifiants ou code invalides") {
		t.Errorf("login with wrong TOTP code: body missing the generic rejection message")
	}
}

func TestLoginGetRedirectsToChildrenWhenAlreadyLoggedIn(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	resp, _ := get(t, client, ts.URL+"/parent/login")
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /parent/login while already logged in: got status %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/parent/children" {
		t.Errorf("GET /parent/login while already logged in: got redirect %q, want /parent/children", loc)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)

	form := url.Values{"csrf_token": {csrfToken}}
	resp, _ := postForm(t, client, ts.URL+"/parent/logout", form, true)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /parent/logout: got status %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	pageResp, _ := get(t, client, ts.URL+"/parent/children")
	if pageResp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /parent/children after logout: got status %d, want %d (session gone)", pageResp.StatusCode, http.StatusSeeOther)
	}
}

func TestSettingsRejectsInvalidMinutesPerDay(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)
	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	if resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true); resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}
	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil || len(children) != 1 {
		t.Fatalf("ChildrenOfParent: %v, %d children", err, len(children))
	}
	childID := children[0].ID

	_, settingsBody := get(t, client, ts.URL+"/parent/children/"+childID+"/settings")
	settingsCSRF := extractCSRF(t, settingsBody)

	saveForm := url.Values{"minutes_per_day": {"not-a-number"}, "keyboard": {"abc"}, "csrf_token": {settingsCSRF}}
	resp, respBody := postForm(t, client, ts.URL+"/parent/children/"+childID+"/settings", saveForm, true)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("saving invalid minutes_per_day: got status %d, want 200 (form redisplayed with an error)", resp.StatusCode)
	}
	if !strings.Contains(respBody, "doit être un nombre de minutes") {
		t.Errorf("saving invalid minutes_per_day: body missing the validation message")
	}

	updated, err := db.ChildByID(t.Context(), childID)
	if err != nil {
		t.Fatalf("ChildByID: %v", err)
	}
	var limit struct {
		MinutesPerDay int `json:"minutes_per_day"`
	}
	if err := json.Unmarshal(updated.DailyLimit, &limit); err != nil {
		t.Fatalf("unmarshaling limit: %v", err)
	}
	if limit.MinutesPerDay != defaultMinutesPerDayForTest {
		t.Errorf("invalid minutes_per_day was still saved: got %d, want unchanged default %d", limit.MinutesPerDay, defaultMinutesPerDayForTest)
	}
}

// defaultMinutesPerDayForTest mirrors the unexported parent.defaultMinutesPerDay
// constant for this external test package.
const defaultMinutesPerDayForTest = 20

func TestSettingsForAnotherParentsChildIsRejected(t *testing.T) {
	ts, db, clientA := newTestServer(t)
	tpA := seedParent(t, db)
	login(t, ts, clientA, tpA)

	_, body := get(t, clientA, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)
	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	if resp, respBody := postForm(t, clientA, ts.URL+"/parent/children", form, true); resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}
	childrenA, err := db.ChildrenOfParent(t.Context(), tpA.id)
	if err != nil || len(childrenA) != 1 {
		t.Fatalf("ChildrenOfParent: %v, %d children", err, len(childrenA))
	}

	jarB, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	clientB := &http.Client{Jar: jarB, CheckRedirect: clientA.CheckRedirect}
	tpB := seedParentWithID(t, db, "parent-other", "other@example.test")
	login(t, ts, clientB, tpB)

	resp, _ := get(t, clientB, ts.URL+"/parent/children/"+childrenA[0].ID+"/settings")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET another parent's child's settings: got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSettingsForUnknownChildIsRejected(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	resp, _ := get(t, client, ts.URL+"/parent/children/does-not-exist/settings")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET an unknown child's settings: got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestCreateChildRejectsEmptyPseudo(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)

	form := url.Values{"pseudo": {""}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("creating a child with an empty pseudo: got status %d, want 200 (form redisplayed)", resp.StatusCode)
	}
	if !strings.Contains(respBody, "le pseudo doit faire") {
		t.Errorf("creating a child with an empty pseudo: body missing the validation message")
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("creating a child with an empty pseudo created %d children, want 0", len(children))
	}
}

func TestCreateChildRejectsInvalidPattern(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)

	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"12"}, "csrf_token": {csrfToken}}
	resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("creating a child with an invalid pattern: got status %d, want 200 (form redisplayed)", resp.StatusCode)
	}
	if !strings.Contains(respBody, "le motif doit avoir") {
		t.Errorf("creating a child with an invalid pattern: body missing the validation message")
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("creating a child with an invalid pattern created %d children, want 0", len(children))
	}
}

// TestCreateChildRejectsWeakPattern is encre-qpx.4's blacklist requirement:
// a pattern of the right length is still rejected if it is one of the
// handful of shapes a class of seven-year-olds converges on.
func TestCreateChildRejectsWeakPattern(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	for _, weak := range []string{"11111", "12345", "54321", "13179"} {
		_, body := get(t, client, ts.URL+"/parent/children")
		csrfToken := extractCSRF(t, body)

		form := url.Values{"pseudo": {"Timéo"}, "pattern": {weak}, "csrf_token": {csrfToken}}
		resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("creating a child with weak pattern %q: got status %d, want 200 (form redisplayed)", weak, resp.StatusCode)
		}
		if !strings.Contains(respBody, "trop facile à deviner") {
			t.Errorf("creating a child with weak pattern %q: body missing the validation message", weak)
		}
	}

	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil {
		t.Fatalf("ChildrenOfParent: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("creating children with weak patterns created %d children, want 0", len(children))
	}
}

func TestSettingsPostRequiresCSRFToken(t *testing.T) {
	ts, db, client := newTestServer(t)
	tp := seedParent(t, db)
	login(t, ts, client, tp)

	_, body := get(t, client, ts.URL+"/parent/children")
	csrfToken := extractCSRF(t, body)
	form := url.Values{"pseudo": {"Timéo"}, "pattern": {"13579"}, "csrf_token": {csrfToken}}
	if resp, respBody := postForm(t, client, ts.URL+"/parent/children", form, true); resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child: got status %d; body: %s", resp.StatusCode, respBody)
	}
	children, err := db.ChildrenOfParent(t.Context(), tp.id)
	if err != nil || len(children) != 1 {
		t.Fatalf("ChildrenOfParent: %v, %d children", err, len(children))
	}

	saveForm := url.Values{"minutes_per_day": {"30"}, "keyboard": {"abc"}}
	resp, _ := postForm(t, client, ts.URL+"/parent/children/"+children[0].ID+"/settings", saveForm, true)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST settings without csrf_token: got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	updated, err := db.ChildByID(t.Context(), children[0].ID)
	if err != nil {
		t.Fatalf("ChildByID: %v", err)
	}
	var limit struct {
		MinutesPerDay int `json:"minutes_per_day"`
	}
	if err := json.Unmarshal(updated.DailyLimit, &limit); err != nil {
		t.Fatalf("unmarshaling limit: %v", err)
	}
	if limit.MinutesPerDay != 20 {
		t.Errorf("settings changed despite the missing csrf_token: got %d minutes, want unchanged 20", limit.MinutesPerDay)
	}
}
