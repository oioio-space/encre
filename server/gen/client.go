package gen

import (
	"bytes"
	"cmp"
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Client answers a single prompt with the model's raw text reply. It exists
// so every test in this package can inject a fake instead of calling the
// real Anthropic API — see [AnthropicClient] for the one that does.
type Client interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// AnthropicAPIKeyEnv is the environment variable [NewAnthropicClient] reads
// the API key from. It is never read from configuration written to disk or
// to the database, by design (encre-wi6's acceptance): a key that leaked
// out of the environment into a backup or a support ticket is a key that
// has to be rotated regardless of who read it.
// #nosec G101 -- this is an environment variable *name*, read at runtime by
// NewAnthropicClient via os.Getenv; no credential value is written here.
const AnthropicAPIKeyEnv = "ANTHROPIC_API_KEY"

// defaultAnthropicModel and defaultAnthropicBaseURL are AnthropicClient's
// defaults, overridable per instance for a test double server or a pinned
// model.
const (
	defaultAnthropicModel   = "claude-sonnet-4-5"
	defaultAnthropicBaseURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion        = "2023-06-01"
	// maxTokens is ENCRE_04 §9's own figure: "max_tokens 800".
	maxTokens = 800
)

// AnthropicClient calls the Anthropic Messages API directly over
// net/http, rather than through github.com/anthropics/anthropic-sdk-go.
//
// That SDK is real, MIT-licensed and v1-stable, and would have been the
// obvious choice for a project doing many kinds of calls against the API.
// This package makes exactly one: one prompt in, one text reply out, with
// no streaming, no tool use and no multi-turn state. Against that single
// use, net/http direct costs a few dozen lines this file already holds and
// buys one fewer dependency to track through go.sum, govulncheck and
// gitleaks/gosec's supply-chain surface — a real cost in a project that
// pins CGO_ENABLED=0 and runs a vulnerability gate on every commit. Pulling
// the SDK in on the day a second, richer use of the API shows up (multi-turn
// justification generation, say) would be the right call then; it is not
// the right call for this one endpoint today.
type AnthropicClient struct {
	// APIKey is required; see [AnthropicAPIKeyEnv].
	APIKey string
	// Model defaults to [defaultAnthropicModel] when empty.
	Model string
	// BaseURL defaults to [defaultAnthropicBaseURL] when empty, for a test
	// double server.
	BaseURL string
	// HTTPClient defaults to [http.DefaultClient] when nil.
	HTTPClient *http.Client
}

// NewAnthropicClient builds an [AnthropicClient] with the API key read from
// [AnthropicAPIKeyEnv]. It returns an error if the variable is unset or
// empty — the caller is expected to fall back to manual sentence entry
// rather than start the server without generation available (ENCRE_04 §9,
// "repli : le parent tape la phrase").
func NewAnthropicClient() (*AnthropicClient, error) {
	key := os.Getenv(AnthropicAPIKeyEnv)
	if key == "" {
		return nil, fmt.Errorf("gen: %s is not set", AnthropicAPIKeyEnv)
	}
	return &AnthropicClient{APIKey: key}, nil
}

// anthropicRequest and anthropicResponse are the small slice of the
// Messages API's shape this package needs — not a full binding of the API,
// only what one single-turn, text-only, non-streaming call reads and
// writes.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicBlock `json:"content"`
	Error   *anthropicError  `json:"error,omitempty"`
}

type anthropicBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Complete implements [Client].
func (c *AnthropicClient) Complete(ctx context.Context, prompt string) (string, error) {
	body, err := jsonv2.Marshal(anthropicRequest{
		Model:     cmp.Or(c.Model, defaultAnthropicModel),
		MaxTokens: maxTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("gen: encoding anthropic request: %w", err)
	}

	url := cmp.Or(c.BaseURL, defaultAnthropicBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gen: building anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gen: calling anthropic api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnthropicResponseBytes))
	if err != nil {
		return "", fmt.Errorf("gen: reading anthropic response: %w", err)
	}

	var ar anthropicResponse
	if err := jsonv2.Unmarshal(raw, &ar); err != nil {
		return "", fmt.Errorf("gen: decoding anthropic response (status %s): %w", resp.Status, err)
	}
	if resp.StatusCode != http.StatusOK {
		if ar.Error != nil {
			return "", fmt.Errorf("gen: anthropic api error (%s): %s", ar.Error.Type, ar.Error.Message)
		}
		return "", fmt.Errorf("gen: anthropic api status %s", resp.Status)
	}
	for _, block := range ar.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("gen: anthropic response carried no text block")
}

// maxAnthropicResponseBytes bounds how much of the API's reply this client
// ever reads: generous for 800 max_tokens of French sentences, and a
// backstop against an unbounded body from a misbehaving proxy or a
// compromised endpoint.
const maxAnthropicResponseBytes = 1 << 20 // 1 MiB
