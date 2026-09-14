package parent

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// pseudoMaxLen bounds a child's pseudo length: generous for a first name or
// a made-up one, short enough to render on the smallest card.
const pseudoMaxLen = 24

// patternPattern matches a valid tap pattern: 5 to 6 digits, each 1-9, one
// per tapped cell of the 3x3 grid (ENCRE_01 §17, raised from 4 by
// encre-qpx.4 — a 4-tap pattern on a 9-cell grid is only ~13 bits, and no
// argon2id parameter this codebase could practically run protects a secret
// that small against an offline guesser who already has the pepper key; one
// or two more taps is worth more than any cost-parameter change). It is not
// a full distinct-cells check — a client bug or a parent replaying a cell
// would still pass — but ENCRE_04 §7's real defense lives in the peppered
// argon2id hash, not in the shape of the string that goes in; see
// [weakPattern] for the handful of shapes rejected on top of length alone.
var patternPattern = regexp.MustCompile(`^[1-9]{5,6}$`)

// weakPattern reports whether pattern is one of the handful of shapes worth
// rejecting outright even though they pass [patternPattern]: every digit the
// same, a run in strict ascending or descending order, or a pattern that
// only ever taps the 3x3 grid's four corners (1, 3, 7, 9 — the numeric-pad
// reading [patternPattern] assumes) — the patterns a class of seven-year-
// olds converges on if left to choose freely, per the same reasoning
// ENCRE_04 §7 already applies to blacklisting 1111/1234/0000 at length 4.
func weakPattern(pattern string) bool {
	if allSameDigit(pattern) {
		return true
	}
	if isRun(pattern, 1) || isRun(pattern, -1) {
		return true
	}
	return cornersOnly(pattern)
}

func allSameDigit(pattern string) bool {
	for i := 1; i < len(pattern); i++ {
		if pattern[i] != pattern[0] {
			return false
		}
	}
	return true
}

// isRun reports whether pattern is a strict run of consecutive digits with
// the given step (1 for ascending, -1 for descending) — "12345" or "654321"
// — accounting for the numeric-pad's own wraparound at the row boundary is
// deliberately not attempted: a plain digit run is the case worth blocking,
// not every geometric line the grid admits.
func isRun(pattern string, step int) bool {
	for i := 1; i < len(pattern); i++ {
		if int(pattern[i]) != int(pattern[i-1])+step {
			return false
		}
	}
	return true
}

// pad corners are the 3x3 grid's four corner cells in a numeric-pad reading
// (7 8 9 / 4 5 6 / 1 2 3).
var padCorners = []byte{'1', '3', '7', '9'}

func cornersOnly(pattern string) bool {
	for i := range len(pattern) {
		if !slices.Contains(padCorners, pattern[i]) {
			return false
		}
	}
	return true
}

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
		s.renderChildren(w, r, "le motif doit avoir 5 ou 6 points sur la grille")
		return
	case weakPattern(pattern):
		s.renderChildren(w, r, "ce motif est trop facile à deviner (pas de suite, pas de répétition, pas que les coins)")
		return
	}

	patternHash, err := auth.HashPattern(pattern, s.pep)
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
