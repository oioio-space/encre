package parent

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
	"github.com/oioio-space/encre/server/store"
)

// listsPageData is what the semaine overview template renders: every list a
// child has, newest first (see [store.Store.ListsOfChild]).
type listsPageData struct {
	CSRFToken string
	Child     *store.Child
	Lists     []*store.WordList
	Error     string
}

// handleListsGet renders the semaine overview for one child: every word
// list, and the link to start a new one (ENCRE_04 §11).
func (s *Server) handleListsGet(w http.ResponseWriter, r *http.Request) {
	child, ok := s.childOwnedBySession(w, r)
	if !ok {
		return
	}
	lists, err := s.db.ListsOfChild(r.Context(), child.ID)
	if err != nil {
		s.logInternalError(w, r, "loading lists", err)
		return
	}
	s.render(w, r, "lists.html", listsPageData{
		CSRFToken: csrfTokenFromContext(r.Context()),
		Child:     child,
		Lists:     lists,
	})
}

// listNewPageData is what the collage form template renders.
type listNewPageData struct {
	CSRFToken string
	Child     *store.Child
	Error     string
}

// handleListNewGet renders the "coller une liste" form: raw text, kind.
func (s *Server) handleListNewGet(w http.ResponseWriter, r *http.Request) {
	child, ok := s.childOwnedBySession(w, r)
	if !ok {
		return
	}
	s.render(w, r, "lists_new.html", listNewPageData{
		CSRFToken: csrfTokenFromContext(r.Context()),
		Child:     child,
	})
}

// handleListsPost is the first step of ENCRE_04 §11's semaine flow: it
// analyses rawText per kind ([buildItems]), creates the list and every item
// it produced, and sends the parent straight to [handleListGet] — the
// validation page — to review them. Analysis never blocks on a lexicon
// miss; an unsure item is created like any other, just flagged for the
// validation step to require confirming (ENCRE_05 backlog, encre-018).
func (s *Server) handleListsPost(w http.ResponseWriter, r *http.Request) {
	child, ok := s.childOwnedBySession(w, r)
	if !ok {
		return
	}

	kind := r.PostFormValue("kind")
	rawText := r.PostFormValue("raw_text")
	label := strings.TrimSpace(r.PostFormValue("label"))
	if label == "" {
		label = s.now().Format("2006-01-02")
	}

	items, err := buildItems(lexique.Embedded(), "", kind, rawText)
	if err != nil {
		s.renderListNewError(w, r, child, "impossible d'analyser ce texte : "+err.Error())
		return
	}

	listID, err := newID()
	if err != nil {
		s.logInternalError(w, r, "generating list id", err)
		return
	}
	shareCode, err := newID()
	if err != nil {
		s.logInternalError(w, r, "generating list share code", err)
		return
	}

	list := &store.WordList{
		ID: listID, ChildID: child.ID, Label: label, ShareCode: shareCode,
		CreatedAt: s.now().UTC(),
	}
	if err := s.db.CreateList(r.Context(), list); err != nil {
		s.logInternalError(w, r, "creating list", err)
		return
	}
	for _, item := range items {
		item.ListID = listID
		item.Enabled = true
		if err := s.db.SaveItem(r.Context(), item); err != nil {
			s.logInternalError(w, r, "saving item", err)
			return
		}
	}

	http.Redirect(w, r, "/parent/lists/"+listID, http.StatusSeeOther)
}

func (s *Server) renderListNewError(w http.ResponseWriter, r *http.Request, child *store.Child, errMessage string) {
	s.render(w, r, "lists_new.html", listNewPageData{
		CSRFToken: csrfTokenFromContext(r.Context()),
		Child:     child,
		Error:     errMessage,
	})
}

// listItemView is one item as the validation page shows it: the store row
// plus the Couleurs derived into a display-ready, sorted slice.
type listItemView struct {
	*store.Item
	Colors []colorCount
	Unsure bool
}

