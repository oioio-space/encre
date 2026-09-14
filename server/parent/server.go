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
	pep  *auth.Pepper
	tmpl *template.Template
	mux  *http.ServeMux
	csrf *csrfSigner

	loginLimiter   *auth.Limiter
	trustedProxies []*net.IPNet

	// clock supplies the current time; nil means [time.Now]. Tests set it
	// to move time without sleeping.
	clock func() time.Time

	// mediaRoot is the directory [Server] writes a parent's recorded item
	// audio under (ENCRE_04 §8: media/{listID}/{itemID}). It defaults to
	// [defaultMediaRoot]; [Server.SetMediaRoot] overrides it, which tests
	// use to point it at a [testing.T.TempDir] instead of the working
	// directory a production process happens to start in.
	mediaRoot string
}

// defaultMediaRoot is [Server.mediaRoot]'s value until [Server.SetMediaRoot]
// is called: a directory named "media" relative to the process's working
// directory, matching where ENCRE_04 §8 already expects Item.AudioPath to
// resolve from.
const defaultMediaRoot = "media"

// SetMediaRoot overrides where s writes recorded item audio. It is meant
// for tests and for a production caller wiring a real data directory; the
// zero value ([defaultMediaRoot]) is not appropriate for a real deployment.
func (s *Server) SetMediaRoot(root string) {
	s.mediaRoot = root
}

// NewServer builds a [Server] backed by db. db is not owned by the returned
// Server — the caller opened it and must close it. pep must not be nil —
// see [auth.ErrPepperRequired] — it is what [auth.HashPattern] and
// [auth.LoginParent] need to hash and verify anything this package persists
// or checks against a stored secret.
func NewServer(db *store.Store, pep *auth.Pepper) (*Server, error) {
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
		pep:          pep,
		tmpl:         tmpl,
		csrf:         csrf,
		loginLimiter: auth.NewLimiter(auth.LoginRateLimit, nil),
		mediaRoot:    defaultMediaRoot,
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

	mux.Handle("GET /parent/children/{id}/lists", s.requireParentSession(http.HandlerFunc(s.handleListsGet)))
	mux.Handle("GET /parent/children/{id}/lists/new", s.requireParentSession(http.HandlerFunc(s.handleListNewGet)))
	mux.Handle("POST /parent/children/{id}/lists", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleListsPost))))
	mux.Handle("POST /parent/children/{id}/quick-word", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleQuickWordPost))))

	mux.Handle("GET /parent/lists/{id}", s.requireParentSession(http.HandlerFunc(s.handleListGet)))
	mux.Handle("POST /parent/lists/{id}/validate", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleListValidatePost))))
	mux.Handle("POST /parent/lists/{id}/dictee-result", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleDicteeResultPost))))
	mux.Handle("PATCH /parent/lists/{id}/items/{itemID}", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleItemPatch))))
	mux.Handle("POST /parent/lists/{id}/items/{itemID}/confirm", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleItemConfirmPost))))
	mux.Handle("POST /parent/lists/{id}/items/{itemID}/colors/{color}/remove", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleItemColorRemovePost))))
	mux.Handle("POST /parent/lists/{id}/items/{itemID}/audio", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleItemAudioPost))))

	mux.Handle("GET /parent/export", s.requireParentSession(http.HandlerFunc(s.handleExportGet)))
	mux.Handle("POST /parent/account/delete", s.requireParentSession(s.requireCSRF(http.HandlerFunc(s.handleAccountDeletePost))))

	mux.HandleFunc("GET /parent/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/parent/login", http.StatusSeeOther)
	})

	s.mux = mux
	return nil
}
