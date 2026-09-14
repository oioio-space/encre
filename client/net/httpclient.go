package net

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/oioio-space/encre/engine"
)

// sendTimeout is ENCRE_04 §2's network timeout: 10 s per attempt, so a dead
// connection does not hang the queue, or the child, forever.
const sendTimeout = 10 * time.Second

// HTTPSender sends a [Job] to the game server's POST /run/{id}/finish
// (ENCRE_04 §7), over the same net/http that runs under fetch in a WASM
// build (ENCRE_04 §2's "Réseau WASM").
type HTTPSender struct {
	// BaseURL is the server's API root, e.g. "https://encre.example.com/api/v1",
	// without a trailing slash.
	BaseURL string
	// Client sends the request. NewHTTPSender sets it to sendTimeout; a test
	// replaces it to reach an httptest.Server instead.
	Client *http.Client
}

// NewHTTPSender returns an HTTPSender posting to baseURL, timing out each
// send after sendTimeout.
func NewHTTPSender(baseURL string) *HTTPSender {
	return &HTTPSender{BaseURL: baseURL, Client: &http.Client{Timeout: sendTimeout}}
}

// finishBody is the JSON body of POST /run/{id}/finish, matching
// server/api/run.go's finishRequest field for field.
type finishBody struct {
	Attempts  []engine.Attempt    `json:"attempts"`
	Talismans []engine.TalismanID `json:"talismans,omitzero"`
	Revanche  [3]bool             `json:"revanche"`
	Cahier    bool                `json:"cahier,omitzero"`
}

// SendFinish implements [Sender]. It classifies the HTTP response into a
// [Result] and returns a non-nil error only when the request could not even
// be built — never for a network failure, which is [ResultUnreachable], a
// value the queue can act on rather than an error it must unwind.
func (h *HTTPSender) SendFinish(ctx context.Context, job Job) (Result, error) {
	body, err := json.Marshal(finishBody{
		Attempts:  job.Attempts,
		Talismans: job.Talismans,
		Revanche:  job.Revanche,
		Cahier:    job.Cahier,
	})
	if err != nil {
		return ResultRejected, fmt.Errorf("net: encoding the finish request for run %s: %w", job.RunID, err)
	}

	target := h.BaseURL + "/run/" + url.PathEscape(job.RunID) + "/finish"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return ResultRejected, fmt.Errorf("net: building the finish request for run %s: %w", job.RunID, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.Client.Do(req)
	if err != nil {
		// A timeout, a dropped connection, DNS failure: the server may never
		// have seen this, so the queue should try again, not give up.
		return ResultUnreachable, nil
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		return ResultApplied, nil
	case resp.StatusCode == http.StatusConflict:
		return ResultAlreadyApplied, nil
	case resp.StatusCode >= http.StatusInternalServerError:
		return ResultUnreachable, nil
	default:
		return ResultRejected, nil
	}
}
