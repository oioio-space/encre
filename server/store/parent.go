package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Parent is one account in the parent panel (ENCRE_04 §6).
type Parent struct {
	// ID is the parent's opaque, caller-assigned identifier.
	ID string
	// Email is unique across all parents; login looks a parent up by it.
	Email string
	// PassHash is the peppered argon2id hash of the parent's password —
	// see [github.com/oioio-space/encre/server/auth.Pepper.PepperHash].
	// Never a bare argon2id hash: [Store] does not enforce that itself, but
	// every caller in this codebase goes through
	// [github.com/oioio-space/encre/server/auth.HashPassword], which does.
	PassHash []byte
	// TOTPSecret is the parent's enrolled TOTP secret, AES-256-GCM sealed
	// under [github.com/oioio-space/encre/server/auth.Pepper.Encrypt], nil
	// before enrollment. Never plaintext at rest: ENCRE_04 §12 replicates
	// this database continuously (Litestream), so a plaintext second factor
	// here would not be a second factor against a leaked replica at all.
	TOTPSecret []byte
	// FamilyCode scopes a child login to this parent's family
	// (encre-qpx.5): unguessable, generated once at parent creation. Empty
	// for a parent row created before migration 0005; such a parent's
	// children remain reachable only through the pre-scoping global lookup
	// until this parent is assigned one.
	FamilyCode string
	CreatedAt  time.Time
}

// LogValue implements [log/slog.LogValuer], redacting PassHash and
// TOTPSecret so a handler that logs a Parent — deliberately or by
// accident — never writes either into a log file, which sits outside
// every protection the peppering and encryption above add against a
// leaked database (encre-qpx.8).
func (p *Parent) LogValue() slog.Value {
	if p == nil {
		return slog.StringValue("<nil>")
	}
	return slog.GroupValue(
		slog.String("id", p.ID),
		slog.String("email", p.Email),
		slog.Bool("hasPassHash", len(p.PassHash) > 0),
		slog.Bool("hasTOTPSecret", len(p.TOTPSecret) > 0),
		slog.Time("createdAt", p.CreatedAt),
	)
}

// String implements [fmt.Stringer] with the same redaction as [Parent.LogValue],
// so a %v or %+v of a Parent in an ad hoc log line or error message is
// exactly as safe as a structured slog call.
func (p *Parent) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Parent{ID: %q, Email: %q, hasPassHash: %t, hasTOTPSecret: %t, CreatedAt: %s}",
		p.ID, p.Email, len(p.PassHash) > 0, len(p.TOTPSecret) > 0, p.CreatedAt)
}

// CreateParent inserts p. It returns an error if p.Email or p.FamilyCode is
// already taken.
func (s *Store) CreateParent(ctx context.Context, p *Parent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO parents (id, email, pass_hash, totp_secret, family_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Email, p.PassHash, nullableBlob(p.TOTPSecret), nullableString(p.FamilyCode), p.CreatedAt.Unix())
	if err != nil {
		return fmt.Errorf("creating parent: %w", err)
	}
	return nil
}

// ParentByID returns the parent with the given ID, or [ErrNotFound] if none
// exists.
func (s *Store) ParentByID(ctx context.Context, id string) (*Parent, error) {
	return s.scanParent(s.db.QueryRowContext(ctx, parentSelect+`WHERE id = ?`, id))
}

// ParentByEmail returns the parent with the given email, or [ErrNotFound] if
// none exists.
func (s *Store) ParentByEmail(ctx context.Context, email string) (*Parent, error) {
	return s.scanParent(s.db.QueryRowContext(ctx, parentSelect+`WHERE email = ?`, email))
}

// ParentByFamilyCode returns the parent with the given family code, or
// [ErrNotFound] if none exists. It is what a family-scoped child login page
// (encre-qpx.5) resolves its URL or cookie's code against, before listing
// [Store.ChildrenOfParent] for that parent alone.
func (s *Store) ParentByFamilyCode(ctx context.Context, code string) (*Parent, error) {
	return s.scanParent(s.db.QueryRowContext(ctx, parentSelect+`WHERE family_code = ?`, code))
}

// SetParentTOTPSecret overwrites p's stored TOTP secret (already encrypted
// by the caller — see [Parent.TOTPSecret]'s doc comment) and returns
// [ErrNotFound] if no parent with that ID exists.
func (s *Store) SetParentTOTPSecret(ctx context.Context, parentID string, secret []byte) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE parents SET totp_secret = ? WHERE id = ?`, nullableBlob(secret), parentID)
	if err != nil {
		return fmt.Errorf("setting parent totp secret: %w", err)
	}
	return requireRowAffected(result, "setting parent totp secret")
}

// SetParentPassHash overwrites p's stored password hash and returns
// [ErrNotFound] if no parent with that ID exists. Callers that change a
// parent's password must follow this with
// [Store.DeleteSessionsForSubject](parentID) — a changed password does not,
// by itself, revoke a session an attacker who had the old one already holds
// (encre-qpx.6).
func (s *Store) SetParentPassHash(ctx context.Context, parentID string, passHash []byte) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE parents SET pass_hash = ? WHERE id = ?`, passHash, parentID)
	if err != nil {
		return fmt.Errorf("setting parent pass hash: %w", err)
	}
	return requireRowAffected(result, "setting parent pass hash")
}

// DeleteParent removes the parent row for id and, through this database's
// foreign-key cascades (see [Open]'s doc comment — foreign_keys is on for
// every connection), every child, word list, item, run and session that
// hangs off it. It is what account deletion (ENCRE_04 §7's "export et
// suppression complets") calls after the caller has already exported
// whatever the parent asked to keep; deleting is not an error when id does
// not exist, so a retried request stays idempotent.
func (s *Store) DeleteParent(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM parents WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting parent: %w", err)
	}
	return nil
}

const parentSelect = `SELECT id, email, pass_hash, totp_secret, family_code, created_at FROM parents `

func (s *Store) scanParent(row *sql.Row) (*Parent, error) {
	var (
		p          Parent
		totpSecret []byte
		familyCode sql.NullString
		createdAt  int64
	)
	err := row.Scan(&p.ID, &p.Email, &p.PassHash, &totpSecret, &familyCode, &createdAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("scanning parent: %w", err)
	}
	p.TOTPSecret = totpSecret
	p.FamilyCode = familyCode.String
	p.CreatedAt = time.Unix(createdAt, 0).UTC()
	return &p, nil
}

// nullableString returns nil (which the driver stores as SQL NULL) for an
// empty string, or s unchanged otherwise.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