// colorCount is one Couleur's trap count, in the fixed order
// [engine.Colors] lists them, for a stable render across requests. Slug is
// the URL segment [parseColor] accepts back, precomputed here so the
// template never has to lowercase or strip an accent itself.
type colorCount struct {
	Color engine.Color
	Count int
	Slug  string
}

// listPageData is what the validation ("collage → validation → voix →
// date") page renders.
type listPageData struct {
	CSRFToken string
	Child     *store.Child
	List      *store.WordList
	Items     []listItemView
	// RecordedCount and Total feed ENCRE_04 §11's "7 / 20" progress counter
	// for the voice-recording step.
	RecordedCount, Total int
	Error                string
}

// handleListGet renders the validation page for one list: every item with
// its Couleurs (tap-correctable), its confirmation state, and its
// recording state, plus the due-date field that closes the flow.
func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) {
	list, child, ok := s.listOwnedBySession(w, r)
	if !ok {
		return
	}
	s.renderList(w, r, list, child, "")
}

func (s *Server) renderList(w http.ResponseWriter, r *http.Request, list *store.WordList, child *store.Child, errMessage string) {
	items, err := s.db.ItemsOfList(r.Context(), list.ID)
	if err != nil {
		s.logInternalError(w, r, "loading items", err)
		return
	}
	slices.SortFunc(items, func(a, b *store.Item) int { return strings.Compare(a.ID, b.ID) })

	views := make([]listItemView, 0, len(items))
	recorded := 0
	for _, item := range items {
		if item.AudioPath != "" {
			recorded++
		}
		views = append(views, listItemView{
			Item:   item,
			Colors: sortedColors(item.Colors),
			Unsure: itemUnsure(item),
		})
	}

	s.render(w, r, "list.html", listPageData{
		CSRFToken:     csrfTokenFromContext(r.Context()),
		Child:         child,
		List:          list,
		Items:         views,
		RecordedCount: recorded,
		Total:         len(items),
		Error:         errMessage,
	})
}

// itemUnsure reports whether item needs the parent's confirmation before
// [Store.ValidateList] will accept its list — see [lexique.Analysis.Unsure],
// which item.Confidence is drawn from.
func itemUnsure(item *store.Item) bool {
	return item.Confidence < lexique.UnsureConfidence && !item.Confirmed
}

// sortedColors turns colors into a stable, ordered slice for the template,
// dropping any Couleur with a zero count (a parent's tap-correction can
// bring one to zero without removing the map entry).
func sortedColors(colors map[engine.Color]int) []colorCount {
	out := make([]colorCount, 0, len(colors))
	for _, c := range engine.Colors() {
		if n := colors[c]; n > 0 {
			out = append(out, colorCount{Color: c, Count: n, Slug: stripDiacritics(c.String())})
		}
	}
	return out
}

// handleListValidatePost closes the semaine flow: it optionally sets the
// list's due date (the form's due_date field, left blank to keep whatever
// was set before), then validates the list — refusing, per encre-018's
// acceptance, if any item is still [itemUnsure].
func (s *Server) handleListValidatePost(w http.ResponseWriter, r *http.Request) {
	list, child, ok := s.listOwnedBySession(w, r)
	if !ok {
		return
	}

	if raw := r.PostFormValue("due_date"); raw != "" {
		due, err := time.Parse("2006-01-02", raw)
		if err != nil {
			s.renderList(w, r, list, child, "date invalide")
			return
		}
		if err := s.db.SetListDueDate(r.Context(), list.ID, due); err != nil {
			s.logInternalError(w, r, "setting due date", err)
			return
		}
	}

	items, err := s.db.ItemsOfList(r.Context(), list.ID)
	if err != nil {
		s.logInternalError(w, r, "loading items", err)
		return
	}
	if i := slices.IndexFunc(items, itemUnsure); i >= 0 {
		s.renderList(w, r, list, child,
			fmt.Sprintf("« %s » est à vérifier : confirmez-le avant de valider la liste", items[i].Text))
		return
	}

	if err := s.db.ValidateList(r.Context(), list.ID); err != nil {
		s.logInternalError(w, r, "validating list", err)
		return
	}
	http.Redirect(w, r, "/parent/children/"+child.ID+"/lists", http.StatusSeeOther)
}

