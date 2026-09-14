package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// adminMetricsHandler serves ENCRE_04 §12's operations dashboard: "parent
// seulement". It reuses server/auth's own session gate rather than
// reinventing one — a request with no valid [auth.CookieParent] session
// never reaches [queryAdminMetrics].
//
// A fresh TOTP window is not required here as it is for the mutating
// /parent/... routes server/api exposes: this endpoint only reads aggregate
// counts, nothing a parent session alone cannot already see indirectly
// through the dashboard, so the session cookie's own auth (ENCRE_04 §7) is
// the whole gate.
func adminMetricsHandler(db *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := auth.SessionFromCookie(r, db, auth.CookieParent, time.Now()); err != nil {
			http.Error(w, "session parent requise", http.StatusUnauthorized)
			return
		}

		metrics, err := queryAdminMetrics(r.Context(), db.DB())
		if err != nil {
			logger.Error("admin metrics query failed", "error", err)
			http.Error(w, "erreur interne", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			logger.Error("admin metrics encode failed", "error", err)
		}
	}
}
