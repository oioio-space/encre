package store

import "errors"

// ErrNotFound is returned when a lookup by ID or another unique key finds no
// row.
var ErrNotFound = errors.New("store: not found")