// handleItemPatch corrects an item's text (kindWord items only — a
// kindSentence or kindDictation item's Targets are rune offsets into its
// Text, which a free-text edit would invalidate; those are corrected by
// deleting the item and pasting it again). Editing re-runs
// [lexique.Lexicon.Analyze] and resets Confirmed to false: a corrected word
// has not been reviewed under its new text yet, whatever its old confidence
// said.
func (s *Server) handleItemPatch(w http.ResponseWriter, r *http.Request) {
	list, child, item, ok := s.itemOwnedBySession(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "formulaire invalide")
		return
	}
	if text := strings.TrimSpace(r.PostFormValue("text")); text != "" && item.Kind == engine.KindWord {
		a := lexique.Embedded().Analyze(text)
		item.Text = text
		item.Colors = a.Traps
		item.Rules = a.Rules()
		item.Family = a.Family
		item.Confidence = a.Confidence
		item.Confirmed = false
	}
	if family := r.PostFormValue("family"); family != "" {
		item.Family = family
	}

	if err := s.db.SaveItem(r.Context(), item); err != nil {
		s.logInternalError(w, r, "saving corrected item", err)
		return
	}
	s.renderList(w, r, list, child, "")
}

// handleItemConfirmPost is the "à vérifier" tap: it accepts item's analysis
// as-is, without changing anything about it.
func (s *Server) handleItemConfirmPost(w http.ResponseWriter, r *http.Request) {
	list, child, item, ok := s.itemOwnedBySession(w, r)
	if !ok {
		return
	}
	item.Confirmed = true
	if err := s.db.SaveItem(r.Context(), item); err != nil {
		s.logInternalError(w, r, "confirming item", err)
		return
	}
	s.renderList(w, r, list, child, "")
}

// handleItemColorRemovePost is ENCRE_05's "Couleurs corrigeables d'un tap":
// one tap removes one trap of the named Couleur from item, for when the
// analysis over-called something the parent knows is not actually a trap
// here. It is a no-op, not an error, when the Couleur is already at zero —
// a parent double-tapping should never see a failure.
func (s *Server) handleItemColorRemovePost(w http.ResponseWriter, r *http.Request) {
	list, child, item, ok := s.itemOwnedBySession(w, r)
	if !ok {
		return
	}
	color, ok := parseColor(r.PathValue("color"))
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "introuvable")
		return
	}
	if item.Colors[color] > 0 {
		item.Colors[color]--
		if item.Colors[color] == 0 {
			delete(item.Colors, color)
		}
		if err := s.db.SaveItem(r.Context(), item); err != nil {
			s.logInternalError(w, r, "correcting item color", err)
			return
		}
	}
	s.renderList(w, r, list, child, "")
}

// parseColor resolves the {color} path segment ("muettes", "jumelles"…)
// against [engine.Color.String], case-insensitively and diacritic-free
// since a URL segment a parent's browser sent should not have to carry a
// literal "é".
func parseColor(name string) (engine.Color, bool) {
	for _, c := range engine.Colors() {
		if strings.EqualFold(stripDiacritics(c.String()), name) {
			return c, true
		}
	}
	return 0, false
}

func stripDiacritics(s string) string {
	return strings.NewReplacer("é", "e", "è", "e", "ê", "e").Replace(strings.ToLower(s))
}

