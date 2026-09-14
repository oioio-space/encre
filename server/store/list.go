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

// ValidateList marks the list as validated, unlocking it for play. It returns
// [ErrNotFound] if no list with that ID exists.
func (s *Store) ValidateList(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE word_lists SET validated = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("validating word list: %w", err)
	}
	return requireRowAffected(result, "validating word list")
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
			family, audio_path, source, confidence, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			list_id = excluded.list_id, kind = excluded.kind, text = excluded.text,
			targets_json = excluded.targets_json, colors_json = excluded.colors_json,
			rules_json = excluded.rules_json, family = excluded.family,
			audio_path = excluded.audio_path, source = excluded.source,
			confidence = excluded.confidence, enabled = excluded.enabled`,
		item.ID, item.ListID, item.Kind, item.Text, targets, colors, rules,
		item.Family, item.AudioPath, item.Source, item.Confidence, item.Enabled)
	if err != nil {
		return fmt.Errorf("saving item: %w", err)
	}
	return nil
}

// ItemsOfList returns every item belonging to the given list, in no
// particular order.
func (s *Store) ItemsOfList(ctx context.Context, listID string) ([]*Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, list_id, kind, text, targets_json, colors_json, rules_json,
			family, audio_path, source, confidence, enabled
		FROM items WHERE list_id = ?`, listID)
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

func scanItem(rows *sql.Rows) (*Item, error) {
	var (
		item                   Item
		targets, colors, rules string
	)
	err := rows.Scan(&item.ID, &item.ListID, &item.Kind, &item.Text, &targets, &colors, &rules,
		&item.Family, &item.AudioPath, &item.Source, &item.Confidence, &item.Enabled)
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
