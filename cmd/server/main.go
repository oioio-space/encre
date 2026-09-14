// Command server is ENCRE's single production binary (brief/ENCRE_04 §1 and
// §12, ticket T31): it serves the child- and parent-facing JSON API under
// /api/v1 (server/api), the built WASM client and its assets under /static,
// the loading page and service worker at the root, and a parent-only
// operations dashboard at /admin/metrics.
//
// It expects Caddy in front of it for TLS, compression and the immutable
// cache header on /static (deploy/Caddyfile) — this binary itself only
// listens in plaintext, on loopback, because that split is what lets
// deploy/Caddyfile's trusted_proxies mirror server/auth.ClientIP's
// trustedProxies exactly (see deploy/README.md's "Trusted proxies" section):
// every request this binary sees either came through Caddy on 127.0.0.1, or
// it did not come from a network path the deployment considers a proxy at
// all.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/internal/httpstatic"
	"github.com/oioio-space/encre/server/api"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/gen"
	"github.com/oioio-space/encre/server/media"
	"github.com/oioio-space/encre/server/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("encre-server: fatal", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}

	// ENCRE_04 §12: "Journal slog JSON". JSON from the first line, not just
	// once something goes wrong, so the family server's log is grep-able
	// (`journalctl -u encre --output cat | jq`) from day one.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	pep, err := auth.LoadPepper()
	if err != nil {
		return fmt.Errorf("loading pepper key (see server/auth.LoadPepper, deploy/README.md): %w", err)
	}

	db, err := store.Open(cfg.dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close() //nolint:errcheck // best-effort on shutdown, already logging the real error if any

	mediaRoot, err := media.NewRoot(cfg.mediaRoot)
	if err != nil {
		return fmt.Errorf("preparing media root: %w", err)
	}
	if _, err := media.LookupFFmpeg(); err != nil {
		// ENCRE_04 §8's own instruction: say so plainly, do not refuse to
		// start. Every upload will answer 503 until ffmpeg is installed.
		logger.Warn("voice upload transcoding disabled: ffmpeg not found on PATH")
	}

	apiServer := api.New(db, engine.DefaultConfig(), time.Now, pep)
	apiServer.SetMediaRoot(mediaRoot)
	if genClient, err := gen.NewAnthropicClient(); err != nil {
		// ENCRE_04 §9's own fallback applies here too: no key in the
		// environment does not stop the server from starting, it only
		// means every parent hits [gen.ErrSentenceTooLong]'s sibling —
		// server/api's 503 — and types sentences by hand until one is set.
		logger.Warn("sentence generation disabled: no Anthropic API key", "env", gen.AnthropicAPIKeyEnv, "error", err)
	} else {
		apiServer.SetGenClient(genClient)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", apiServer.Handler())
	mux.Handle("GET /admin/metrics", adminMetricsHandler(db, logger))
	staticDir := cfg.webRoot + "/static"
	mux.Handle("/static/", http.StripPrefix("/static/",
		httpstatic.Precompressed(http.Dir(staticDir), http.FileServer(http.Dir(staticDir)))))
	mux.Handle("/media/", http.StripPrefix("/media/", cachedFileServer(mediaRoot.Dir())))
	mux.HandleFunc("GET /sw.js", serveNoCache(cfg.webRoot, "sw.js"))
	mux.HandleFunc("GET /{$}", serveNoCache(cfg.webRoot, "index.html"))

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Generous: the client fetches its own WASM through /static, which
		// this same server may be asked to stream on a slow link.
		WriteTimeout: 5 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, func() {
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	})

	logger.Info("encre-server listening", "addr", cfg.addr, "web", cfg.webRoot, "db", cfg.dsn)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving: %w", err)
	}
	return nil
}

// serverConfig is every flag [run] needs, split out so [parseFlags] has
// something to build and tests can construct one without touching os.Args.
type serverConfig struct {
	addr      string
	dsn       string
	webRoot   string
	mediaRoot string
}

func parseFlags(args []string) (serverConfig, error) {
	fs := flagSetWithDefaults()
	if err := fs.fs.Parse(args); err != nil {
		return serverConfig{}, err
	}
	return serverConfig{addr: *fs.addr, dsn: *fs.dsn, webRoot: *fs.webRoot, mediaRoot: *fs.mediaRoot}, nil
}

// serveNoCache serves exactly one file from dir, with a Cache-Control that
// forbids caching it.
//
// index.html and sw.js are the two files a returning player must always get
// fresh: index.html because it names the current content-hashed /static
// asset (deploy's cache immutability only holds because the *name* changes
// on every release, and that only works if the page pointing at the name is
// never served stale), and sw.js because a stale service worker can pin a
// client to a broken app version until its own next poll — browsers already
// re-check it at most once a day, an HTTP cache must not extend that.
func serveNoCache(dir, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, dir+"/"+name)
	}
}

// mediaCacheControl is the header [cachedFileServer] sets on every
// recording: a voice recording never changes once transcoded — a new
// upload gets a new path, since [media.Root.Path] is keyed by the item or
// sentence ID, not a version — so a long, immutable cache is safe, and
// this is one child's family's data, never meant to sit in a shared CDN
// cache the way /static's public assets do.
const mediaCacheControl = "private, max-age=31536000, immutable"

// cachedFileServer serves dir (server/media.Root.Dir()) with
// [mediaCacheControl] on every response. [net/http.FileServer] itself
// already refuses ".." path segments and anything resolving outside dir,
// the same protection [media.Root.Path] gives every path this package
// writes in the first place.
func cachedFileServer(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", mediaCacheControl)
		fs.ServeHTTP(w, r)
	})
}