// handleItemAudioPost stores a parent's recording for one item — ENCRE_04
// §11's "gros bouton par item" — under [Server.mediaRoot]/{listID}/{itemID}.
// It writes whatever bytes the browser's MediaRecorder produced as-is; the
// webm→ogg transcode ENCRE_04 §8 describes is encre-6z2's job (encre-018
// blocks it, deliberately: the parent flow must not wait on that pipeline
// existing), so item.AudioPath here names the file exactly as uploaded.
//
// The request's body was already read and size-capped by [requireCSRF]'s
// [parseAnyForm] (see [maxRequestBytes]) before this handler ever runs, so
// there is nothing left to parse here — only [http.Request.FormFile] to
// read from what parseAnyForm already populated.
func (s *Server) handleItemAudioPost(w http.ResponseWriter, r *http.Request) {
	list, child, item, ok := s.itemOwnedBySession(w, r)
	if !ok {
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		s.renderList(w, r, list, child, "aucun enregistrement reçu")
		return
	}
	defer func() { _ = file.Close() }()

	relPath := filepath.Join(list.ID, item.ID+audioExt(header.Filename))
	if err := s.writeMedia(relPath, file); err != nil {
		s.logInternalError(w, r, "writing item audio", err)
		return
	}

	item.AudioPath = filepath.ToSlash(relPath)
	if err := s.db.SaveItem(r.Context(), item); err != nil {
		s.logInternalError(w, r, "saving item audio path", err)
		return
	}
	s.renderList(w, r, list, child, "")
}

// audioExt returns the uploaded filename's extension, or ".webm" — what
// MediaRecorder produces by default — when it has none, so a stored path
// always ends in something a later transcode step can dispatch on.
func audioExt(filename string) string {
	if ext := filepath.Ext(filename); ext != "" {
		return ext
	}
	return ".webm"
}

// writeMedia writes src to relPath under s.mediaRoot, creating any
// directory the path needs. It goes through [os.OpenRoot] rather than a
// plain [os.MkdirAll]/[os.OpenFile] pair — even though relPath is built
// from this package's own opaque, [crypto/rand]-generated IDs, never a
// filename a browser sent — so a future caller of writeMedia cannot
// reintroduce a path-traversal write by passing it something less trusted
// without the write itself refusing to escape mediaRoot.
func (s *Server) writeMedia(relPath string, src io.Reader) error {
	rootPath := s.mediaRoot
	if rootPath == "" {
		rootPath = defaultMediaRoot
	}
	if err := os.MkdirAll(rootPath, 0o750); err != nil {
		return fmt.Errorf("creating media root: %w", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("opening media root: %w", err)
	}
	defer func() { _ = root.Close() }()

	if err := root.MkdirAll(filepath.Dir(relPath), 0o750); err != nil {
		return fmt.Errorf("creating media directory: %w", err)
	}
	dst, err := root.OpenFile(relPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating media file: %w", err)
	}
	defer func() { _ = dst.Close() }()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("writing media file: %w", err)
	}
	return nil
}

// listOwnedBySession loads the list named by the "id" path value and
// confirms it — through its child — belongs to the session's parent. On
// failure it has already written the response and returns ok == false;
// callers must return immediately.
func (s *Server) listOwnedBySession(w http.ResponseWriter, r *http.Request) (*store.WordList, *store.Child, bool) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		s.renderError(w, r, http.StatusForbidden, "session invalide")
		return nil, nil, false
	}
	list, err := s.db.ListByID(r.Context(), r.PathValue("id"))
	if err != nil {
		s.logInternalError(w, r, "loading list", err)
		return nil, nil, false
	}
	child, err := s.db.ChildByID(r.Context(), list.ChildID)
	if err != nil {
		s.logInternalError(w, r, "loading list's child", err)
		return nil, nil, false
	}
	if child.ParentID != sess.SubjectID {
		s.renderError(w, r, http.StatusNotFound, "introuvable")
		return nil, nil, false
	}
	return list, child, true
}

// itemOwnedBySession loads the item named by the "itemID" path value inside
// the list [listOwnedBySession] resolves, confirming both belong to the
// session's parent.
func (s *Server) itemOwnedBySession(w http.ResponseWriter, r *http.Request) (*store.WordList, *store.Child, *store.Item, bool) {
	list, child, ok := s.listOwnedBySession(w, r)
	if !ok {
		return nil, nil, nil, false
	}
	item, err := s.db.ItemByID(r.Context(), r.PathValue("itemID"))
	if err != nil {
		s.logInternalError(w, r, "loading item", err)
		return nil, nil, nil, false
	}
	if item.ListID != list.ID {
		s.renderError(w, r, http.StatusNotFound, "introuvable")
		return nil, nil, nil, false
	}
	return list, child, item, true
}

