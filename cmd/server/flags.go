package main

import "flag"

// serverFlags is the parsed form of a [flag.FlagSet] before its values are
// dereferenced into a [serverConfig] — kept separate from serverConfig
// itself because a [flag.FlagSet]'s Var methods want pointers, and a
// pointer-typed serverConfig would let a handler accidentally mutate the
// config it was built from.
type serverFlags struct {
	fs      *flag.FlagSet
	addr    *string
	dsn     *string
	webRoot *string
}

// flagSetWithDefaults declares every flag [run] accepts, on a fresh
// [flag.FlagSet] rather than [flag.CommandLine] — so parsing twice (every
// test that calls [parseFlags] more than once) never panics on a redefined
// flag. Defaults match a bare `go run ./cmd/server` against a build produced
// by `mise run wasm:build`; production overrides every one of them from
// deploy/encre.service.
func flagSetWithDefaults() *serverFlags {
	fs := flag.NewFlagSet("encre-server", flag.ContinueOnError)
	return &serverFlags{
		fs: fs,
		addr: fs.String("addr", "127.0.0.1:8081",
			"address to listen on (plaintext; Caddy terminates TLS in front, deploy/Caddyfile)"),
		dsn: fs.String("db", "file:encre.db",
			"modernc.org/sqlite data source name for the database (server/store.Open)"),
		webRoot: fs.String("web", "dist/web",
			"directory `mise run wasm:build` writes: index.html and sw.js at its root, "+
				"content-hashed assets under web/static (served at /static)"),
	}
}
