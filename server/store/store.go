// Package store persists ENCRE's server state in SQLite (ENCRE_04 §6).
//
// The driver is [modernc.org/sqlite], a pure-Go transpile of SQLite: this
// project forbids CGO, so no wrapper over the C library is an option. Every
// [Store] runs in WAL journal mode with foreign keys enforced, and applies
// its embedded migrations on [Open] or [OpenMemory] — callers never run SQL
// DDL themselves.
//
// A typical server opens one Store for its lifetime:
//
//	db, err := store.Open("file:/var/lib/encre/encre.db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer db.Close()
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is a handle on ENCRE's SQLite database. Its zero value is not usable;
// build one with [Open] or [OpenMemory]. A Store is safe for concurrent use
// by multiple goroutines, as [database/sql.DB] is.
type Store struct {
	db *sql.DB
}

// pragmas are appended to every DSN this package opens.
//
// They travel in the DSN rather than being run once through the pool, because
// PRAGMA is per-connection: executing them on the pool sets them on whichever
// connection happened to serve the call, and every connection opened later
// starts without them. A store that enforces foreign keys on its first
// connection and drops them on its second is worse than one that never
// enforced them, because the tests would still pass.
const pragmas = "_pragma=journal_mode(WAL)" +
	"&_pragma=foreign_keys(1)" +
	"&_pragma=busy_timeout(5000)"

// fileConns is how many connections a file-backed database may open at once.
//
// WAL exists so that readers do not queue behind one another; a pool of one
// would throw that away. Writers still serialise inside SQLite, and the busy
// timeout above is what makes them wait instead of failing.
const fileConns = 8

// Open opens (creating if needed) the SQLite database named by dsn, enables
// WAL journaling, foreign-key enforcement and a busy timeout on every
// connection, applies any migration not yet recorded in schema_migrations, and
// returns the ready Store. dsn is a modernc.org/sqlite data source name,
// typically "file:/path/to/db.sqlite".
func Open(dsn string) (*Store, error) {
	return open(dsn, fileConns)
}

// OpenMemory opens a private, in-memory database for tests: a fresh schema on
// every call, discarded when the Store is closed.
//
// It is limited to a single connection, and that is not a tuning choice: an
// in-memory database lives only as long as a connection to it is open, so a
// larger pool lets migrations and later queries race each other into creating
// separate empty databases.
func OpenMemory() (*Store, error) {
	return open("file::memory:?cache=shared", 1)
}

func open(dsn string, maxConns int) (*Store, error) {
	db, err := sql.Open("sqlite", withPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)

	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// withPragmas adds this package's pragmas to a data source name, keeping
// whatever query the caller already wrote.
func withPragmas(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + pragmas
}

// DB returns the underlying [database/sql.DB], for queries this package does
// not yet wrap and for tests that inspect pragmas directly.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close releases the database's connections. Close is idempotent.
func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate applies every embedded migration not yet recorded in
// schema_migrations, in filename order, each inside its own transaction.
// Calling Migrate again once every migration has run does nothing and
// returns nil, so [Open] and [OpenMemory] can call it unconditionally.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL DEFAULT (unixepoch())
		)`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		applied, err := s.migrationApplied(ctx, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := s.applyMigration(ctx, name); err != nil {
			return fmt.Errorf("applying migration %s: %w", name, err)
		}
	}
	return nil
}

// migrationNames lists the embedded migration files, sorted so that
// "0001_init.sql" runs before "0002_....sql".
func migrationNames() ([]string, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func (s *Store) migrationApplied(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM schema_migrations WHERE version = ?`, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking migration %s: %w", name, err)
	}
	return count > 0, nil
}

func (s *Store) applyMigration(ctx context.Context, name string) error {
	sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("reading migration file: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}
	return tx.Commit()
}
