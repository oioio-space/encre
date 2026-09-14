package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

// WordList is one parent-authored list of words a child studies (ENCRE_04
// §6): the raw text the parent pasted, analysed into [Item] rows.
type WordList struct {
	ID      string
	ChildID string
	Label   string
	// ShareCode lets a second parent or a co-teacher add to the list.
	ShareCode string
	// DueDate is when the list's dictée falls, or the zero Time if none is
	// set.
	DueDate time.Time
	// Validated says whether the parent has approved every item, unlocking
	// the list for play.
	Validated bool
	CreatedAt time.Time
}

// Item is one word, sentence or dictation to spell (ENCRE_04 §6), analysed
// from a WordList's raw text by the lexique package.
type Item struct {
	ID     string
	ListID string
	Kind   engine.Kind
	Text   string
	// Targets marks which spans of Text are graded, for a KindSentence or
	// KindDictation item where the rest of the text is given to the child.
	Targets []lexique.Span
	// Colors counts the traps per Couleur (lexique.Analysis.Traps).
	Colors map[engine.Color]int
	Rules  []engine.Rule
	Family string
	// AudioPath is where the parent's or Piper's recording lives, relative to
	// the media root, or "" before one is attached.
	AudioPath string
	// Source says how the item was produced: parent-typed, lexique-analysed,
	// or AI-generated. Its values are defined by the caller.
	Source int
	// Confidence is how sure the analysis is, 0 to 1 (lexique.Analysis).
	Confidence float64
	// Enabled says whether the item is currently in the deck it belongs to.
	Enabled bool
	// Confirmed says whether the parent has explicitly accepted this item's
	// analysis. It only matters when Confidence is below
	// [lexique.UnsureConfidence] ("à vérifier") — [Store.ValidateList]
	// refuses to validate a list carrying an unconfirmed unsure item
	// (ENCRE_05 backlog, encre-018).
	Confirmed bool
}

// CreateList inserts l. It returns an error if l.ChildID names no child.
func (s *Store) CreateList(ctx context.Context, l *WordList) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO word_lists (id, child_id, label, share_code, due_date, validated, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.ChildID, l.Label, l.ShareCode, nullableUnixPtr(l.DueDate), l.Validated, l.CreatedAt.Unix())
	if err != nil {
		return fmt.Errorf("creating word list: %w", err)
	}
	return nil
}

// ListByID returns the word list with the given ID, or [ErrNotFound] if none
// exists.
func (s *Store) ListByID(ctx context.Context, id string) (*WordList, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, child_id, label, share_code, due_date, validated, created_at
		FROM word_lists WHERE id = ?`, id)
	return scanWordListRow(row)
}

// DeleteList removes the list with the given ID. Its items, their sentences
// and word states cascade with it (the ON DELETE CASCADE foreign keys of
// migration 0001) — a single statement, not a manual walk down the tree.
//
// It does not touch anything under media/{listID}/ on disk: this package
// has no notion of the media root ([github.com/oioio-space/encre/server/media.Root]).
// A caller that also wants the list's recordings gone must call
// [github.com/oioio-space/encre/server/media.Root.RemoveList] itself,
// typically right after this succeeds (ENCRE_04 §8, "supprimé avec la
// liste").
func (s *Store) DeleteList(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM word_lists WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting word list: %w", err)
	}
	return requireRowAffected(result, "deleting word list")
}

// ValidateList marks the list as validated, unlocking it for play. It returns
// [ErrNotFound] if no list with that ID exists.
func (s *Store) ValidateList(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE word_lists SET validated = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("validating word list: %w", err)
	}
	return requireRowAffected(result, "validating word list")
}

// SetListDueDate sets the date a list's dictée falls on — the "date" step of
// ENCRE_04 §11's semaine flow — or clears it when due is the zero Time. It
// returns [ErrNotFound] if no list with that ID exists.
func (s *Store) SetListDueDate(ctx context.Context, id string, due time.Time) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE word_lists SET due_date = ? WHERE id = ?`, nullableUnixPtr(due), id)
	if err != nil {
		return fmt.Errorf("setting word list due date: %w", err)
	}
	return requireRowAffected(result, "setting word list due date")
}

