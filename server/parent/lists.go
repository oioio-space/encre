package parent

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
	"github.com/oioio-space/encre/server/store"
)

// The three list kinds a parent picks on the collage page (ENCRE_04 §7,
// encre-018), matching [store.Item.Kind]'s three [engine.Kind] values one
// for one. They are strings, not the numeric engine.Kind, because they come
// straight off an HTML <select> and a name reads better in a URL or a form
// than a bare digit does.
const (
	kindWord      = "mot"
	kindSentence  = "phrase"
	kindDictation = "dictee"
)

// Item.Source values this package writes. store.Item.Source has no type of
// its own — "its values are defined by the caller" — so this is the one
// caller that currently exists: every item this package builds went through
// [lexique.Lexicon.Analyze] or [lexique.Lexicon.AnalyzeSentence].
const sourceLexique = 1

// targetMark delimits a target word inside a kindSentence paste: the parent
// wraps the word or words the child must write in single asterisks, e.g.
// "Le *chat* dort sur le lit." Asterisks were picked over the "___" gap
// notation content/03 §9's AI-generated phrases use, because that notation
// already means something different here — a gap with nothing typed at all
// — while a parent typing a sentence writes the whole thing and only marks
// which word counts; asterisks are also the one bit of markup most parents
// already know from messaging apps, need no shift-heavy characters on a
// phone keyboard, and are a single rune to strip back out.
const targetMark = '*'

// buildItems parses rawText per kind and analyses every resulting item
// against lex, returning items ready for [store.Store.SaveItem] — each with
// a fresh ID, Source set to [sourceLexique], and Confirmed always false (an
// item straight off analysis has not been reviewed by anyone yet, however
// high its Confidence). It returns an error if kind is not one of
// [kindWord], [kindSentence] or [kindDictation], or if rawText holds
// nothing to analyse.
func buildItems(lex *lexique.Lexicon, listID, kind, rawText string) ([]*store.Item, error) {
	var drafts []textDraft
	switch kind {
	case kindWord:
		drafts = splitWords(rawText)
	case kindSentence:
		draft, err := splitSentenceTarget(rawText)
		if err != nil {
			return nil, err
		}
		drafts = []textDraft{draft}
	case kindDictation:
		drafts = splitDictation(rawText)
	default:
		return nil, fmt.Errorf("parent: unknown list kind %q", kind)
	}
	if len(drafts) == 0 {
		return nil, fmt.Errorf("parent: rien à analyser dans le texte collé")
	}

	items := make([]*store.Item, 0, len(drafts))
	for _, d := range drafts {
		id, err := newID()
		if err != nil {
			return nil, fmt.Errorf("generating item id: %w", err)
		}
		items = append(items, analyseItem(lex, listID, id, kind, d))
	}
	return items, nil
}

// textDraft is one item's text before analysis, with the target spans a
// kindSentence paste marked (empty for kindWord and kindDictation, where
// [lexique.Lexicon.AnalyzeSentence] is called with no targets — meaning the
// whole text is the target, which is exactly what a single word or a
// dictée's own chunk is).
type textDraft struct {
	text    string
	targets []lexique.Span
}

// splitWords turns a kindWord paste into one draft per word: split on
// commas, semicolons and newlines — the separators a parent's list is most
// likely to already use, pasted from anywhere from a school handout to a
// spreadsheet column — then on whitespace, so "chat, chien" and "chat\nchien"
// both yield two words. Blank entries are dropped.
func splitWords(rawText string) []textDraft {
	normalized := strings.NewReplacer(",", "\n", ";", "\n").Replace(rawText)
	var drafts []textDraft
	for line := range strings.Lines(normalized) {
		for word := range strings.FieldsSeq(line) {
			drafts = append(drafts, textDraft{text: word})
		}
	}
	return drafts
}

