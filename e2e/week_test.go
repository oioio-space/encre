package e2e

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/api"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/parent"
	"github.com/oioio-space/encre/server/store"
)

// weekWord is the one word this file's journey plays: common enough that
// the embedded lexicon is confident about it (see
// server/parent/week_test.go's own TestSemaineTenWordsUnderInteractionBudget,
// which pastes it among ten words that all clear the 0.6 confidence bar
// with no "confirmer" tap), and short enough that its score is easy to
// reason about by hand: four letters, no traps recorded here.
const weekWord = "chat"

// weekJourney is a running week's worth of real HTTP servers sharing one
// store: [parent.Server], playing the parent's own browser, and
// [api.Server], playing the child's device — the two halves of ENCRE_04 a
// production deployment runs side by side against the same database, and
// the only way a bug in the seam between them (a column one side writes
// that the other reads differently, a validated flag the play side does not
// actually check) would ever surface in a test.
type weekJourney struct {
	db  *store.Store
	now *time.Time

	parentURL    string
	parentClient *http.Client

	api *testServer // wraps the api.Server httptest instance; see helpers_test.go

	childID string
}

// newWeekJourney seeds one parent, has them create one child and paste and
// validate a one-word list through the real /parent HTTP handlers, then
// logs that child into the real /api/v1 handlers — the "du collage du
// parent à la run appliquée" bead encre-qpx.2 asks for, stopping just short
// of the first run so every test built on this can start theirs under
// whatever attempts it needs.
func newWeekJourney(t *testing.T) *weekJourney {
	t.Helper()
	db := openTestDB(t)
	pep := testPepper(t)
	now := time.Unix(1_700_000_000, 0).UTC() // a Monday, arbitrarily

	parentSrv, err := parent.NewServer(db, pep)
	if err != nil {
		t.Fatalf("parent.NewServer() error = %v", err)
	}
	parentSrv.SetMediaRoot(t.TempDir())
	parentHS := httptest.NewServer(parentSrv)
	t.Cleanup(parentHS.Close)

	apiSrv := api.New(db, engine.DefaultConfig(), func() time.Time { return now }, pep)
	apiHS := httptest.NewServer(apiSrv.Handler())
	t.Cleanup(apiHS.Close)

	parentJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	parentClient := &http.Client{
		Jar: parentJar,
		// The parent panel answers a successful form post with a redirect;
		// this journey wants to read each step's own response (and, for the
		// CSRF token, the page it left on), not the page the browser would
		// land on next.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	j := &weekJourney{
		db: db, now: &now,
		parentURL: parentHS.URL, parentClient: parentClient,
	}

	tp := seedParentAccount(t, db, pep, "parent-mia")
	j.parentLogin(t, tp)
	j.childID = j.createChild(t, "Mia", "13579")
	j.pasteAndValidateList(t, j.childID, weekWord)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	j.api = &testServer{Server: apiHS, Client: &http.Client{Jar: jar}, DB: db, now: &now}
	loginChild(t, j.api, "Mia", "13579")

	return j
}

// Advance moves the journey's shared clock forward by d — both servers read
// the same *time.Time, so a run/start on the api side after Advance sees
// the same "now" a parent-side query would.
func (j *weekJourney) Advance(d time.Duration) { *j.now = j.now.Add(d) }

// weekTestParent is a parent account seeded directly through the store,
// with password and TOTP already set up, mirroring
// server/parent/server_test.go's own seedParentWithID — duplicated here
// rather than imported for the reason helpers_test.go's own doc comment
// gives for testServer.
type weekTestParent struct {
	id, email, password, totpSecret string
}

