package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DicteeResult is one item's outcome from a dictée the child took on paper,
// reported back by the parent (ENCRE_04 §7's "POST /lists/{id}/dictee-result").
// ENCRE_05's acceptance for encre-0qo is explicit that this may only ever
// raise a child's standing, never lower it — see whichever caller applies
// results against [engine.WordState] for where that rule is enforced;
// nothing here rejects a false Correct, since a dictée result is only ever a
// bonus signal on top of what the game itself already measured.
type DicteeResult struct {
	ID     string
	ListID string
	ItemID string
	// Correct says whether the child spelled this item right on paper.
	Correct   bool
	EnteredAt time.Time
}

// SaveDicteeResult inserts r, or replaces it in place if r.ID already
// exists. It returns an error if r.ItemID names no item.
func (s *Store) SaveDicteeResult(ctx context.Context, r *DicteeResult) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dictee_results (id, list_id, item_id, correct, entered_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			list_id = excluded.list_id, item_id = excluded.item_id,
			correct = excluded.correct, entered_at = excluded.entered_at`,
		r.ID, r.ListID, r.ItemID, r.Correct, r.EnteredAt.Unix())
	if err != nil {
		return fmt.Errorf("saving dictee result: %w", err)
	}
	return nil
}

// DicteeResultsOfList returns every dictée result recorded against listID,
// in no particular order.
func (s *Store) DicteeResultsOfList(ctx context.Context, listID string) ([]*DicteeResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, list_id, item_id, correct, entered_at
		FROM dictee_results WHERE list_id = ?`, listID)
	if err != nil {
		return nil, fmt.Errorf("querying dictee results: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*DicteeResult
	for rows.Next() {
		r, err := scanDicteeResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating dictee results: %w", err)
	}
	return results, nil
}

// dicteeResultScanner is what [sql.Row] and [sql.Rows] share of the Scan
// method, letting scanDicteeResult serve both a single-row and a multi-row
// query.
type dicteeResultScanner interface {
	Scan(dest ...any) error
}

func scanDicteeResult(row dicteeResultScanner) (*DicteeResult, error) {
	var (
		r         DicteeResult
		enteredAt int64
	)
	err := row.Scan(&r.ID, &r.ListID, &r.ItemID, &r.Correct, &enteredAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning dictee result: %w", err)
	}
	r.EnteredAt = time.Unix(enteredAt, 0).UTC()
	return &r, nil
}