// splitDictation turns a kindDictation paste into one draft per sentence,
// cut at '.', '!', '?' or '…' — "découpée aux ponctuations", ENCRE_05's own
// words for this kind — with the punctuation kept on the sentence it closes,
// the way a dictée is normally shown to a child a clause at a time. Runs of
// punctuation (an ellipsis typed as "...", a "?!") are treated as one
// boundary, and leading whitespace on the next sentence is trimmed.
func splitDictation(rawText string) []textDraft {
	var (
		drafts  []textDraft
		current strings.Builder
	)
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			drafts = append(drafts, textDraft{text: text})
		}
		current.Reset()
	}
	inBoundary := false
	for _, r := range rawText {
		if isSentenceEnd(r) {
			current.WriteRune(r)
			inBoundary = true
			continue
		}
		if inBoundary {
			if unicode.IsSpace(r) {
				continue
			}
			flush()
			inBoundary = false
		}
		current.WriteRune(r)
	}
	flush()
	return drafts
}

// isSentenceEnd reports whether r is a rune [splitDictation] cuts a dictée
// on: the three terminal punctuation marks French school dictées use, plus
// the horizontal ellipsis as one rune (a run of three '.' already falls out
// of the loop above one rune at a time, which has the same effect).
func isSentenceEnd(r rune) bool {
	switch r {
	case '.', '!', '?', '…':
		return true
	default:
		return false
	}
}

// splitSentenceTarget strips [targetMark]-delimited target words out of a
// kindSentence paste, returning the plain text and the rune-offset
// [lexique.Span] of each target. It returns an error if rawText marks no
// target at all — a "phrase avec cibles" with no cible is not this kind's
// job; the parent wanted kindDictation or made a typo, either of which is
// better caught here than silently graded on nothing.
func splitSentenceTarget(rawText string) (textDraft, error) {
	trimmed := strings.TrimSpace(rawText)
	var (
		out     []rune
		targets []lexique.Span
		open    = -1
	)
	for _, r := range trimmed {
		switch {
		case r == targetMark && open < 0:
			open = len(out)
		case r == targetMark:
			targets = append(targets, lexique.Span{Start: open, End: len(out)})
			open = -1
		default:
			out = append(out, r)
		}
	}
	if len(targets) == 0 {
		return textDraft{}, fmt.Errorf("parent: aucune cible marquée d'astérisques dans la phrase")
	}
	return textDraft{text: string(out), targets: targets}, nil
}

// analyseItem runs lex over one draft and folds the result into a
// [store.Item]. A kindWord draft calls [lexique.Lexicon.Analyze] directly;
// kindSentence and kindDictation both go through
// [lexique.Lexicon.AnalyzeSentence] — the latter with no targets, so every
// word in the chunk is graded — and their possibly-many
// [lexique.Analysis] values are folded into one item by [foldAnalyses].
func analyseItem(lex *lexique.Lexicon, listID, id, kind string, d textDraft) *store.Item {
	item := &store.Item{ID: id, ListID: listID, Text: d.text, Targets: d.targets, Source: sourceLexique}

	switch kind {
	case kindWord:
		item.Kind = engine.KindWord
		foldAnalysis(item, lex.Analyze(d.text))
	case kindSentence:
		item.Kind = engine.KindSentence
		foldAnalyses(item, lex.AnalyzeSentence(d.text, d.targets))
	default: // kindDictation
		item.Kind = engine.KindDictation
		foldAnalyses(item, lex.AnalyzeSentence(d.text, nil))
	}
	return item
}

// foldAnalysis records a single [lexique.Analysis] — a kindWord item's only
// one — directly onto item.
func foldAnalysis(item *store.Item, a lexique.Analysis) {
	item.Colors = a.Traps
	item.Rules = a.Rules()
	item.Family = a.Family
	item.Confidence = a.Confidence
}

// foldAnalyses merges every word of a kindSentence or kindDictation item
// into one summary: Colors and Rules accumulate across every analysed word,
// Family keeps the first one offered (there is room for only one
// justification on the card), and Confidence takes the weakest word — one
// unsure word is enough to ask the parent to look at the whole item, the
// same reasoning [lexique.Analysis.Unsure] applies to a single word.
func foldAnalyses(item *store.Item, analyses []lexique.Analysis) {
	item.Colors = map[engine.Color]int{}
	item.Confidence = 1
	for _, a := range analyses {
		for color, n := range a.Traps {
			item.Colors[color] += n
		}
		item.Rules = append(item.Rules, a.Rules()...)
		if item.Family == "" {
			item.Family = a.Family
		}
		item.Confidence = min(item.Confidence, a.Confidence)
	}
	slices.Sort(item.Rules)
	item.Rules = slices.Compact(item.Rules)
}