func seedParentAccount(t *testing.T, db *store.Store, pep *auth.Pepper, id string) weekTestParent {
	t.Helper()
	email := id + "@example.test"
	hash, err := auth.HashPassword("correct horse battery staple", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	enrollment, err := auth.EnrollTOTP(email)
	if err != nil {
		t.Fatalf("EnrollTOTP() error = %v", err)
	}
	sealed, err := pep.Encrypt([]byte(enrollment.Secret))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	p := &store.Parent{
		ID: id, Email: email, PassHash: []byte(hash), TOTPSecret: sealed,
		FamilyCode: id + "-family", CreatedAt: time.Now(),
	}
	if err := db.CreateParent(t.Context(), p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}
	return weekTestParent{id: id, email: email, password: "correct horse battery staple", totpSecret: enrollment.Secret}
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

// parentGet issues a same-origin GET against the parent panel.
func (j *weekJourney) parentGet(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	resp, err := j.parentClient.Get(j.parentURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET %s body: %v", path, err)
	}
	return resp, string(raw)
}

// parentPostForm issues a same-origin POST against the parent panel, the
// only origin its CSRF check accepts (server/parent/csrf.go's
// requireSameOrigin).
func (j *weekJourney) parentPostForm(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, j.parentURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := j.parentClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading POST %s body: %v", path, err)
	}
	return resp, string(raw)
}

// parentLogin drives the real /parent/login form (email, password, a fresh
// TOTP code) to completion.
func (j *weekJourney) parentLogin(t *testing.T, tp weekTestParent) {
	t.Helper()
	code, err := totp.GenerateCode(tp.totpSecret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}
	form := url.Values{"email": {tp.email}, "password": {tp.password}, "totp": {code}}
	resp, body := j.parentPostForm(t, "/parent/login", form)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("parent login: status = %d, want %d; body: %s", resp.StatusCode, http.StatusSeeOther, body)
	}
}

// createChild drives the real "add a child" form and returns the new
// child's ID, read back from the store rather than parsed out of the
// response — [Server.handleChildrenPost] re-renders the children list on
// success, not a page naming the ID directly.
func (j *weekJourney) createChild(t *testing.T, pseudo, pattern string) string {
	t.Helper()
	_, body := j.parentGet(t, "/parent/children")
	form := url.Values{"pseudo": {pseudo}, "pattern": {pattern}, "csrf_token": {extractCSRF(t, body)}}
	resp, respBody := j.parentPostForm(t, "/parent/children", form)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("creating child %q: status = %d; body: %s", pseudo, resp.StatusCode, respBody)
	}
	children, err := j.db.DB().QueryContext(t.Context(), `SELECT id FROM children WHERE pseudo = ?`, pseudo)
	if err != nil {
		t.Fatalf("querying created child: %v", err)
	}
	defer func() { _ = children.Close() }()
	if !children.Next() {
		t.Fatalf("created child %q not found", pseudo)
	}
	var id string
	if err := children.Scan(&id); err != nil {
		t.Fatalf("scanning created child id: %v", err)
	}
	return id
}

// pasteAndValidateList drives "coller une liste, la valider" to completion
// for childID: paste words as a kindWord list, confirm any item the
// embedded lexicon is unsure about (server/parent/week_test.go's own
// TestSemaineTenWordsUnderInteractionBudget is the reference for this
// step — a common word like weekWord should need none), then validate with
// a due date. It fails the test if the list is not left [store.WordList.Validated]
// at the end.
func (j *weekJourney) pasteAndValidateList(t *testing.T, childID string, words ...string) string {
	t.Helper()
	_, newBody := j.parentGet(t, "/parent/children/"+childID+"/lists/new")
	form := url.Values{
		"label": {"Semaine"}, "kind": {"mot"}, "raw_text": {strings.Join(words, ", ")},
		"csrf_token": {extractCSRF(t, newBody)},
	}
	resp, _ := j.parentPostForm(t, "/parent/children/"+childID+"/lists", form)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("posting list: status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	listID := loc[strings.LastIndex(loc, "/")+1:]
	_, listBody := j.parentGet(t, strings.TrimPrefix(loc, j.parentURL))

	items, err := j.db.ItemsOfList(t.Context(), listID)
	if err != nil {
		t.Fatalf("ItemsOfList() error = %v", err)
	}
	csrfToken := extractCSRF(t, listBody)
	for _, item := range items {
		if item.Confidence >= 0.6 || item.Confirmed {
			continue
		}
		resp, body := j.parentPostForm(t, "/parent/lists/"+listID+"/items/"+item.ID+"/confirm",
			url.Values{"csrf_token": {csrfToken}})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("confirming %q: status = %d", item.Text, resp.StatusCode)
		}
		csrfToken = extractCSRF(t, body)
	}

	validateResp, validateBody := j.parentPostForm(t, "/parent/lists/"+listID+"/validate",
		url.Values{"due_date": {"2026-09-20"}, "csrf_token": {csrfToken}})
	if validateResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("validating list: status = %d, want %d; body: %s", validateResp.StatusCode, http.StatusSeeOther, validateBody)
	}

	got, err := j.db.ListByID(t.Context(), listID)
	if err != nil {
		t.Fatalf("ListByID() error = %v", err)
	}
	if !got.Validated {
		t.Fatalf("list %s is not validated after the paste-and-validate flow", listID)
	}
	return listID
}

