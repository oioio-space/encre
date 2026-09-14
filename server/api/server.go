package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// Server routes and serves ENCRE_04 §7's child-facing /api/v1 endpoints.
//
// Its zero value is not usable; build one with [New]. A Server is safe for
// concurrent use, as the [store.Store] it wraps is: nothing here mutates cfg
// or now after New returns.
type Server struct {
	db  *store.Store
	cfg engine.Config
	pep *auth.Pepper
	// now stands in for time.Now everywhere a handler needs the current
	// time — session expiry, today's play time, the deck's week — so tests
	// can move it without sleeping.
	now func() time.Time
	log *slog.Logger

	// patternLimiter enforces ENCRE_04 §7's 10-patterns-per-minute-per-child
	// limit, keyed by child ID (encre-qpx.5) — possible here for the first
	// time because [handleChildLogin] knows the child ID before checking
	// the pattern, not only after a successful one.
	patternLimiter *auth.Limiter
}

// New builds a Server that scores runs under cfg and persists to db. now is
// called for every "current time" a handler needs; pass [time.Now] in
// production and a fixed or moving stand-in in tests. pep must not be nil —
// see [auth.ErrPepperRequired] — it is this server's only path to the
// out-of-database key [auth.HashPattern] and [auth.VerifyParentTOTP] (via
// server/parent, not this package directly) need.
func New(db *store.Store, cfg engine.Config, now func() time.Time, pep *auth.Pepper) *Server {
	return &Server{
		db: db, cfg: cfg, now: now, pep: pep, log: slog.Default(),
		patternLimiter: auth.NewLimiter(auth.PatternRateLimit, now),
	}
}

// Handler returns the [http.Handler] that serves every route this package
// exposes under /api/v1.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/family/{code}/children", s.handleFamilyChildren)
	mux.HandleFunc("POST /api/v1/child/login", s.handleChildLogin)
	mux.HandleFunc("GET /api/v1/child/me", s.handleChildMe)
	mux.HandleFunc("POST /api/v1/run/start", s.handleRunStart)
	mux.HandleFunc("POST /api/v1/run/{id}/room", s.handleRunRoom)
	mux.HandleFunc("POST /api/v1/run/{id}/finish", s.handleRunFinish)
	mux.HandleFunc("POST /api/v1/run/{id}/heartbeat", s.handleRunHeartbeat)
	return mux
}
