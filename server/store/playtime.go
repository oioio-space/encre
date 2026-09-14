package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// AddPlayTime adds seconds to the child's play time for the given day
// (days since the epoch, matching [engine.WordState.LastPlayedW]'s unit of
// week but counted in days here), creating the row if it does not exist yet.
func (s *Store) AddPlayTime(ctx context.Context, childID string, day int32, seconds int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO play_time (child_id, day, seconds, bonus_seconds) VALUES (?, ?, ?, 0)
		ON CONFLICT(child_id, day) DO UPDATE SET seconds = seconds + excluded.seconds`,
		childID, day, seconds)
	if err != nil {
		return fmt.Errorf("adding play time: %w", err)
	}
	return nil
}

// PlayTime returns the seconds played and the bonus seconds granted for the
// given child and day. Both are zero if no row exists yet.
func (s *Store) PlayTime(ctx context.Context, childID string, day int32) (seconds, bonusSeconds int, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT seconds, bonus_seconds FROM play_time WHERE child_id = ? AND day = ?`,
		childID, day).Scan(&seconds, &bonusSeconds)
	switch {
	case err == nil:
		return seconds, bonusSeconds, nil
	case errors.Is(err, sql.ErrNoRows):
		return 0, 0, nil
	default:
		return 0, 0, fmt.Errorf("querying play time: %w", err)
	}
}
