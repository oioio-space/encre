package net

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oioio-space/encre/engine"
)

// runStateKey is the Store key the run in progress is saved under. There is
// at most one: a child plays one run at a time.
const runStateKey = "run"

// RunStateVersion is the current shape of a saved [RunState]. [LoadRunState]
// refuses a save carrying any other version rather than risk decoding it
// into a struct it no longer matches — see [LoadRunState]'s doc comment.
const RunStateVersion = 1

// RunState is the run in progress, saved after every word (ENCRE_04 §10) so
// closing the tab, or losing the connection, loses nothing: the next launch
// resumes engine.Run exactly as it stood, attempts included.
type RunState struct {
	Version int
	SavedAt time.Time
	Run     engine.Run
}

// SaveRunState saves run as the RunState to resume from, stamped with now.
// Call it after every attempt is appended to run.Attempts — the "sauvegardée
// à chaque mot" ENCRE_04 §10 asks for is this call, made once per word.
func SaveRunState(s Store, run engine.Run, now time.Time) error {
	data, err := json.Marshal(RunState{Version: RunStateVersion, SavedAt: now, Run: run})
	if err != nil {
		return fmt.Errorf("net: encoding the run state: %w", err)
	}
	if err := s.Save(runStateKey, data); err != nil {
		return fmt.Errorf("net: saving the run state: %w", err)
	}
	return nil
}

// LoadRunState returns the run saved by [SaveRunState] and true, or a zero
// RunState and false when there is nothing to resume.
//
// "Nothing to resume" covers three cases the same way, on purpose: no save
// exists yet, the save is not valid JSON (a corrupted write, a browser that
// crashed mid-save), or it carries a Version this build does not recognise
// (an older or newer release than the one that wrote it). None of the three
// is treated as an error a caller must handle — a game that panicked or got
// stuck on a dead screen because of one bad save would be worse for the child
// than simply starting a fresh run, which is what "false" tells the caller to
// do.
func LoadRunState(s Store) (RunState, bool) {
	data, err := s.Load(runStateKey)
	if err != nil {
		return RunState{}, false
	}
	var rs RunState
	if err := json.Unmarshal(data, &rs); err != nil {
		return RunState{}, false
	}
	if rs.Version != RunStateVersion {
		return RunState{}, false
	}
	return rs, true
}

// ClearRunState removes the saved run, once it has finished (ENCRE_04 §7's
// POST /run/{id}/finish) and there is nothing left to resume.
func ClearRunState(s Store) error {
	if err := s.Clear(runStateKey); err != nil {
		return fmt.Errorf("net: clearing the run state: %w", err)
	}
	return nil
}
