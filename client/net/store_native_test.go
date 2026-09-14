//go:build !js

package net_test

import (
	"errors"
	"testing"

	"github.com/oioio-space/encre/client/net"
)

func TestFileStoreRoundTripsSaveAndLoad(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store, err := net.NewFileStore()
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	if err := store.Save("run", []byte(`{"hello":"world"}`)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load("run")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := `{"hello":"world"}`; string(got) != want {
		t.Errorf("Load() = %q, want %q", got, want)
	}
}

func TestFileStoreLoadOfAnUnsavedKeyReturnsErrNotFound(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store, err := net.NewFileStore()
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	if _, err := store.Load("absent"); !errors.Is(err, net.ErrNotFound) {
		t.Errorf("Load(%q) error = %v, want ErrNotFound", "absent", err)
	}
}

func TestFileStoreClearRemovesAndIsSafeToRepeat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store, err := net.NewFileStore()
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if err := store.Save("run", []byte("x")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Clear("run"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := store.Load("run"); !errors.Is(err, net.ErrNotFound) {
		t.Errorf("Load after Clear error = %v, want ErrNotFound", err)
	}
	if err := store.Clear("run"); err != nil {
		t.Errorf("Clear of an already-cleared key: %v, want nil", err)
	}
}
