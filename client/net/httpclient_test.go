package net_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oioio-space/encre/client/net"
)

func TestHTTPSenderClassifiesResponses(t *testing.T) {
	tests := map[string]struct {
		status int
		want   net.Result
	}{
		"200 is applied":         {http.StatusOK, net.ResultApplied},
		"409 is already applied": {http.StatusConflict, net.ResultAlreadyApplied},
		"400 is rejected":        {http.StatusBadRequest, net.ResultRejected},
		"403 is rejected":        {http.StatusForbidden, net.ResultRejected},
		"500 is unreachable":     {http.StatusInternalServerError, net.ResultUnreachable},
		"503 is unreachable too": {http.StatusServiceUnavailable, net.ResultUnreachable},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.Path, "/run/run-1/finish"; got != want {
					t.Errorf("request path = %s, want %s", got, want)
				}
				w.WriteHeader(tc.status)
			}))
			t.Cleanup(srv.Close)

			sender := net.NewHTTPSender(srv.URL)
			got, err := sender.SendFinish(t.Context(), net.Job{RunID: "run-1"})
			if err != nil {
				t.Fatalf("SendFinish: %v", err)
			}
			if got != tc.want {
				t.Errorf("SendFinish() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHTTPSenderReportsAnUnreachableServerRatherThanAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // closed before use: every request now refuses the connection

	sender := net.NewHTTPSender(srv.URL)
	got, err := sender.SendFinish(t.Context(), net.Job{RunID: "run-1"})
	if err != nil {
		t.Fatalf("SendFinish: %v, want a classified Result and a nil error", err)
	}
	if got != net.ResultUnreachable {
		t.Errorf("SendFinish() = %v, want ResultUnreachable", got)
	}
}