// TestFullWeekJourney is the whole system, once, the way encre-qpx.2 asks
// for it: a parent pastes and validates a real list through server/parent's
// real handlers (which run every word through the real embedded lexique
// lexicon), a child logs into server/api with the real login handler,
// starts a run against the deck server/api built from that validated list,
// plays it, and finishes it — and the store the parent and the child each
// touched is the same *store.Store, so a column one side wrote that the
// other reads differently would show up here and nowhere smaller.
//
// It checks, in order: the run actually contains the parent's word; a
// correct attempt on it moves the child's persisted word state; finishing
// the same run twice is refused the second time, with the state moved only
// once (encre-qpx.2's own acceptance criterion); a second child cannot
// finish the first child's run; and a heartbeat call visibly spends the
// child's daily play-time budget.
func TestFullWeekJourney(t *testing.T) {
	j := newWeekJourney(t)

	_, run := startRun(t, j.api, nil)
	if run.RunID == "" {
		t.Fatal("run/start did not return a RunID")
	}
	if len(run.Deck.Week) != 1 || run.Deck.Week[0].Text != weekWord {
		t.Fatalf("Deck.Week = %+v, want the one word the parent validated (%q)", run.Deck.Week, weekWord)
	}

	attempts := []engine.Attempt{{WordID: run.Deck.Week[0].ID, Manche: 0, Correct: true, Typed: weekWord}}
	first := j.api.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts, [3]bool{}))
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first run/finish: status = %d, want %d", first.StatusCode, http.StatusOK)
	}
	got := decodeBody[finishRunResponse](t, first)
	if got.Outcome.Scores[0] != 4 { // "chat": four letters, no traps recorded here
		t.Errorf("Outcome.Scores[0] = %v, want 4", got.Outcome.Scores[0])
	}

	statesAfterFirst, err := j.db.WordStates(t.Context(), j.childID)
	if err != nil {
		t.Fatalf("WordStates() error = %v", err)
	}
	st := statesAfterFirst[run.Deck.Week[0].ID]
	if st == nil || len(st.SuccessDays) != 1 {
		t.Fatalf("word state after the first finish = %+v, want exactly one recorded success day", st)
	}

	// encre-qpx.2's own acceptance criterion: applied twice is refused the
	// second time, and the state has moved only once.
	second := j.api.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts, [3]bool{}))
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second run/finish: status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
	statesAfterSecond, err := j.db.WordStates(t.Context(), j.childID)
	if err != nil {
		t.Fatalf("WordStates() error = %v", err)
	}
	if len(statesAfterSecond[run.Deck.Week[0].ID].SuccessDays) != 1 {
		t.Errorf("word state moved on the rejected second finish: %+v -> %+v",
			st, statesAfterSecond[run.Deck.Week[0].ID])
	}

	// A sibling cannot finish this child's run (encre-qpx.2's own acceptance
	// criterion: "un enfant ne peut pas terminer la run d'un autre").
	seedChild(t, j.db, "child2", "Leo", "2468")
	siblingJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	sibling := &testServer{Server: j.api.Server, Client: &http.Client{Jar: siblingJar}, DB: j.db, now: j.now}
	loginChild(t, sibling, "Leo", "2468")
	thirdRunResp := sibling.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts, [3]bool{}))
	if thirdRunResp.StatusCode != http.StatusForbidden {
		t.Errorf("a sibling finishing this run: status = %d, want %d", thirdRunResp.StatusCode, http.StatusForbidden)
	}

	// "le temps de jeu est décompté" (encre-qpx.2's own acceptance
	// criterion): a heartbeat visibly spends the child's daily budget.
	hbResp := j.api.post(t, "/api/v1/run/"+run.RunID+"/heartbeat", map[string]int{"seconds": 30})
	if hbResp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat: status = %d, want %d", hbResp.StatusCode, http.StatusOK)
	}
	type heartbeatResponse struct{ RemainingSeconds int }
	hb := decodeBody[heartbeatResponse](t, hbResp)
	const defaultDailyLimitSeconds = 20 * 60 // server/parent/settings.go's defaultMinutesPerDay
	if hb.RemainingSeconds != defaultDailyLimitSeconds-30 {
		t.Errorf("RemainingSeconds after a 30s heartbeat = %d, want %d", hb.RemainingSeconds, defaultDailyLimitSeconds-30)
	}
}

