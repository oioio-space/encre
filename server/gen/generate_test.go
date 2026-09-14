package gen_test

import (
	"context"
	"errors"
	"testing"

	"github.com/oioio-space/encre/server/gen"
)

// fakeClient is a [gen.Client] that never touches the network: it answers
// with a fixed string or a fixed error, so every test in this file is a
// pure function of that answer.
type fakeClient struct {
	reply string
	err   error
}

func (f fakeClient) Complete(context.Context, string) (string, error) {
	return f.reply, f.err
}

func wl() gen.Whitelist {
	return gen.Whitelist{"le": true, "chat": true, "dort": true, "sur": true, "lit": true}
}

// TestGenerateSentencesKeepsOnlyFilterAcceptedOnes drives the whole path
// with a realistic reply: two valid sentences, one too long, one with a
// non-whitelisted word.
func TestGenerateSentencesKeepsOnlyFilterAcceptedOnes(t *testing.T) {
	client := fakeClient{reply: `{"mot": "chat", "phrases": [
		{"texte": "Le chat dort sur le lit.", "cible": "chat", "forme": "chat"},
		{"texte": "Le chat somnole paisiblement sur le lit tous les soirs de la semaine.", "cible": "chat", "forme": "chat"},
		{"texte": "Le chat ronronne sur le lit.", "cible": "chat", "forme": "chat"},
		{"texte": "Le chat dort sur le canapé.", "cible": "chat", "forme": "chat"}
	]}`}

	got, err := gen.GenerateSentences(t.Context(), client, gen.WordRequest{Mot: "chat"}, wl())
	if err != nil {
		t.Fatalf("GenerateSentences() error = %v", err)
	}
	if len(got.Accepted) != 1 {
		t.Errorf("len(Accepted) = %d, want 1: %+v", len(got.Accepted), got.Accepted)
	}
	if got.Rejected != 3 {
		t.Errorf("Rejected = %d, want 3", got.Rejected)
	}
}

// TestGenerateSentencesReturnsErrorOnMalformedJSON is the ticket's third
// named acceptance case: "une réponse JSON malformée n'explose pas" — it
// must come back as an error, never a panic.
func TestGenerateSentencesReturnsErrorOnMalformedJSON(t *testing.T) {
	client := fakeClient{reply: `{"mot": "chat", "phrases": [`}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GenerateSentences() panicked on malformed JSON: %v", r)
		}
	}()
	_, err := gen.GenerateSentences(t.Context(), client, gen.WordRequest{Mot: "chat"}, wl())
	if err == nil {
		t.Error("GenerateSentences() with malformed JSON: want error, got nil")
	}
}

// TestGenerateSentencesPropagatesClientError checks the API-unavailable
// path: the caller must be able to tell "generation failed" apart from "the
// model produced nothing worth keeping" — this is the former, and it comes
// back as a wrapped error, not a Result with an empty Accepted.
func TestGenerateSentencesPropagatesClientError(t *testing.T) {
	wantErr := errors.New("connection refused")
	client := fakeClient{err: wantErr}

	_, err := gen.GenerateSentences(t.Context(), client, gen.WordRequest{Mot: "chat"}, wl())
	if !errors.Is(err, wantErr) {
		t.Errorf("GenerateSentences() error = %v, want it to wrap %v", err, wantErr)
	}
}
