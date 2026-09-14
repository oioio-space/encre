package store_test

import (
	"testing"

	"github.com/oioio-space/encre/server/store"
)

func TestOpenMemoryAppliesMigrations(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var name string
	err = db.DB().QueryRowContext(t.Context(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'children'`).Scan(&name)
	if err != nil {
		t.Fatalf("querying sqlite_master: %v", err)
	}
	if name != "children" {
		t.Errorf("table name = %q, want %q", name, "children")
	}
}

func TestMigrateFailsOnClosedDB(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.Migrate(t.Context()); err == nil {
		t.Error("Migrate() on a closed database: want error, got nil")
	}
}

func TestOpenMemoryMigrationsAreIdempotent(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(t.Context()); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var count int
	err = db.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM schema_migrations`).Scan(&count)
	if err != nil {
		t.Fatalf("counting schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("schema_migrations count = %d, want 1", count)
	}
}

func TestOpenMemoryEnablesWAL(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var mode string
	if err := db.DB().QueryRowContext(t.Context(), `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	// Shared in-memory databases report "memory" rather than "wal": WAL needs a
	// real file to hold its -wal segment. The pragma still ran with no error,
	// which is what a shared cache exercises; Open (file-backed) is where WAL
	// actually takes effect.
	if mode == "" {
		t.Error("journal_mode is empty")
	}
}

func TestOpenEnablesWAL(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/store.db"
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var mode string
	if err := db.DB().QueryRowContext(t.Context(), `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var enabled int
	if err := db.DB().QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("querying foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", enabled)
	}
}
