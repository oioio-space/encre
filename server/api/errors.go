package api

import (
	"errors"
	"net/http"

	"github.com/oioio-space/encre/server/store"
)

// errorBody is the shape of every error response this package writes.
type errorBody struct {
	Error string `json:"error"`
}

// writeError writes {"error": msg} with the given status. msg is shown to
// the client, so it must never repeat an internal error's text — see
// [Server.writeStoreError] for the handlers that catch one.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// writeStoreError answers a [server/store.Store] error: [store.ErrNotFound]
// becomes a 404 with notFoundMsg, and anything else becomes a 500 with a
// message that names nothing about the failure — the failure itself is
// logged server-side, never handed to the client (ENCRE_04 §7, the same
// contract server/auth holds for its own errors).
func (s *Server) writeStoreError(w http.ResponseWriter, err error, notFoundMsg string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, notFoundMsg)
		return
	}
	s.log.Error("store error", "error", err)
	writeError(w, http.StatusInternalServerError, "erreur serveur")
}
