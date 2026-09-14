package store_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/oioio-space/encre/server/store"
)

// TestEveryConnectionCarriesThePragmas is the trap behind opening the pool up.
//
// PRAGMA foreign_keys and PRAGMA busy_timeout are per-connection, not
// per-database: running them once through database/sql sets them on whichever
// pooled connection happened to serve the call, and every later connection
// starts without them. A store that enforces foreign keys on its first
// connection and silently drops them on its second is worse than one that
// never enforced them, because the tests would pass.
func TestEveryConnectionCarriesThePragmas(t *testing.T) {
	t.Parallel()

	s, err := store.Open("file:" + filepath.Join(t.TempDir(), "encre.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Hold several connections open at once so the pool is forced to build new
	// ones, then ask each of them what it believes.
	const conns = 4
	var wg sync.WaitGroup
	ready, release := make(chan struct{}, conns), make(chan struct{})
	for range conns {
		wg.Go(func() {
			c, err := s.DB().Conn(t.Context())
			if err != nil {
				t.Errorf("Conn() error = %v", err)
				return
			}
			defer func() { _ = c.Close() }()

			var fk int
			if err := c.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&fk); err != nil {
				t.Errorf("PRAGMA foreign_keys error = %v", err)
				return
			}
			if fk != 1 {
				t.Errorf("PRAGMA foreign_keys = %d on a pooled connection, want 1", fk)
			}
			var busy int
			if err := c.QueryRowContext(t.Context(), `PRAGMA busy_timeout`).Scan(&busy); err != nil {
				t.Errorf("PRAGMA busy_timeout error = %v", err)
				return
			}
			if busy == 0 {
				t.Error("PRAGMA busy_timeout = 0 on a pooled connection, want the configured wait")
			}
			ready <- struct{}{}
			<-release
		})
	}
	for range conns {
		<-ready
	}
	close(release)
	wg.Wait()
}

// TestTheFilePoolLetsReadersWorkTogether checks the change itself: WAL exists so
// that readers do not queue behind one another, and a pool of one throws that
// away.
func TestTheFilePoolLetsReadersWorkTogether(t *testing.T) {
	t.Parallel()

	s, err := store.Open("file:" + filepath.Join(t.TempDir(), "encre.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if got := s.DB().Stats().MaxOpenConnections; got == 1 {
		t.Error("MaxOpenConnections = 1 on a file database, want room for concurrent readers")
	}
}
