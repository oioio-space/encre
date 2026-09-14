// Package net saves the run in progress and the outbox of unsent finish
// requests, and replays that outbox against the server — the offline and
// resume story of brief/ENCRE_04 §2 and §10.
//
// Two things sit behind the same [Store] interface, chosen by build tag: in
// the browser, [LocalStore] wraps window.localStorage through syscall/js; in
// a native build, [FileStore] writes a JSON file in os.UserConfigDir(). The
// interface exists so the save-every-word and resume-after-close logic can be
// tested against an in-memory fake, without a browser or a running server —
// see the package's own tests for that fake.
package net

import "errors"

// ErrNotFound is returned by [Store.Load] when nothing has been saved under
// key yet — an empty save slot, not a failure. A fresh install, or a child
// who has never lost a connection, sees this on every key.
var ErrNotFound = errors.New("net: no data saved for this key")

// Store is the persistence contract the rest of this package saves through.
// A key is a short name such as "run" or "outbox"; an implementation is free
// to turn that into a localStorage key or a file name however it likes, as
// long as Load after Save returns exactly what was saved.
type Store interface {
	// Load returns the bytes saved under key. It returns ErrNotFound, and no
	// other error, when key has never been saved or has been [Store.Clear]ed.
	Load(key string) ([]byte, error)
	// Save writes data under key, replacing whatever was saved there before.
	Save(key string, data []byte) error
	// Clear removes whatever is saved under key. Clearing a key that was
	// never saved is not an error.
	Clear(key string) error
}
