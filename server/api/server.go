package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/gen"
	"github.com/oioio-space/encre/server/media"
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

	// genClient calls the Anthropic API for [handleGenerateSentences]
	// (ENCRE_04 §9). It is nil until [Server.SetGenClient] is called —
	// typically because [gen.NewAnthropicClient] found no API key in the
	// environment — and a nil genClient is not an error condition this
	// package panics or refuses to start over: [handleGenerateSentences]
	// answers 503 and the parent falls back to typing the sentence by
	// hand, exactly the path ENCRE_03 §9 asks for when the API is simply
	// unavailable.
	genClient gen.Client
	// whitelist is the CE1 whitelist [handleGenerateSentences] filters
	// generated sentences against. It defaults to [gen.EmbeddedWhitelist]
	// and is only ever overridden by a test.
	whitelist gen.Whitelist

	// mediaRoot is where [handleUploadItemAudio] and
	// [handleUploadSentenceAudio] write transcoded recordings (ENCRE_04
	// §8). It is the zero [media.Root] until [Server.SetMediaRoot] is
	// called; both upload handlers answer 503 while it is unset, the same
	// "not configured, not broken" contract [genClient] holds.
	mediaRoot    media.Root
	mediaRootSet bool
}

// SetMediaRoot sets the directory [handleUploadItemAudio] and
// [handleUploadSentenceAudio] write transcoded recordings under (ENCRE_04
// §8). Build root with [media.NewRoot].
func (s *Server) SetMediaRoot(root media.Root) { s.mediaRoot, s.mediaRootSet = root, true }

// SetGenClient sets the client [handleGenerateSentences] and
// [handleRegenerateSentence] call to generate sentences (ENCRE_04 §9). It is
// safe to call with a nil client — the zero value already is nil — to
// explicitly disable generation.
func (s *Server) SetGenClient(c gen.Client) { s.genClient = c }

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
		whitelist:      gen.EmbeddedWhitelist(),
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
	mux.HandleFunc("GET /api/v1/child/bestiary", s.handleChildBestiary)
	mux.HandleFunc("GET /api/v1/child/rules", s.handleChildRules)
	mux.HandleFunc("GET /api/v1/child/exploits", s.handleChildExploits)
	mux.HandleFunc("GET /api/v1/child/result-card/{runID}", s.handleChildResultCard)
	mux.HandleFunc("POST /api/v1/lists/{id}/dictee-result", s.handleListDicteeResult)
	mux.HandleFunc("POST /api/v1/children/{id}/quick-word", s.handleQuickWord)
	mux.HandleFunc("GET /api/v1/children/{id}/dashboard", s.handleChildDashboard)
	mux.HandleFunc("POST /api/v1/lists/{id}/items/{itemID}/sentences/generate", s.handleGenerateSentences)
	mux.HandleFunc("POST /api/v1/lists/{id}/items/{itemID}/sentences/manual", s.handleManualSentence)
	mux.HandleFunc("POST /api/v1/lists/{id}/sentences/{sid}/approve", s.handleApproveSentence)
	mux.HandleFunc("POST /api/v1/lists/{id}/sentences/{sid}/regenerate", s.handleRegenerateSentence)
	mux.HandleFunc("POST /api/v1/lists/{id}/items/{itemID}/audio", s.handleUploadItemAudio)
	mux.HandleFunc("POST /api/v1/lists/{id}/sentences/{sid}/audio", s.handleUploadSentenceAudio)
	return mux
}