// ListsOfChild returns every word list belonging to childID, most recently
// created first — the order the semaine page and the export both want a
// parent's lists shown in.
func (s *Store) ListsOfChild(ctx context.Context, childID string) ([]*WordList, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, child_id, label, share_code, due_date, validated, created_at
		FROM word_lists WHERE child_id = ? ORDER BY created_at DESC`, childID)
	if err != nil {
		return nil, fmt.Errorf("querying word lists: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var lists []*WordList
	for rows.Next() {
		l, err := scanWordListRow(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating word lists: %w", err)
	}
	return lists, nil
}

// wordListScanner is what [sql.Row] and [sql.Rows] share of the Scan method,
// letting scanWordListRow serve both [Store.ListByID] and [Store.ListsOfChild].
type wordListScanner interface {
	Scan(dest ...any) error
}

func scanWordListRow(row wordListScanner) (*WordList, error) {
	var (
		l         WordList
		dueDate   sql.NullInt64
		createdAt int64
	)
	err := row.Scan(&l.ID, &l.ChildID, &l.Label, &l.ShareCode, &dueDate, &l.Validated, &createdAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning word list: %w", err)
	}
	if dueDate.Valid {
		l.DueDate = time.Unix(dueDate.Int64, 0).UTC()
	}
	l.CreatedAt = time.Unix(createdAt, 0).UTC()
	return &l, nil
}

// SaveItem inserts item, or replaces it in place if item.ID already exists.
func (s *Store) SaveItem(ctx context.Context, item *Item) error {
	targets, err := json.Marshal(item.Targets)
	if err != nil {
		return fmt.Errorf("marshaling targets_json: %w", err)
	}
	colors, err := json.Marshal(item.Colors)
	if err != nil {
		return fmt.Errorf("marshaling colors_json: %w", err)
	}
	rules, err := json.Marshal(item.Rules)
	if err != nil {
		return fmt.Errorf("marshaling rules_json: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO items (
			id, list_id, kind, text, targets_json, colors_json, rules_json,
			family, audio_path, source, confidence, enabled, confirmed
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			list_id = excluded.list_id, kind = excluded.kind, text = excluded.text,
			targets_json = excluded.targets_json, colors_json = excluded.colors_json,
			rules_json = excluded.rules_json, family = excluded.family,
			audio_path = excluded.audio_path, source = excluded.source,
			confidence = excluded.confidence, enabled = excluded.enabled,
			confirmed = excluded.confirmed`,
		item.ID, item.ListID, item.Kind, item.Text, targets, colors, rules,
		item.Family, item.AudioPath, item.Source, item.Confidence, item.Enabled, item.Confirmed)
	if err != nil {
		return fmt.Errorf("saving item: %w", err)
	}
	return nil
}

// ItemByID returns the item with the given ID, or [ErrNotFound] if none
// exists.
func (s *Store) ItemByID(ctx context.Context, id string) (*Item, error) {
	row := s.db.QueryRowContext(ctx, itemSelect+`WHERE id = ?`, id)
	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

// ItemsOfList returns every item belonging to the given list, in no
// particular order.
func (s *Store) ItemsOfList(ctx context.Context, listID string) ([]*Item, error) {
	rows, err := s.db.QueryContext(ctx, itemSelect+`WHERE list_id = ?`, listID)
	if err != nil {
		return nil, fmt.Errorf("querying items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating items: %w", err)
	}
	return items, nil
}

const itemSelect = `
	SELECT id, list_id, kind, text, targets_json, colors_json, rules_json,
		family, audio_path, source, confidence, enabled, confirmed
	FROM items `

// itemScanner is what [sql.Row] and [sql.Rows] share of the Scan method,
// letting scanItem serve both [Store.ItemByID] and the multi-row queries.
type itemScanner interface {
	Scan(dest ...any) error
}

func scanItem(row itemScanner) (*Item, error) {
	var (
		item                   Item
		targets, colors, rules string
	)
	err := row.Scan(&item.ID, &item.ListID, &item.Kind, &item.Text, &targets, &colors, &rules,
		&item.Family, &item.AudioPath, &item.Source, &item.Confidence, &item.Enabled, &item.Confirmed)
	if err != nil {
		return nil, fmt.Errorf("scanning item: %w", err)
	}
	if err := json.Unmarshal([]byte(targets), &item.Targets); err != nil {
		return nil, fmt.Errorf("unmarshaling targets_json: %w", err)
	}
	if err := json.Unmarshal([]byte(colors), &item.Colors); err != nil {
		return nil, fmt.Errorf("unmarshaling colors_json: %w", err)
	}
	if err := json.Unmarshal([]byte(rules), &item.Rules); err != nil {
		return nil, fmt.Errorf("unmarshaling rules_json: %w", err)
	}
	return &item, nil
}

// nullableUnixPtr returns nil for the zero Time (binding SQL NULL), or its
// Unix seconds otherwise.
func nullableUnixPtr(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Unix()
}
