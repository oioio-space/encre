//go:build !js

package anim

import (
	"fmt"
	"os"
)

// Reload reads path and parses it as a juice.json over [DefaultJuice] (see
// [LoadJuice]). It is native-only: the browser build has no filesystem to read
// from, so a WASM build calls the Reload of reload_js.go instead, which always
// fails — hot reload there is a matter for a future dev-server endpoint, not
// disk access that does not exist.
func Reload(path string) (Juice, error) {
	// #nosec G304 -- native-only (see the build tag); path is typed by the
	// developer at the keyboard, pressing F5 to reload their own juice.json,
	// not untrusted input reaching this binary over a network.
	raw, err := os.ReadFile(path)
	if err != nil {
		return Juice{}, fmt.Errorf("reloading %s: %w", path, err)
	}
	return LoadJuice(raw)
}
