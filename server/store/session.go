package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SessionKind tells which side of ENCRE_04 §7's cookie a session belongs to.
type SessionKind int

const (
	// SessionChild is a 24-hour child session, created by /child/login.
	SessionChild SessionKind = iota
	// SessionParent is a 7-day parent session, created by /parent/login.
	SessionParent
)

// Session is one active cookie session (ENCRE_04 §6, §7).
type Session struct {
	// Token is the opaque, caller-generated session cookie value.
	Token     string
	Kind      SessionKind
	SubjectID string
	ExpiresAt time.Time
	// TOTPOKUntil is how long a parent session may perform sensitive actions
	// without asking for a fresh TOTP code again, or the zero Time if no TOTP
	// check has passed yet.
	TOTPOKUntil time.Time
}

// CreateSession inserts sess.
func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (token, kind, subject_id, expires_at, totp_ok_until)
		VALUES (?, ?, ?, ?, ?)`,
		sess.Token, sess.Kind, sess.SubjectID, sess.ExpiresAt.Unix(), nullableUnixPtr(sess.TOTPOKUntil))
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

// Session returns the session for the given token, or [ErrNotFound] if none
// exists. It does not check expiry: callers compare ExpiresAt against the
// current time themselves.
func (s *Store) Session(ctx context.Context, token string) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT token, kind, subject_id, expires_at, totp_ok_until
		FROM sessions WHERE token = ?`, token)

	var (
		sess        Session
		expiresAt   int64
		totpOKUntil sql.NullInt64
	)
	err := row.Scan(&sess.Token, &sess.Kind, &sess.SubjectID, &expiresAt, &totpOKUntil)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning session: %w", err)
	}
	sess.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	if totpOKUntil.Valid {
		sess.TOTPOKUntil = time.Unix(totpOKUntil.Int64, 0).UTC()
	}
	return &sess, nil
}

// DeleteSession removes the session for the given token. Deleting a token
// that does not exist is not an error, so logout stays idempotent.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// PurgeExpiredSessions deletes every session whose expiry has passed.
func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("purging expired sessions: %w", err)
	}
	return nil
}