// render writes a template with a generic "text/html" content type header,
// the one thing every GET-rendering handler in this package repeats.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slogRenderError(r, name, err)
	}
}

// handleQuickWordPost is ENCRE_04 §11's "ajouter ce mot" button: it appends
// one word to the child's standing "Mes mots" list (created on first use,
// already validated so the word is immediately playable), skipping the
// full collage → validation flow on purpose — a quick add is meant to take
// one tap, not a review pass. The word is auto-confirmed for the same
// reason: the parent typed this exact word deliberately, which is itself
// the confirmation encre-018 asks the multi-word collage flow for
// separately.
func (s *Server) handleQuickWordPost(w http.ResponseWriter, r *http.Request) {
	child, ok := s.childOwnedBySession(w, r)
	if !ok {
		return
	}
	word := strings.TrimSpace(r.PostFormValue("text"))
	if word == "" {
		s.renderError(w, r, http.StatusBadRequest, "mot manquant")
		return
	}

	list, err := s.quickWordList(r, child.ID)
	if err != nil {
		s.logInternalError(w, r, "loading quick word list", err)
		return
	}

	items, err := buildItems(lexique.Embedded(), list.ID, kindWord, word)
	if err != nil {
		s.renderError(w, r, http.StatusBadRequest, "mot invalide")
		return
	}
	item := items[0]
	item.Confirmed = true
	item.Enabled = true
	if err := s.db.SaveItem(r.Context(), item); err != nil {
		s.logInternalError(w, r, "saving quick word", err)
		return
	}
	http.Redirect(w, r, "/parent/lists/"+list.ID, http.StatusSeeOther)
}

// quickWordLabel names the standing list [handleQuickWordPost] appends to.
const quickWordLabel = "Mes mots"

// quickWordList returns childID's "Mes mots" list, creating and validating
// it on first use.
func (s *Server) quickWordList(r *http.Request, childID string) (*store.WordList, error) {
	lists, err := s.db.ListsOfChild(r.Context(), childID)
	if err != nil {
		return nil, err
	}
	if i := slices.IndexFunc(lists, func(l *store.WordList) bool { return l.Label == quickWordLabel }); i >= 0 {
		return lists[i], nil
	}

	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("generating quick word list id: %w", err)
	}
	shareCode, err := newID()
	if err != nil {
		return nil, fmt.Errorf("generating quick word list share code: %w", err)
	}
	list := &store.WordList{
		ID: id, ChildID: childID, Label: quickWordLabel, ShareCode: shareCode,
		Validated: true, CreatedAt: s.now().UTC(),
	}
	if err := s.db.CreateList(r.Context(), list); err != nil {
		return nil, err
	}
	return list, nil
}

// handleDicteeResultPost records a dictée's outcome — ENCRE_04 §7's
// "dictee-result". Per encre-0qo's acceptance ("bonus positif uniquement,
// jamais de malus"), only items the parent marked correct are written at
// all: a wrong answer on paper leaves no row here, so nothing downstream
// can ever turn a dictée result into a penalty by consuming this table.
// Fields are posted as "correct.{itemID}"="on" for whichever items the
// parent checked.
func (s *Server) handleDicteeResultPost(w http.ResponseWriter, r *http.Request) {
	list, child, ok := s.listOwnedBySession(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "formulaire invalide")
		return
	}

	items, err := s.db.ItemsOfList(r.Context(), list.ID)
	if err != nil {
		s.logInternalError(w, r, "loading items", err)
		return
	}
	for _, item := range items {
		if r.PostFormValue("correct."+item.ID) != "on" {
			continue
		}
		id, err := newID()
		if err != nil {
			s.logInternalError(w, r, "generating dictee result id", err)
			return
		}
		result := &store.DicteeResult{ID: id, ListID: list.ID, ItemID: item.ID, Correct: true, EnteredAt: s.now().UTC()}
		if err := s.db.SaveDicteeResult(r.Context(), result); err != nil {
			s.logInternalError(w, r, "saving dictee result", err)
			return
		}
	}
	s.renderList(w, r, list, child, "")
}
