package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Parent is one account in the parent panel (ENCRE_04 §6).
type Parent struct {
	// ID is the parent's opaque, caller-assigned identifier.
	ID string
	// Email is unique across all parents; login looks a parent up by it.
	Email string
	// PassHash is the argon2id hash of the parent's password.
	PassHash []byte
	// TOTPSecret is the parent's enrolled TOTP secret, nil before enrollment.
	TOTPSecret []byte
	CreatedAt  time.Time
}

// CreateParent inserts p. It returns an error if p.Email is already taken.
func (s *Store) CreateParent(ctx context.Context, p *Parent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO parents (id, email, pass_hash, totp_secret, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Email, p.PassHash, nullableBlob(p.TOTPSecret), p.CreatedAt.Unix())
	if err != nil {
		return fmt.Errorf("creating parent: %w", err)
	}
	return nil
}

// ParentByID returns the parent with the given ID, or [ErrNotFound] if none
// exists.
func (s *Store) ParentByID(ctx context.Context, id string) (*Parent, error) {
	return s.scanParent(s.db.QueryRowContext(ctx, `
		SELECT id, email, pass_hash, totp_secret, created_at
		FROM parents WHERE id = ?`, id))
}

// ParentByEmail returns the parent with the given email, or [ErrNotFound] if
// none exists.
func (s *Store) ParentByEmail(ctx context.Context, email string) (*Parent, error) {
	return s.scanParent(s.db.QueryRowContext(ctx, `
		SELECT id, email, pass_hash, totp_secret, created_at
		FROM parents WHERE email = ?`, email))
}

func (s *Store) scanParent(row *sql.Row) (*Parent, error) {
	var (
		p          Parent
		totpSecret []byte
		createdAt  int64
	)
	err := row.Scan(&p.ID, &p.Email, &p.PassHash, &totpSecret, &createdAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning parent: %w", err)
	}
	p.TOTPSecret = totpSecret
	p.CreatedAt = time.Unix(createdAt, 0).UTC()
	return &p, nil
}

// nullableBlob returns nil (which the driver stores as SQL NULL) for an empty
// blob, or b unchanged otherwise, so an absent secret round-trips as NULL
// rather than a zero-length BLOB.
func nullableBlob(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}
