package api_test

import (
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/oioio-space/encre/engine"
)

// runStartResponse mirrors enough of the real response for tests to read
// out of it without importing the unexported request/response types.
type runStartResponse struct {
	RunID   string
	Deck    engine.Deck
	Rank    int
	Targets [3]float64
	Audio   map[string]string
}

func startRun(t *testing.T, ts *testServer, gardeIDs []string) (*http.Response, runStartResponse) {
	t.Helper()
	resp := ts.post(t, "/api/v1/run/start", map[string]any{"gardeIDs": gardeIDs})
	if resp.StatusCode != http.StatusOK {
		return resp, runStartResponse{}
	}
	return resp, decodeBody[runStartResponse](t, resp)
}

func TestRunStartReturnsDeckWithAudioURLs(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")

	resp, got := startRun(t, ts, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("run/start status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got.RunID == "" {
		t.Error("RunID is empty")
	}
	if len(got.Deck.Week) != 1 || got.Deck.Week[0].ID != "chat" {
		t.Fatalf("Deck.Week = %+v, want [chat]", got.Deck.Week)
	}
	if got.Audio["chat"] != "/media/chat.ogg" {
		t.Errorf("Audio[chat] = %q, want %q", got.Audio["chat"], "/media/chat.ogg")
	}
}

// TestRunStartRefusesWhenTimeIsExhausted proves the acceptance criterion:
// once today's play time is spent, start must refuse rather than open
// another run, and say so with a client (not server) error status.
func TestRunStartRefusesWhenTimeIsExhausted(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	setDailyLimitMinutes(t, ts.DB, "child1", 1) // 60 seconds
	loginChild(t, ts, "Mia", "1379")

	day := int32(ts.Now().Unix() / (24 * 60 * 60))
	if err := ts.DB.AddPlayTime(t.Context(), "child1", day, 60); err != nil {
		t.Fatalf("AddPlayTime() error = %v", err)
	}

	resp, _ := startRun(t, ts, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("run/start once time is exhausted: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestRunStartRequiresSession(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := startRun(t, ts, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("run/start with no session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestHeartbeatDecrementsRemainingTimeAndStartRespectsIt drives heartbeat
// through the real endpoint and checks that the time it records is the same
// time run/start later refuses on — proving the two share one source of
// truth rather than heartbeat updating something start never reads.
func TestHeartbeatDecrementsRemainingTimeAndStartRespectsIt(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	setDailyLimitMinutes(t, ts.DB, "child1", 1) // 60 seconds
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")

	_, first := startRun(t, ts, nil)
	if first.RunID == "" {
		t.Fatal("first run/start did not return a RunID")
	}

	resp := ts.post(t, "/api/v1/run/"+first.RunID+"/heartbeat", map[string]int{"seconds": 30})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type heartbeatResponse struct{ RemainingSeconds int }
	got := decodeBody[heartbeatResponse](t, resp)
	if got.RemainingSeconds != 30 {
		t.Errorf("RemainingSeconds after a 30s heartbeat on a 60s budget = %d, want 30", got.RemainingSeconds)
	}

	resp2 := ts.post(t, "/api/v1/run/"+first.RunID+"/heartbeat", map[string]int{"seconds": 30})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second heartbeat status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	got2 := decodeBody[heartbeatResponse](t, resp2)
	if got2.RemainingSeconds != 0 {
		t.Errorf("RemainingSeconds after 60s of heartbeats on a 60s budget = %d, want 0", got2.RemainingSeconds)
	}

	startResp, _ := startRun(t, ts, nil)
	if startResp.StatusCode != http.StatusForbidden {
		t.Errorf("run/start after the heartbeat budget is spent: status = %d, want %d", startResp.StatusCode, http.StatusForbidden)
	}
}

func TestHeartbeatRejectsOutOfBoundsSeconds(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	for _, seconds := range []int{0, -1, 121} {
		resp := ts.post(t, "/api/v1/run/"+run.RunID+"/heartbeat", map[string]int{"seconds": seconds})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("heartbeat with seconds=%d: status = %d, want %d", seconds, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// finishBody is the JSON body of a finish request. The response can only
// ever be what engine.Replay computes from attempts — there is no field
// here for a client-supplied score, combo or target, which is itself part
// of what proves ENCRE_04 §4's server authority.
func finishBody(attempts []engine.Attempt) map[string]any {
	return map[string]any{
		"attempts":  attempts,
		"talismans": []engine.TalismanID{},
		"revanche":  [3]bool{},
		"cahier":    false,
	}
}

// TestRunFinishRecomputesScoreFromAttempts fails if a future change wires a
// client-trusted score through instead of engine.Replay's own result.
func TestRunFinishRecomputesScoreFromAttempts(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	attempts := []engine.Attempt{{WordID: "chat", Manche: 0, Correct: true, Typed: "chat"}}
	resp := ts.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("run/finish status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type finishResponse struct {
		Outcome engine.Outcome
	}
	got := decodeBody[finishResponse](t, resp)
	// One correct attempt on a trap-free word ("chat" carries no Traps here)
	// at combo 1 scores exactly its letter count: 4. Anything else means
	// something other than Replay decided the score.
	if got.Outcome.Scores[0] != 4 {
		t.Errorf("Outcome.Scores[0] = %v, want 4 (Replay's own computation)", got.Outcome.Scores[0])
	}
}

// TestRunFinishAppliedOnlyOnce is the test encre-qpx.3 asks for: a run
// finished twice must be refused the second time through the real
// handler — not engine.Apply called directly — and the child's state must
// only have moved once.
func TestRunFinishAppliedOnlyOnce(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	attempts := []engine.Attempt{{WordID: "chat", Manche: 0, Correct: true, Typed: "chat"}}
	first := ts.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts))
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first run/finish status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	childAfterFirst, err := ts.DB.ChildByID(t.Context(), "child1")
	if err != nil {
		t.Fatalf("ChildByID() error = %v", err)
	}

	second := ts.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts))
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second run/finish status = %d, want %d", second.StatusCode, http.StatusConflict)
	}

	childAfterSecond, err := ts.DB.ChildByID(t.Context(), "child1")
	if err != nil {
		t.Fatalf("ChildByID() error = %v", err)
	}
	if childAfterSecond.Engine().Base != childAfterFirst.Engine().Base {
		t.Errorf("child's Base rolling rate moved on the rejected second finish: %+v -> %+v",
			childAfterFirst.Engine().Base, childAfterSecond.Engine().Base)
	}

	statesAfterFirst, err := ts.DB.WordStates(t.Context(), "child1")
	if err != nil {
		t.Fatalf("WordStates() error = %v", err)
	}
	if statesAfterFirst["chat"] == nil || len(statesAfterFirst["chat"].SuccessDays) != 1 {
		t.Errorf("word state for chat after two finishes = %+v, want exactly one recorded success day", statesAfterFirst["chat"])
	}
}

func TestRunFinishRejectsAnotherChildsRun(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Leo", "2468")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	// A second client, its own cookie jar, logged in as the other child.
	otherJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	ts.Client = &http.Client{Jar: otherJar}
	loginChild(t, ts, "Leo", "2468")

	attempts := []engine.Attempt{{WordID: "chat", Manche: 0, Correct: true, Typed: "chat"}}
	resp := ts.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("finishing another child's run: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestRunFinishRejectsOversizedAttempts(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	attempts := make([]engine.Attempt, 201)
	for i := range attempts {
		attempts[i] = engine.Attempt{WordID: "chat", Manche: 0}
	}
	resp := ts.post(t, "/api/v1/run/"+run.RunID+"/finish", finishBody(attempts))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("finish with 201 attempts: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRunRoomRejectsUnofferedChoice(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	resp := ts.post(t, "/api/v1/run/"+run.RunID+"/room", map[string]any{"index": 0, "roomID": "Salle inexistante"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("room with an unoffered choice: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRunRoomAcceptsAnOfferedChoice(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	resp := ts.post(t, "/api/v1/run/"+run.RunID+"/room", map[string]any{"index": 0, "roomID": run.Deck.Rooms[0][0]})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("room with an offered choice: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestRunRoomRejectsAnotherChildsRun(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedChild(t, ts.DB, "child2", "Leo", "2468")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	otherJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	ts.Client = &http.Client{Jar: otherJar}
	loginChild(t, ts, "Leo", "2468")

	resp := ts.post(t, "/api/v1/run/"+run.RunID+"/room", map[string]any{"index": 0, "roomID": run.Deck.Rooms[0][0]})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("room on another child's run: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestFinishRejectsDuplicateJSONKey and TestFinishRejectsInvalidUTF8 check
// the Go 1.27 note on this ticket: encoding/json/v2's stricter defaults must
// actually be reachable through the handler, not just available in the
// standard library.
func TestFinishRejectsDuplicateJSONKey(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	body := `{"attempts":[],"attempts":[],"talismans":[],"revanche":[false,false,false],"cahier":false}`
	resp := ts.postRaw(t, "/api/v1/run/"+run.RunID+"/finish", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("finish with a duplicate JSON key: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestFinishRejectsInvalidUTF8(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	body := "{\"attempts\":[{\"WordID\":\"chat\",\"Manche\":0,\"Typed\":\"\xff\xfe\"}]}"
	resp := ts.postRaw(t, "/api/v1/run/"+run.RunID+"/finish", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("finish with invalid UTF-8: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestFinishRejectsOversizedBody(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	seedValidatedWord(t, ts.DB, "child1", "chat", "chat")
	loginChild(t, ts, "Mia", "1379")
	_, run := startRun(t, ts, nil)

	// A single attempt's JSON runs under 100 bytes; 20,000 of them is well
	// past maxBodyBytes (1 MiB), so this exercises the byte cap rather than
	// the separate attempt-count cap TestRunFinishRejectsOversizedAttempts
	// checks.
	huge := `{"attempts":[` + repeatAttempt(20_000) + `]}`
	resp := ts.postRaw(t, "/api/v1/run/"+run.RunID+"/finish", huge)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("finish with an oversized body: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func repeatAttempt(n int) string {
	const one = `{"WordID":"chat","Manche":0,"Correct":true,"Typed":"chat"},`
	s := ""
	for range n {
		s += one
	}
	return s[:len(s)-1]
}
