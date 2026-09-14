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

// DeleteSessionsForSubject removes every session belonging to subjectID,
// parent or child, regardless of token. It is what encre-qpx.6 requires a
// parent's password change, TOTP re-enrollment or account deletion to call:
// none of those actions otherwise has any effect on a session an attacker
// already holds — sessions.subject_id carries no foreign key (it names a row
// in either parents or children depending on kind, so it cannot), so nothing
// cascades here without this call. It returns the number of sessions
// removed, mainly so a caller like account deletion can assert something was
// actually revoked in its own tests.
func (s *Store) DeleteSessionsForSubject(ctx context.Context, subjectID string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE subject_id = ?`, subjectID)
	if err != nil {
		return 0, fmt.Errorf("deleting sessions for subject: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("checking rows affected: %w", err)
	}
	return n, nil
}

// PurgeExpiredSessions deletes every session whose expiry is before now.
// Nothing in this codebase calls it on a schedule yet — see
// [github.com/oioio-space/encre/server/auth.PurgeExpiredTOTPUses]'s doc
// comment for the same caveat about its own table — so a deployment's
// startup or a periodic job must call it explicitly, or expired sessions
// (harmless — [LookupSession] already rejects them by [Session.ExpiresAt] —
// but unbounded) accumulate forever (encre-qpx.8, L6).
func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now.Unix())
	if err != nil {
		return fmt.Errorf("purging expired sessions: %w", err)
	}
	return nil
}
