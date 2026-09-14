package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oioio-space/encre/engine"
)

// Run is one played run's record (ENCRE_04 §6). It is what the server keeps
// once a run is scored, separate from [engine.Run], the working struct
// [engine.Replay] takes: this row has no Attempts (kept in the attempts
// table) and no Revanche or Cahier (decided during Replay, not persisted).
type Run struct {
	ID      string
	ChildID string
	// StartedAt is when the run began; FinishedAt is the zero Time until the
	// run ends.
	StartedAt, FinishedAt time.Time
	// Rank is the child's rank at the time of the run, for a replay that must
	// not be rewritten by a later rank change.
	Rank      int
	Deck      engine.Deck
	Targets   [3]float64
	Talismans []engine.TalismanID
	Rooms     [2]engine.RoomID
	// FailedAt is the manche the run ended in, or -1 when it was won or is
	// still in progress.
	FailedAt int
	// Applied says whether [Store.MarkApplied] has already recorded this
	// run's effects on the child.
	Applied bool
}

// Attempt is one word attempted during a run (ENCRE_04 §6): the record
// [Store.SaveAttempts] writes once a run finishes, separate from
// [engine.Attempt] which Replay consumes to recompute the score.
type Attempt struct {
	RunID   string
	Idx     int
	ItemID  string
	Manche  int
	Blind   bool
	Copy    bool
	Correct bool
	Typed   string
	Millis  int
	// Chips and Mult are what this attempt scored, kept for the result card
	// and for audit — Replay recomputes them from scratch rather than
	// trusting these back.
	Chips, Mult float64
}

// CreateRun inserts r. It returns an error if r.ChildID names no child.
func (s *Store) CreateRun(ctx context.Context, r *Run) error {
	deck, talismans, rooms, err := marshalRunJSON(r)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO runs (
			id, child_id, started_at, finished_at, rank, deck_json,
			targets_json, talismans_json, rooms_json, failed_at, applied
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ChildID, r.StartedAt.Unix(), nullableUnixPtr(r.FinishedAt), r.Rank, deck,
		mustMarshal(r.Targets), talismans, rooms, nullableFailedAt(r.FailedAt), r.Applied)
	if err != nil {
		return fmt.Errorf("creating run: %w", err)
	}
	return nil
}

// RunByID returns the run with the given ID, or [ErrNotFound] if none exists.
func (s *Store) RunByID(ctx context.Context, id string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, child_id, started_at, finished_at, rank, deck_json,
			targets_json, talismans_json, rooms_json, failed_at, applied
		FROM runs WHERE id = ?`, id)

	var (
		r                               Run
		startedAt                       int64
		finishedAt                      sql.NullInt64
		failedAt                        sql.NullInt64
		deck, targets, talismans, rooms string
	)
	err := row.Scan(&r.ID, &r.ChildID, &startedAt, &finishedAt, &r.Rank, &deck,
		&targets, &talismans, &rooms, &failedAt, &r.Applied)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning run: %w", err)
	}
	r.StartedAt = time.Unix(startedAt, 0).UTC()
	if finishedAt.Valid {
		r.FinishedAt = time.Unix(finishedAt.Int64, 0).UTC()
	}
	r.FailedAt = -1
	if failedAt.Valid {
		r.FailedAt = int(failedAt.Int64)
	}
	if err := unmarshalRunJSON(&r, deck, targets, talismans, rooms); err != nil {
		return nil, err
	}
	return &r, nil
}

// MarkApplied records that run's effects have been written to its child,
// atomically: a single UPDATE conditioned on applied = 0 decides the
// outcome, so two callers racing to apply the same run cannot both succeed.
// It returns [ErrNotFound] if no run with that ID exists, or
// [engine.ErrAlreadyApplied] if it was already marked applied — the
// idempotence ENCRE_04 §4 requires so a client that resends a run after a
// lost network reply does not score it twice.
func (s *Store) MarkApplied(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE runs SET applied = 1 WHERE id = ? AND applied = 0`, id)
	if err != nil {
		return fmt.Errorf("marking run applied: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("marking run applied: checking rows affected: %w", err)
	}
	if n > 0 {
		return nil
	}

	// The UPDATE above already made the atomic decision (it changed nothing);
	// this SELECT only distinguishes why, for the caller's error message.
	var exists bool
	err = s.db.QueryRowContext(ctx, `SELECT true FROM runs WHERE id = ?`, id).Scan(&exists)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("marking run applied: checking existence: %w", err)
	default:
		return fmt.Errorf("run %s: %w", id, engine.ErrAlreadyApplied)
	}
}

// SaveAttempts inserts every attempt, or replaces it in place if its
// (RunID, Idx) already exists.
func (s *Store) SaveAttempts(ctx context.Context, attempts []*Attempt) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO attempts (run_id, idx, item_id, manche, blind, copy, correct, typed, millis, chips, mult)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, idx) DO UPDATE SET
			item_id = excluded.item_id, manche = excluded.manche, blind = excluded.blind,
			copy = excluded.copy, correct = excluded.correct, typed = excluded.typed,
			millis = excluded.millis, chips = excluded.chips, mult = excluded.mult`)
	if err != nil {
		return fmt.Errorf("preparing attempt insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, a := range attempts {
		_, err := stmt.ExecContext(ctx, a.RunID, a.Idx, a.ItemID, a.Manche, a.Blind, a.Copy,
			a.Correct, a.Typed, a.Millis, a.Chips, a.Mult)
		if err != nil {
			return fmt.Errorf("saving attempt %d of run %s: %w", a.Idx, a.RunID, err)
		}
	}
	return tx.Commit()
}

func marshalRunJSON(r *Run) (deck, talismans, rooms string, err error) {
	deckJSON, err := json.Marshal(r.Deck)
	if err != nil {
		return "", "", "", fmt.Errorf("marshaling deck_json: %w", err)
	}
	talismansJSON, err := json.Marshal(r.Talismans)
	if err != nil {
		return "", "", "", fmt.Errorf("marshaling talismans_json: %w", err)
	}
	roomsJSON, err := json.Marshal(r.Rooms)
	if err != nil {
		return "", "", "", fmt.Errorf("marshaling rooms_json: %w", err)
	}
	return string(deckJSON), string(talismansJSON), string(roomsJSON), nil
}

func unmarshalRunJSON(r *Run, deck, targets, talismans, rooms string) error {
	if err := json.Unmarshal([]byte(deck), &r.Deck); err != nil {
		return fmt.Errorf("unmarshaling deck_json: %w", err)
	}
	if err := json.Unmarshal([]byte(targets), &r.Targets); err != nil {
		return fmt.Errorf("unmarshaling targets_json: %w", err)
	}
	if err := json.Unmarshal([]byte(talismans), &r.Talismans); err != nil {
		return fmt.Errorf("unmarshaling talismans_json: %w", err)
	}
	if err := json.Unmarshal([]byte(rooms), &r.Rooms); err != nil {
		return fmt.Errorf("unmarshaling rooms_json: %w", err)
	}
	return nil
}

// mustMarshal marshals v, which must be a type that cannot fail to marshal
// (an array of floats here), and panics otherwise — a marshaling error on
// such a type would mean this package is broken, not that the caller sent
// bad data.
func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("store: marshaling %T: %v", v, err))
	}
	return string(b)
}

// nullableFailedAt returns nil for the "no failure" sentinel -1 (binding SQL
// NULL), or failedAt otherwise.
func nullableFailedAt(failedAt int) any {
	if failedAt < 0 {
		return nil
	}
	return failedAt
}
