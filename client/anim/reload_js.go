//go:build js

package anim

import "fmt"

// Reload always fails in the browser build: WASM has no filesystem, so there
// is no path on disk to read juice.json from. It exists with the same
// signature as the native Reload of reload.go so a caller can invoke it
// unconditionally on either build; only the native one ever succeeds.
func Reload(path string) (Juice, error) {
	return Juice{}, fmt.Errorf("anim: hot reload of %s is not available on wasm (no filesystem)", path)
}
