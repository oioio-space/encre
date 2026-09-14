//go:build !js

package anim_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oioio-space/encre/client/anim"
)

func TestReloadParsesAnExistingJuiceJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "juice.json")
	if err := os.WriteFile(path, []byte(`{"hitstop_trap_ms": 999}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	j, err := anim.Reload(path)
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if got, want := j.HitstopTrap.Duration(), 999*time.Millisecond; got != want {
		t.Errorf("HitstopTrap = %v, want %v", got, want)
	}
}

func TestReloadReportsAMissingFile(t *testing.T) {
	if _, err := anim.Reload(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("Reload of a missing file = nil error, want one")
	}
}