// playWeekWord starts a fresh run, answers weekWord once (correctly or not)
// in manche 0, finishes it through the real handler, and returns the
// Events the finish response carried — a Shine may or may not be among
// them (it is drawn from a run's own random seed), so callers check with
// slices.Contains rather than an exact slice comparison.
func (j *weekJourney) playWeekWord(t *testing.T, correct bool) []engine.Event {
	t.Helper()
	// A child session does not outlive [auth.ChildSessionTTL] (a day), so a
	// week spread across [weekJourney.Advance] calls needs a fresh login
	// each time it crosses that TTL — exactly what a real device would do
	// too, days apart.
	loginChild(t, j.api, "Mia", "13579")
	_, run := startRun(t, j.api, nil)
	if run.RunID == "" || len(run.Deck.Week) == 0 {
		t.Fatalf("run/start did not return a usable deck: %+v", run)
	}
	attempts := []engine.Attempt{{WordID: run.Deck.Week[0].ID, Manche: 0, Correct: correct, Typed: weekWord}}
	resp := j.api.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts, [3]bool{}))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("run/finish: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	return decodeBody[finishRunResponse](t, resp).Events
}

// TestWordLifecycleEventsAcrossAWeek is the other half of encre-qpx.2's
// "les Events émis correspondent aux transitions attendues (dorure,
// ternissure, malédiction)": dorure ([engine.GoldEarned]) and malédiction
// ([engine.Cursed]) both actually fire through the real POST
// /run/{id}/finish handler, driven by [engine.Record] the same way a real
// week of play would reach them — three correct answers spread across
// at least engine.Config.GoldMinSpanDays days for the first, three recent
// wrong answers for the second, three more correct answers in a row to tame
// it back ([engine.Tamed]).
//
// Ternissure ([engine.Tarnished]) is deliberately not exercised here: it is
// only ever raised by [engine.Tarnish], and nothing in server/api's
// handleRunFinish calls it — sim/sim.go is the only caller in the whole
// tree. A real run finishing a real word can gild it and curse it, but
// cannot currently tarnish it; see this package's own doc comment
// (doc_test.go) for where that gap is reported rather than worked around
// here.
func TestWordLifecycleEventsAcrossAWeek(t *testing.T) {
	j := newWeekJourney(t)
	cfg := engine.DefaultConfig()

	// Three correct answers, each on a distinct day, spanning at least
	// cfg.GoldMinSpanDays (7): engine.Record's own condition for
	// [engine.GoldEarned] ("remembered", engine/word.go).
	if got := j.playWeekWord(t, true); slices.Contains(got, engine.GoldEarned) {
		t.Fatalf("GoldEarned fired on the first success: %v, want it to need %d days", got, cfg.GoldDays)
	}
	j.Advance(8 * 24 * time.Hour)
	j.playWeekWord(t, true)
	j.Advance(8 * 24 * time.Hour)
	if got := j.playWeekWord(t, true); !slices.Contains(got, engine.GoldEarned) {
		t.Fatalf("Events after the third success spanning %d days = %v, want GoldEarned among them",
			2*8, got)
	}

	// A wrong answer on a now-gold word loses it.
	if got := j.playWeekWord(t, false); !slices.Contains(got, engine.GoldLost) {
		t.Fatalf("Events after a wrong answer on a gold word = %v, want GoldLost among them", got)
	}
	// cfg.CurseFails (3) recent wrong answers curse the word — the first of
	// the three already landed above (it is what lost the gold), so two
	// more, all still inside cfg.CurseWeeks, complete it.
	j.playWeekWord(t, false)
	if got := j.playWeekWord(t, false); !slices.Contains(got, engine.Cursed) {
		t.Fatalf("Events after %d recent wrong answers = %v, want Cursed among them", cfg.CurseFails, got)
	}

	// tamingRun (3) correct answers in a row, while cursed, tame it back
	// onto gold in the same call.
	j.playWeekWord(t, true)
	j.playWeekWord(t, true)
	if got := j.playWeekWord(t, true); !slices.Contains(got, engine.Tamed) {
		t.Fatalf("Events after 3 correct answers in a row on a cursed word = %v, want Tamed among them", got)
	} else if !slices.Contains(got, engine.GoldEarned) {
		t.Errorf("Events on the taming answer = %v, want GoldEarned alongside Tamed (engine/word.go's Record)", got)
	}
}
