package parent

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// Server implements [net/http.Handler] for ENCRE's parent panel: connexion,
// enfants and réglages (ENCRE_04 §11, ENCRE_01 §17). Build one with
// [NewServer]; its zero value is not usable.
type Server struct {
	db   *store.Store
	tmpl *template.Template
	mux  *http.ServeMux
	csrf *csrfSigner

	loginLimiter   *auth.Limiter
	trustedProxies []*net.IPNet

	// clock supplies the current time; nil means [time.Now]. Tests set it
	// to move time without sleeping.
	clock func() time.Time
}

// NewServer builds a [Server] backed by db. db is not owned by the returned
// Server — the caller opened it and must close it.
func NewServer(db *store.Store) (*Server, error) {
	tmpl, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	csrf, err := newCSRFSigner()
	if err != nil {
		return nil, err
	}

	s := &Server{
		db:           db,
		tmpl:         tmpl,
		csrf:         csrf,
		loginLimiter: auth.NewLimiter(auth.LoginRateLimit, nil),
	}
	if err := s.routes(); err != nil {
		return nil, err
	}
	return s, nil
}

// ServeHTTP implements [net/http.Handler].
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// routes builds s.mux: every /parent/ route except the login page and
// static assets requires [Server.requireParentSession], and every mutating
// route additionally requires [Server.requireCSRF].
func (s *Server) routes() error {
	static, err := staticHandler()
	if err != nil {
		return fmt.Errorf("building static handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET "+staticPrefix, static)

	mux.HandleFunc("GET /parent/login", s.handleLoginGet)
	mux.HandleFunc("POST /parent/login", s.handleLoginPost)

	mux.Handle("POST /parent/logout", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleLogout))))

	mux.Handle("GET /parent/children", s.requireParentSession(http.HandlerFunc(s.handleChildrenGet)))
	mux.Handle("POST /parent/children", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleChildrenPost))))

	mux.Handle("GET /parent/children/{id}/settings", s.requireParentSession(http.HandlerFunc(s.handleSettingsGet)))
	mux.Handle("POST /parent/children/{id}/settings", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleSettingsPost))))

	mux.HandleFunc("GET /parent/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/parent/login", http.StatusSeeOther)
	})

	s.mux = mux
	return nil
}
