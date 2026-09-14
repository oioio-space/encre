package game

import (
	"fmt"
	"time"

	"github.com/oioio-space/encre/client/net"
	"github.com/oioio-space/encre/engine"
)

// RunSession is the offline-and-resume plumbing of ENCRE_04 §2 and §10, above
// [net.Store] and [net.Queue]: the Run scene records an attempt through it,
// and does not need to know that doing so durably saves the run, or that
// finishing it means queueing rather than sending.
//
// The zero value is not usable; construct one with [NewRunSession] for a run
// just started, or resume one with [ResumeRunSession].
type RunSession struct {
	store net.Store
	run   engine.Run
}

// NewRunSession returns a RunSession over a freshly started run, not yet
// saved. Its first [RunSession.RecordAttempt] performs that first save.
func NewRunSession(store net.Store, run engine.Run) *RunSession {
	return &RunSession{store: store, run: run}
}

// ResumeRunSession looks for a run saved by an earlier session — the tab the
// child closed at the twelfth word, or the connection that dropped — and
// returns a RunSession over it and true. It returns nil, false when there is
// nothing to resume: no run was ever saved, the save was corrupted, or it was
// written by a version of the game this one does not recognise. All three
// cases mean the same thing to the caller — start at Boot, not mid-run — so
// RunSession does not distinguish them; see [net.LoadRunState] for why.
func ResumeRunSession(store net.Store) (*RunSession, bool) {
	rs, ok := net.LoadRunState(store)
	if !ok {
		return nil, false
	}
	return &RunSession{store: store, run: rs.Run}, true
}

// Run returns the run as it stands, attempts included — what the Run scene
// resumes drawing and scoring from.
func (s *RunSession) Run() engine.Run {
	return s.run
}

// RecordAttempt appends a to the run and saves the run state before
// returning, so the word the child just answered survives even a tab closed
// on the very next frame. ENCRE_04 §10 asks for exactly this cadence: saved
// after every word, not batched for later.
func (s *RunSession) RecordAttempt(a engine.Attempt) error {
	s.run.Attempts = append(s.run.Attempts, a)
	if err := net.SaveRunState(s.store, s.run, time.Now()); err != nil {
		return fmt.Errorf("game: saving run progress: %w", err)
	}
	return nil
}

// Finish queues the run's POST /run/{id}/finish request (ENCRE_04 §7) and
// clears the saved run state: once a run is done there is nothing left to
// resume, only something left to send, and that is [net.Queue.Drain]'s job
// from here — immediately, if the connection allows it, or on a later
// attempt if it does not. Losing the connection at the very end of a run is
// what this makes harmless.
func (s *RunSession) Finish(queue *net.Queue, talismans []engine.TalismanID, revanche [3]bool, cahier bool) error {
	job := net.Job{
		RunID:     s.run.ID,
		Attempts:  s.run.Attempts,
		Talismans: talismans,
		Revanche:  revanche,
		Cahier:    cahier,
	}
	if err := queue.Enqueue(job); err != nil {
		return fmt.Errorf("game: queueing the finish request: %w", err)
	}
	if err := net.ClearRunState(s.store); err != nil {
		return fmt.Errorf("game: clearing the run state after finish: %w", err)
	}
	return nil
}
