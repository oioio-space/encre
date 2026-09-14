//go:build js

package net

import (
	"errors"
	"fmt"
	"syscall/js"
)

// storagePrefix keys LocalStore's saves inside localStorage, so nothing else
// sharing the browser origin can collide with them.
const storagePrefix = "encre."

// ErrStorageFull is returned by [LocalStore.Save] when the browser's quota
// for localStorage (a few MB, ENCRE_04 §2) is exhausted. The caller must not
// treat this as "saved" — see LocalStore's doc comment for what it does
// instead, so a full quota does not lose the child's run in silence.
var ErrStorageFull = errors.New("net: localStorage is full")

// LocalStore is the browser [Store]: syscall/js over window.localStorage.
// [FileStore] of store_native.go is its native counterpart.
//
// localStorage.setItem throws when the quota is exhausted, which syscall/js
// turns into a Go panic; Save recovers it and returns [ErrStorageFull]
// instead. It also keeps the last value passed to Save in memory (mem), so a
// save that failed to persist is still readable for the rest of this
// session — the run is not lost in silence, only its durability across a
// reload is, which is the best a full quota allows.
type LocalStore struct {
	mem map[string][]byte
}

// NewLocalStore returns a LocalStore backed by window.localStorage.
func NewLocalStore() *LocalStore {
	return &LocalStore{mem: map[string][]byte{}}
}

func storageKey(key string) string { return storagePrefix + key }

// Load implements [Store].
func (l *LocalStore) Load(key string) ([]byte, error) {
	item := js.Global().Get("localStorage").Call("getItem", storageKey(key))
	if item.IsNull() || item.IsUndefined() {
		if data, ok := l.mem[key]; ok {
			return data, nil
		}
		return nil, ErrNotFound
	}
	return []byte(item.String()), nil
}

// Save implements [Store]. See the type doc comment for what happens when
// localStorage's quota is full.
func (l *LocalStore) Save(key string, data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			l.mem[key] = data
			err = fmt.Errorf("%w: %v", ErrStorageFull, r)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", storageKey(key), string(data))
	l.mem[key] = data
	return nil
}

// Clear implements [Store].
func (l *LocalStore) Clear(key string) error {
	delete(l.mem, key)
	js.Global().Get("localStorage").Call("removeItem", storageKey(key))
	return nil
}
