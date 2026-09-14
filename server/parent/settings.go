package parent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/oioio-space/encre/server/store"
)

// keyboardABC and keyboardAZERTY are the two values [childSettings.Keyboard]
// may hold.
const (
	keyboardABC    = "abc"
	keyboardAZERTY = "azerty"
)

// defaultMinutesPerDay is ENCRE_01 §17's default daily play-time limit.
const defaultMinutesPerDay = 20

// maxMinutesPerDay bounds what the réglages form accepts, generous enough
// for any limit a parent would reasonably set while still catching a typo
// (24h expressed in minutes, entered by mistake) before it reaches the
// child's row.
const maxMinutesPerDay = 240

// childSettings is the decoded form of [store.Child.Settings]: the panel's
// per-child toggles that have no column of their own (ENCRE_04 §6).
type childSettings struct {
	// Keyboard is [keyboardABC] or [keyboardAZERTY]. ENCRE_07 §4.2 forbids
	// offering AZERTY in portrait — see settingsFormData's Keyboard field —
	// but does not forbid a child who already has it set from keeping it
	// when the client later renders in portrait; that is the client's
	// layout decision, not this package's.
	Keyboard string `json:"keyboard"`
	// Cahier says whether the Cahier feature (ENCRE_01 §17) is on for this
	// child.
	Cahier bool `json:"cahier"`
	// Rules maps a [Rule.ID] to whether it is active for this child.
	// Entries absent from the map fall back to that rule's
	// [RuleGroup.DefaultOn].
	Rules map[string]bool `json:"rules"`
}

// defaultChildSettings is what a newly created child's settings start as:
// ABC keyboard, Cahier on, every fine rule at its [RuleGroup.DefaultOn] —
// which means the Sosies group starts off (ENCRE_03 §1).
func defaultChildSettings() childSettings {
	return childSettings{
		Keyboard: keyboardABC,
		Cahier:   true,
		Rules:    defaultRuleSettings(),
	}
}

// decodeChildSettings parses raw (a [store.Child.Settings] value) into a
// childSettings, filling in [defaultChildSettings] for anything raw does not
// set — including raw being empty, which every not-yet-migrated or
// freshly created row is.
func decodeChildSettings(raw []byte) (childSettings, error) {
	settings := defaultChildSettings()
	if len(raw) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return childSettings{}, fmt.Errorf("decoding child settings: %w", err)
	}
	if settings.Rules == nil {
		settings.Rules = defaultRuleSettings()
	}
	return settings, nil
}

// ruleEnabled reports whether rule is active in settings, falling back to
// its group's default when settings.Rules has no explicit entry for it.
func ruleEnabled(settings childSettings, ruleID string, groupDefault bool) bool {
	if on, ok := settings.Rules[ruleID]; ok {
		return on
	}
	return groupDefault
}

// childDailyLimit is the decoded form of [store.Child.DailyLimit]
// (ENCRE_01 §17).
type childDailyLimit struct {
	MinutesPerDay int `json:"minutes_per_day"`
}

// decodeDailyLimit parses raw into a childDailyLimit, defaulting to
// [defaultMinutesPerDay] when raw is empty.
func decodeDailyLimit(raw []byte) (childDailyLimit, error) {
	limit := childDailyLimit{MinutesPerDay: defaultMinutesPerDay}
	if len(raw) == 0 {
		return limit, nil
	}
	if err := json.Unmarshal(raw, &limit); err != nil {
		return childDailyLimit{}, fmt.Errorf("decoding daily limit: %w", err)
	}
	return limit, nil
}

// settingsFormData is what the réglages template renders.
type settingsFormData struct {
	CSRFToken  string
	Child      *store.Child
	Settings   childSettings
	Limit      childDailyLimit
	RuleGroups []RuleGroup
	Error      string
}

// handleSettingsGet renders the réglages page for one child.
func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	child, ok := s.childOwnedBySession(w, r)
	if !ok {
		return
	}
	s.renderSettings(w, r, child, "")
}

// handleSettingsPost applies the réglages form: daily limit, keyboard
// layout, Cahier on/off, and every fine rule toggle.
func (s *Server) handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	child, ok := s.childOwnedBySession(w, r)
	if !ok {
		return
	}

	minutes, err := strconv.Atoi(r.PostFormValue("minutes_per_day"))
	if err != nil || minutes < 0 || minutes > maxMinutesPerDay {
		s.renderSettings(w, r, child, fmt.Sprintf("la limite doit être un nombre de minutes entre 0 et %d", maxMinutesPerDay))
		return
	}

	keyboard := r.PostFormValue("keyboard")
	if keyboard != keyboardABC && keyboard != keyboardAZERTY {
		keyboard = keyboardABC
	}

	settings := childSettings{
		Keyboard: keyboard,
		Cahier:   r.PostFormValue("cahier") == "on",
		Rules:    make(map[string]bool),
	}
	for _, group := range ruleCatalog {
		for _, rule := range group.Rules {
			settings.Rules[rule.ID] = r.PostFormValue("rule."+rule.ID) == "on"
		}
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		s.logInternalError(w, r, "marshaling child settings", err)
		return
	}
	limitJSON, err := json.Marshal(childDailyLimit{MinutesPerDay: minutes})
	if err != nil {
		s.logInternalError(w, r, "marshaling daily limit", err)
		return
	}

	child.Settings = settingsJSON
	child.DailyLimit = limitJSON
	if err := s.db.SaveChild(r.Context(), child); err != nil {
		s.logInternalError(w, r, "saving child settings", err)
		return
	}

	s.renderSettings(w, r, child, "")
}

// childOwnedBySession loads the child named by the "id" path value and
// confirms it belongs to the session's parent. On failure it has already
// written the response and returns ok == false; callers must return
// immediately.
func (s *Server) childOwnedBySession(w http.ResponseWriter, r *http.Request) (*store.Child, bool) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		s.renderError(w, r, http.StatusForbidden, "session invalide")
		return nil, false
	}
	child, err := s.db.ChildByID(r.Context(), r.PathValue("id"))
	if err != nil {
		s.logInternalError(w, r, "loading child", err)
		return nil, false
	}
	if child.ParentID != sess.SubjectID {
		// Reported the same way as a missing child: this parent must not
		// learn that a given ID belongs to someone else's account.
		s.renderError(w, r, http.StatusNotFound, "introuvable")
		return nil, false
	}
	return child, true
}

// renderSettings writes the réglages page for child, with errMessage shown
// above the form if non-empty.
func (s *Server) renderSettings(w http.ResponseWriter, r *http.Request, child *store.Child, errMessage string) {
	settings, err := decodeChildSettings(child.Settings)
	if err != nil {
		s.logInternalError(w, r, "decoding child settings", err)
		return
	}
	limit, err := decodeDailyLimit(child.DailyLimit)
	if err != nil {
		s.logInternalError(w, r, "decoding daily limit", err)
		return
	}

	data := settingsFormData{
		CSRFToken:  csrfTokenFromContext(r.Context()),
		Child:      child,
		Settings:   settings,
		Limit:      limit,
		RuleGroups: ruleCatalog,
		Error:      errMessage,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "settings.html", data); err != nil {
		slogRenderError(r, "settings.html", err)
	}
}
