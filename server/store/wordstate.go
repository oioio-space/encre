package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oioio-space/encre/engine"
)

// WordStates returns every [engine.WordState] the given child has, keyed by
// item ID. A child with no recorded state returns an empty, non-nil map.
func (s *Store) WordStates(ctx context.Context, childID string) (map[string]*engine.WordState, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, state_json FROM word_states WHERE child_id = ?`, childID)
	if err != nil {
		return nil, fmt.Errorf("querying word states: %w", err)
	}
	defer func() { _ = rows.Close() }()

	states := map[string]*engine.WordState{}
	for rows.Next() {
		var (
			itemID string
			raw    string
		)
		if err := rows.Scan(&itemID, &raw); err != nil {
			return nil, fmt.Errorf("scanning word state: %w", err)
		}
		var st engine.WordState
		if err := json.Unmarshal([]byte(raw), &st); err != nil {
			return nil, fmt.Errorf("unmarshaling state_json for %s: %w", itemID, err)
		}
		states[itemID] = &st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating word states: %w", err)
	}
	return states, nil
}

// SaveWordStates writes every state in states for childID, inserting or
// overwriting one row per item ID.
func (s *Store) SaveWordStates(ctx context.Context, childID string, states map[string]*engine.WordState) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO word_states (child_id, item_id, state_json) VALUES (?, ?, ?)
		ON CONFLICT(child_id, item_id) DO UPDATE SET state_json = excluded.state_json`)
	if err != nil {
		return fmt.Errorf("preparing word state upsert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for itemID, st := range states {
		raw, err := json.Marshal(st)
		if err != nil {
			return fmt.Errorf("marshaling state_json for %s: %w", itemID, err)
		}
		if _, err := stmt.ExecContext(ctx, childID, itemID, raw); err != nil {
			return fmt.Errorf("saving word state for %s: %w", itemID, err)
		}
	}
	return tx.Commit()
}
