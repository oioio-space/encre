package api

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

// runManches mirrors the unexported constant of the same value inside
// engine/replay.go: a run always holds three manches, and this package needs
// the number too when it decides how many fresh words a week's deck offers.
const runManches = 3

// mediaPrefix is where server/store.Item.AudioPath resolves to, once served:
// brief/ENCRE_04 §8 has media/{listID}/{itemID}.ogg on disk, served under
// this prefix. This ticket only builds the URL; wiring a file server under
// it belongs to whichever ticket stands cmd/server up.
const mediaPrefix = "/media/"

// wordCatalog is what [Server.wordCatalog] gathers to build a run's deck:
// the words worth putting in front of the child this run, the full pool
// [engine.BuildDeck] draws the Garde and the recycled Old words from, and
// where to find each one's audio.
type wordCatalog struct {
	Week, Known []engine.Word
	Audio       map[string]string
}

// wordCatalog loads every enabled item of every validated word list
// belonging to childID and splits it into a wordCatalog.
//
// server/store has no single query for "every item across every list a
// child owns" — ENCRE_04 §6 normalizes items under word_lists, not
// directly under children, and nothing in this ticket needs a new list
// enumeration method added to add one confidently. This walks word_lists
// with the DB handle directly, the same escape hatch
// server/auth.LoginChild already uses to look a child up by a column with
// no dedicated Store method (pseudo, there; child_id here) — see that
// package for the precedent. Everything downstream of the list IDs still
// goes through [store.Store.ItemsOfList].
//
// Items with no recorded [engine.WordState] (never played) are ranked
// ahead of already-seen ones for the manche content: a week is supposed to
// bring something new, and the number of items validated by the parent will
// usually run past what fits in three manches long before it runs out.
func (s *Server) wordCatalog(ctx context.Context, childID string, states map[string]*engine.WordState) (wordCatalog, error) {
	listIDs, err := s.validatedListIDs(ctx, childID)
	if err != nil {
		return wordCatalog{}, err
	}

	var all []engine.Word
	audio := map[string]string{}
	for _, listID := range listIDs {
		items, err := s.db.ItemsOfList(ctx, listID)
		if err != nil {
			return wordCatalog{}, fmt.Errorf("loading items of list %s: %w", listID, err)
		}
		for _, item := range items {
			if !item.Enabled {
				continue
			}
			all = append(all, itemToWord(item))
			if item.AudioPath != "" {
				audio[item.ID] = mediaPrefix + item.AudioPath
			}
		}
	}
	// A stable, ID-sorted order rather than whatever order the lists and
	// their items happened to come back in: engine.BuildDeck's own shuffle
	// draws from this slice, and a response that reorders itself between
	// two otherwise-identical calls would make a test — and a support
	// ticket — impossible to pin down.
	slices.SortFunc(all, func(a, b engine.Word) int { return cmp.Compare(a.ID, b.ID) })

	var unseen, seen []engine.Word
	for _, w := range all {
		if st := states[w.ID]; st != nil && st.Seen {
			seen = append(seen, w)
			continue
		}
		unseen = append(unseen, w)
	}

	limit := s.cfg.WordsPerManche * runManches
	week := unseen
	if len(week) > limit {
		week = slices.Clone(week[:limit])
	}
	if need := limit - len(week); need > 0 {
		week = append(week, seen[:min(need, len(seen))]...)
	}

	return wordCatalog{Week: week, Known: all, Audio: audio}, nil
}

// validatedListIDs returns the IDs of every validated word list belonging to
// childID.
func (s *Server) validatedListIDs(ctx context.Context, childID string) ([]string, error) {
	rows, err := s.db.DB().QueryContext(ctx,
		`SELECT id FROM word_lists WHERE child_id = ? AND validated = 1`, childID)
	if err != nil {
		return nil, fmt.Errorf("querying validated lists: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning list id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating validated lists: %w", err)
	}
	return ids, nil
}

// itemToWord adapts a [store.Item] to the [engine.Word] BuildDeck and Replay
// need. Sentence carries the item's text a second time when Kind is
// KindSentence, matching engine.Word's own doc ("Sentence is the sentence
// with its gap, when Kind is KindSentence"); server/store.Item has one Text
// field for both a plain word and a sentence, so there is nothing else to
// tell them apart by.
func itemToWord(item *store.Item) engine.Word {
	w := engine.Word{
		ID:      item.ID,
		Text:    item.Text,
		Letters: utf8.RuneCountInString(item.Text),
		Traps:   item.Colors,
		Rules:   item.Rules,
		Family:  item.Family,
		Kind:    item.Kind,
	}
	if item.Kind == engine.KindSentence {
		w.Sentence = item.Text
	}
	return w
}

// filterGoldGarde keeps only the requested IDs whose recorded state is gold
// and not cursed: the Garde ENCRE_04 §4 offers is drawn from the child's
// gold words, and BuildDeck itself does not check that — it trusts whatever
// IDs it is handed against the known pool it is also handed. Silently
// dropping an ID that does not qualify, rather than rejecting the whole
// request, keeps a stale client (one that cached a word that has since lost
// its gold, or was never gold) from being unable to start a run at all.
func filterGoldGarde(ids []string, states map[string]*engine.WordState) []string {
	var out []string
	for _, id := range ids {
		if st := states[id]; st != nil && st.Gold && !st.Cursed {
			out = append(out, id)
		}
	}
	return out
}
