//go:build !js

package net

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// configDirName is the folder FileStore creates inside os.UserConfigDir()
// for the run state and outbox files (ENCRE_04 §2's native persistence).
const configDirName = "encre"

// FileStore is the native [Store]: one JSON file per key, inside
// os.UserConfigDir()/encre. [LocalStore] of store_js.go is its browser
// counterpart; callers on either side see only [Store], never FileStore or
// LocalStore, so the resume and outbox logic is the same code on both.
type FileStore struct {
	dir string
}

// NewFileStore returns a FileStore rooted at os.UserConfigDir()/encre,
// creating that directory if it does not already exist.
func NewFileStore() (*FileStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("net: locating the user config directory: %w", err)
	}
	dir := filepath.Join(base, configDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("net: creating %s: %w", dir, err)
	}
	return &FileStore{dir: dir}, nil
}

// path returns the file a key is saved under. Keys are the package's own
// constants (runStateKey, outboxKey), never untrusted input.
func (f *FileStore) path(key string) string {
	return filepath.Join(f.dir, key+".json")
}

// Load implements [Store].
func (f *FileStore) Load(key string) ([]byte, error) {
	// #nosec G304 -- key is one of this package's own constants, not
	// attacker-controlled input; path is rooted at f.dir, built above.
	data, err := os.ReadFile(f.path(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("net: reading %s: %w", key, err)
	}
	return data, nil
}

// Save implements [Store].
func (f *FileStore) Save(key string, data []byte) error {
	if err := os.WriteFile(f.path(key), data, 0o600); err != nil {
		return fmt.Errorf("net: writing %s: %w", key, err)
	}
	return nil
}

// Clear implements [Store].
func (f *FileStore) Clear(key string) error {
	if err := os.Remove(f.path(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("net: removing %s: %w", key, err)
	}
	return nil
}
