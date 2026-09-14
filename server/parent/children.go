package parent

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// pseudoMaxLen bounds a child's pseudo length: generous for a first name or
// a made-up one, short enough to render on the smallest card.
const pseudoMaxLen = 24

// patternPattern matches a valid tap pattern: exactly four digits, each 1-9,
// one per tapped cell of the 3x3 grid (ENCRE_01 §17). It is not a full
// distinct-cells check — a client bug or a parent replaying a cell would
// still pass — but ENCRE_04 §7 lives in the argon2id hash, not in the shape
// of the string that goes in.
var patternPattern = regexp.MustCompile(`^[1-9]{4}$`)

// childrenPageData is what the enfants template renders.
type childrenPageData struct {
	CSRFToken string
	Children  []*store.Child
	Error     string
}

// handleChildrenGet lists the session's children and the "add a child"
// form.
func (s *Server) handleChildrenGet(w http.ResponseWriter, r *http.Request) {
	s.renderChildren(w, r, "")
}

// handleChildrenPost creates a child under the session's parent: pseudo,
// tap pattern, and the settings a new child always starts with —
// [defaultChildSettings], with the Sosies rule group off.
func (s *Server) handleChildrenPost(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		s.renderError(w, r, http.StatusForbidden, "session invalide")
		return
	}

	pseudo := r.PostFormValue("pseudo")
	pattern := r.PostFormValue("pattern")

	switch {
	case pseudo == "" || len([]rune(pseudo)) > pseudoMaxLen:
		s.renderChildren(w, r, "le pseudo doit faire entre 1 et 24 caractères")
		return
	case !patternPattern.MatchString(pattern):
		s.renderChildren(w, r, "le motif doit avoir 4 points sur la grille")
		return
	}

	patternHash, err := auth.HashPattern(pattern)
	if err != nil {
		s.logInternalError(w, r, "hashing child pattern", err)
		return
	}

	settingsJSON, err := json.Marshal(defaultChildSettings())
	if err != nil {
		s.logInternalError(w, r, "marshaling default child settings", err)
		return
	}
	limitJSON, err := json.Marshal(childDailyLimit{MinutesPerDay: defaultMinutesPerDay})
	if err != nil {
		s.logInternalError(w, r, "marshaling default daily limit", err)
		return
	}

	id, err := newID()
	if err != nil {
		s.logInternalError(w, r, "generating child id", err)
		return
	}

	child := &store.Child{
		ID:          id,
		ParentID:    sess.SubjectID,
		Pseudo:      pseudo,
		PatternHash: []byte(patternHash),
		DailyLimit:  limitJSON,
		Settings:    settingsJSON,
		CreatedAt:   s.now().UTC(),
	}
	if err := s.db.CreateChild(r.Context(), child); err != nil {
		s.logInternalError(w, r, "creating child", err)
		return
	}

	s.renderChildren(w, r, "")
}

// renderChildren writes the enfants page, with errMessage shown above the
// form if non-empty.
func (s *Server) renderChildren(w http.ResponseWriter, r *http.Request, errMessage string) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		s.renderError(w, r, http.StatusForbidden, "session invalide")
		return
	}
	children, err := s.db.ChildrenOfParent(r.Context(), sess.SubjectID)
	if err != nil {
		s.logInternalError(w, r, "loading children", err)
		return
	}

	data := childrenPageData{
		CSRFToken: csrfTokenFromContext(r.Context()),
		Children:  children,
		Error:     errMessage,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "children.html", data); err != nil {
		slogRenderError(r, "children.html", err)
	}
}
