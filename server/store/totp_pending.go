package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TOTPPending is one parent's in-progress TOTP enrollment (encre-qpx.7,
// migration 0004): the secret [server/auth.BeginTOTPEnrollment] generated,
// held server-side between rendering the QR code and the parent proving
// they scanned it, so the secret never has to round-trip through the
// client's browser as a hidden form field or cookie.
type TOTPPending struct {
	ParentID string
	// Secret is the AES-256-GCM-sealed TOTP secret — see
	// [Parent.TOTPSecret]'s doc comment for why this is never plaintext at
	// rest, which applies here exactly as much as it does once enrollment
	// completes.
	Secret    []byte
	ExpiresAt time.Time
}

// PutTOTPPending inserts or replaces the pending enrollment for
// p.ParentID: a parent who reloads the enrollment page and gets a fresh
// secret must not leave the previous one live in the table.
func (s *Store) PutTOTPPending(ctx context.Context, p *TOTPPending) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO totp_pending (parent_id, secret, expires_at) VALUES (?, ?, ?)
		ON CONFLICT (parent_id) DO UPDATE SET secret = excluded.secret, expires_at = excluded.expires_at`,
		p.ParentID, p.Secret, p.ExpiresAt.Unix())
	if err != nil {
		return fmt.Errorf("putting totp pending: %w", err)
	}
	return nil
}

// TOTPPendingByParentID returns parentID's pending enrollment, or
// [ErrNotFound] if none exists. It does not check expiry — callers compare
// ExpiresAt against the current time themselves, exactly as [Session] does.
func (s *Store) TOTPPendingByParentID(ctx context.Context, parentID string) (*TOTPPending, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT parent_id, secret, expires_at FROM totp_pending WHERE parent_id = ?`, parentID)

	var (
		p         TOTPPending
		expiresAt int64
	)
	err := row.Scan(&p.ParentID, &p.Secret, &expiresAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning totp pending: %w", err)
	}
	p.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	return &p, nil
}

// DeleteTOTPPending removes parentID's pending enrollment, whether it
// completed, was abandoned, or expired. Deleting one that does not exist is
// not an error.
func (s *Store) DeleteTOTPPending(ctx context.Context, parentID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM totp_pending WHERE parent_id = ?`, parentID); err != nil {
		return fmt.Errorf("deleting totp pending: %w", err)
	}
	return nil
}

// PurgeExpiredTOTPPending deletes every pending enrollment whose expiry is
// before now, the same "nothing calls this on a schedule yet, a deployment
// must" caveat as [Store.PurgeExpiredSessions] applies to.
func (s *Store) PurgeExpiredTOTPPending(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM totp_pending WHERE expires_at < ?`, now.Unix())
	if err != nil {
		return fmt.Errorf("purging expired totp pending: %w", err)
	}
	return nil
}
