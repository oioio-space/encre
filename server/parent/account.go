package parent

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// exportChild is one child's complete data, as [handleExportGet] writes it:
// the account fields plus every word list and its items, verbatim from the
// store — nothing summarized, nothing left out, per ENCRE_04 §7's "export
// et suppression complets".
type exportChild struct {
	Child *store.Child      `json:"child"`
	Lists []exportChildList `json:"lists"`
}

// exportChildList is one word list with its items and any recorded dictée
// results, inside an [exportChild].
type exportChildList struct {
	List    *store.WordList       `json:"list"`
	Items   []*store.Item         `json:"items"`
	Dictees []*store.DicteeResult `json:"dictee_results"`
}

// exportData is the whole of a parent's account, as [handleExportGet] sends
// it: this is every row this codebase's schema attaches to the parent's
// ID — see [github.com/oioio-space/encre/server/store]'s schema, `parents`
// down to `dictee_results`, all reachable by FOREIGN KEY from parent_id or
// child_id.
type exportData struct {
	Parent   *store.Parent `json:"parent"`
	Children []exportChild `json:"children"`
}

// handleExportGet writes the session's whole account as a JSON download —
// ENCRE_04 §7's "GET /export". It is reachable through
// [Server.requireParentSession] alone, which already requires a fresh TOTP
// check on every request (see that method's doc comment), so no further
// gate is added here for what is, after account deletion, the most
// sensitive thing this package serves.
func (s *Server) handleExportGet(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		s.renderError(w, r, http.StatusForbidden, "session invalide")
		return
	}

	data, err := s.exportParent(r, sess.SubjectID)
	if err != nil {
		s.logInternalError(w, r, "exporting parent data", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="encre-export.json"`)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slogRenderError(r, "export.json", err)
	}
}

func (s *Server) exportParent(r *http.Request, parentID string) (*exportData, error) {
	p, err := s.db.ParentByID(r.Context(), parentID)
	if err != nil {
		return nil, fmt.Errorf("loading parent: %w", err)
	}
	children, err := s.db.ChildrenOfParent(r.Context(), parentID)
	if err != nil {
		return nil, fmt.Errorf("loading children: %w", err)
	}

	data := &exportData{Parent: p, Children: make([]exportChild, 0, len(children))}
	for _, c := range children {
		ec, err := s.exportChild(r, c)
		if err != nil {
			return nil, err
		}
		data.Children = append(data.Children, ec)
	}
	return data, nil
}

func (s *Server) exportChild(r *http.Request, c *store.Child) (exportChild, error) {
	lists, err := s.db.ListsOfChild(r.Context(), c.ID)
	if err != nil {
		return exportChild{}, fmt.Errorf("loading lists of child %s: %w", c.ID, err)
	}

	ec := exportChild{Child: c, Lists: make([]exportChildList, 0, len(lists))}
	for _, l := range lists {
		items, err := s.db.ItemsOfList(r.Context(), l.ID)
		if err != nil {
			return exportChild{}, fmt.Errorf("loading items of list %s: %w", l.ID, err)
		}
		dictees, err := s.db.DicteeResultsOfList(r.Context(), l.ID)
		if err != nil {
			return exportChild{}, fmt.Errorf("loading dictee results of list %s: %w", l.ID, err)
		}
		ec.Lists = append(ec.Lists, exportChildList{List: l, Items: items, Dictees: dictees})
	}
	return ec, nil
}

// accountDeleteConfirmation is the exact text [handleAccountDeletePost]
// requires in the form's "confirm" field — a lightweight guard against a
// mis-tapped button on a destructive, irreversible action, in the same
// spirit as GitHub's "type the repository name to confirm".
const accountDeleteConfirmation = "SUPPRIMER"

// handleAccountDeletePost is ENCRE_04 §7's "DELETE /account", reached
// through a plain POST form since a browser form cannot send DELETE: it
// revokes every session belonging to the parent — so a browser tab left
// open elsewhere loses access immediately, not just this one — then
// deletes the parent row, which cascades to every child, list, item, run
// and session underneath it (see [store.Store.DeleteParent]). There is no
// export performed here: a parent who wants their data first uses
// [handleExportGet], which this handler does not require, because forcing
// an export on every deletion would make "I don't want my data, don't make
// me download it first" impossible to honour.
func (s *Server) handleAccountDeletePost(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		s.renderError(w, r, http.StatusForbidden, "session invalide")
		return
	}
	if r.PostFormValue("confirm") != accountDeleteConfirmation {
		s.renderError(w, r, http.StatusBadRequest, `tapez "SUPPRIMER" pour confirmer`)
		return
	}

	// Every session for this subject, not only the one on r: a browser tab
	// left open elsewhere must lose access immediately too (encre-qpx.6).
	if _, err := s.db.DeleteSessionsForSubject(r.Context(), sess.SubjectID); err != nil {
		s.logInternalError(w, r, "revoking sessions before account deletion", err)
		return
	}
	if err := s.db.DeleteParent(r.Context(), sess.SubjectID); err != nil {
		s.logInternalError(w, r, "deleting parent account", err)
		return
	}

	auth.ClearSessionCookie(w, auth.CookieParent)
	http.Redirect(w, r, "/parent/login", http.StatusSeeOther)
}
