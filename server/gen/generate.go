package gen

import (
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"strings"
)

// apiSentences is the JSON shape ENCRE_03 §9's prompt asks the model to
// answer with: {"mot": "...", "phrases": [...]}.
type apiSentences struct {
	Mot     string     `json:"mot"`
	Phrases []Sentence `json:"phrases"`
}

// Result is what [GenerateSentences] returns for one word: the sentences
// that passed [Filter], ready to store as unapproved
// ([github.com/oioio-space/encre/server/store.Sentence.Approved] false —
// the parent still taps each one), and how many the model returned but
// [Filter] rejected, for the caller to log or show as "moins de 3 phrases
// générées, complétez à la main".
type Result struct {
	Accepted []Sentence
	Rejected int
}

// GenerateSentences asks client for word's three sentences (ENCRE_03 §9)
// and keeps only the ones [Filter] accepts against wl.
//
// It never panics on what the model sends back: a network failure, a
// non-200 response, or JSON that fails to parse (duplicate keys, invalid
// UTF-8, wrong shape — decoded with encoding/json/v2 for the same
// reasons [github.com/oioio-space/encre/server/api]'s decodeJSON gives)
// all come back as a plain error, for the caller to answer with the manual
// fallback ENCRE_04 §9 asks for. A response that parses but whose
// sentences all fail Filter is not an error at all — Result.Accepted is
// simply empty, and Result.Rejected says why.
func GenerateSentences(ctx context.Context, client Client, word WordRequest, wl Whitelist) (Result, error) {
	raw, err := client.Complete(ctx, BuildPrompt(word))
	if err != nil {
		return Result{}, fmt.Errorf("gen: generating sentences for %q: %w", word.Mot, err)
	}

	var parsed apiSentences
	if err := jsonv2.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return Result{}, fmt.Errorf("gen: parsing sentences for %q: %w", word.Mot, err)
	}

	var out Result
	for _, s := range parsed.Phrases {
		if err := Filter(s, wl); err != nil {
			out.Rejected++
			continue
		}
		out.Accepted = append(out.Accepted, s)
	}
	return out, nil
}
