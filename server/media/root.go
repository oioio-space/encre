package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// dirPerm and filePerm are the permissions [Root] creates directories and
// writes files with: owner read/write/execute (or read/write for files),
// nothing for group or other — this is one child's family's recordings,
// never meant to be world-readable on the host.
const (
	dirPerm  = 0o700
	filePerm = 0o600
)

// ErrInvalidID is returned by every [Root] method for a listID or itemID
// that fails [ValidID].
var ErrInvalidID = errors.New("media: invalid id")

// Root is the media directory ENCRE_04 §8 lays out as media/{listID}/{itemID}.ogg.
//
// Its zero value is not usable; build one with [NewRoot].
type Root struct {
	dir string
}

// NewRoot builds a [Root] rooted at dir, creating dir if it does not exist.
func NewRoot(dir string) (Root, error) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return Root{}, fmt.Errorf("media: creating root %s: %w", dir, err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Root{}, fmt.Errorf("media: resolving root %s: %w", dir, err)
	}
	return Root{dir: abs}, nil
}

// Dir returns the root directory, for a static file server to serve from.
func (r Root) Dir() string { return r.dir }

// Path returns the absolute path media/{listID}/{itemID}.ogg is stored at,
// creating the list's directory if it does not exist yet. It returns
// [ErrInvalidID] — never a path — if either ID fails [ValidID]; this is the
// only function in this package that turns an ID into a filesystem path,
// and it never sees a value that did not already pass that check.
func (r Root) Path(listID, itemID string) (string, error) {
	if !ValidID(listID) || !ValidID(itemID) {
		return "", ErrInvalidID
	}
	listDir := filepath.Join(r.dir, listID)
	if err := os.MkdirAll(listDir, dirPerm); err != nil {
		return "", fmt.Errorf("media: creating list dir: %w", err)
	}
	return filepath.Join(listDir, itemID+".ogg"), nil
}

// RelPath returns the path [Path] would write to, relative to r's root —
// what [github.com/oioio-space/encre/server/store.Item.AudioPath] and
// [github.com/oioio-space/encre/server/store.Sentence.AudioPath] store, and
// what the static file server under /media/ resolves against.
func (r Root) RelPath(listID, itemID string) (string, error) {
	if !ValidID(listID) || !ValidID(itemID) {
		return "", ErrInvalidID
	}
	return filepath.Join(listID, itemID+".ogg"), nil
}

// RemoveList deletes every recording under listID — the "supprimé avec la
// liste" ENCRE_04 §8 asks for. Removing a list that has no media directory
// is not an error.
func (r Root) RemoveList(listID string) error {
	if !ValidID(listID) {
		return ErrInvalidID
	}
	if err := os.RemoveAll(filepath.Join(r.dir, listID)); err != nil {
		return fmt.Errorf("media: removing list %s: %w", listID, err)
	}
	return nil
}
