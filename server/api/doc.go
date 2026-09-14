// Package api implements ENCRE's child-facing /api/v1 HTTP endpoints
// (brief/ENCRE_04 §7): login, the profile screen, and the four calls a run
// makes from start to finish.
//
// It is a thin layer over [github.com/oioio-space/encre/server/store] and
// [github.com/oioio-space/encre/server/auth], neither of which it replaces:
// this package decides what a request is allowed to do and which engine
// function answers it, and leaves persistence and authentication to those
// two. The one rule every handler here enforces is ENCRE_04 §4's "the server
// is the authority": nothing a client sends — a score, a combo, a target, a
// deck — is trusted for anything that matters. [engine.Replay] recomputes a
// run's outcome from its attempts and the deck's own recorded seed, and that
// recomputation is the one that counts.
//
// # Idempotence, end to end (encre-qpx.3)
//
// [engine.Apply] guards against scoring a run twice with
// [engine.Child.AppliedRuns], but server/store does not persist that map —
// it normalizes on runs.applied instead ([server/store.Store.ChildByID]
// always returns a Child with an empty AppliedRuns). [handleRunFinish]
// therefore calls [server/store.Store.MarkApplied] — a single atomic
// UPDATE ... WHERE applied = 0 — before calling Apply, and that call is what
// decides whether a finish request's effects are written at all: two
// concurrent or retried finish calls for the same run race MarkApplied, at
// most one wins, and Apply's own AppliedRuns check never gets to matter
// (which is why leaving it in Apply is harmless rather than redundant only
// in theory). [engine.Replay] runs first, before MarkApplied: it is a pure
// function that changes nothing, so an invalid request can be rejected
// with a 400 without ever spending the one chance MarkApplied grants a run.
//
// # Authority a client cannot buy
//
// A typical wiring:
//
//	db, err := store.Open(dsn)
//	if err != nil {
//		log.Fatal(err)
//	}
//	srv := api.New(db, engine.DefaultConfig(), time.Now)
//	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
package api
