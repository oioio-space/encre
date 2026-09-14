package api

import (
	"errors"
	"net/http"
	"slices"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/store"
)

// maxGardeIDs bounds a run/start request's gardeIDs well past
// maxGardeSlots (engine's actual cap, 5): it exists only so a client cannot
// make the handler build an arbitrarily large lookup before BuildDeck ever
// gets to trim it down.
const maxGardeIDs = 10

// runStartRequest is the body of POST /run/start.
type runStartRequest struct {
	GardeIDs []string `json:"gardeIDs"`
}

// runStartResponse is the body of POST /run/start: everything
// brief/ENCRE_04 §7 asks it to carry — the deck, targets, boss and rooms
// (inside Deck), plus the audio URLs and p̂ estimates the deck's own words
// do not carry themselves.
type runStartResponse struct {
	RunID   string               `json:"runID"`
	Deck    engine.Deck          `json:"deck"`
	Rank    int                  `json:"rank"`
	Targets [3]float64           `json:"targets"`
	Levels  map[engine.Color]int `json:"levels"`
	Audio   map[string]string    `json:"audio"`
	PHat    map[string]float64   `json:"pHat"`
}

// handleRunStart refuses to open a run once today's play time is spent, and
// otherwise builds and persists a fresh deck for the child behind the
// session cookie.
func (s *Server) handleRunStart(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[runStartRequest](w, r)
	if !ok {
		return
	}
	if len(req.GardeIDs) > maxGardeIDs {
		writeError(w, http.StatusBadRequest, "trop de mots demandés pour la Garde")
		return
	}

	ctx := r.Context()
	child, err := s.db.ChildByID(ctx, sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}

	remaining, err := s.remainingSeconds(ctx, child)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	if remaining <= 0 {
		writeError(w, http.StatusForbidden, "le temps de jeu du jour est épuisé")
		return
	}

	states, err := s.db.WordStates(ctx, sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	catalog, err := s.wordCatalog(ctx, sess.SubjectID, states)
	if err != nil {
		s.log.Error("loading word catalog", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	now := s.now()
	seed, err := newDeckSeed()
	if err != nil {
		s.log.Error("drawing deck seed", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	runID, err := newRunID()
	if err != nil {
		s.log.Error("generating run id", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	ec := child.Engine()
	garde := filterGoldGarde(req.GardeIDs, states)
	deck := engine.BuildDeck(&ec, catalog.Week, catalog.Known, states, garde, weekOf(now), seed, s.cfg)
	targets := engine.Targets(&ec, deck, states, s.cfg)

	run := &store.Run{
		ID:        runID,
		ChildID:   sess.SubjectID,
		StartedAt: now,
		Rank:      ec.Rank,
		Deck:      deck,
		Targets:   targets,
		FailedAt:  -1,
	}
	if err := s.db.CreateRun(ctx, run); err != nil {
		s.log.Error("creating run", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	writeJSON(w, http.StatusOK, runStartResponse{
		RunID:   runID,
		Deck:    deck,
		Rank:    ec.Rank,
		Targets: targets,
		Levels:  ec.Level,
		Audio:   catalog.Audio,
		PHat:    deckPHat(&ec, deck, states),
	})
}

// deckPHat estimates p̂ (engine.PHat) for every word the deck's manche
// content and Garde carry, under a baseline context (two listens, no boss,
// no blind) — the estimate the card shows before the child has chosen how
// to play the word, not the one that will actually score it.
func deckPHat(c *engine.Child, d engine.Deck, states map[string]*engine.WordState) map[string]float64 {
	out := map[string]float64{}
	ctx := engine.Ctx{Levels: c.Level, Listens: 2, Boss: engine.NoBoss}
	for _, w := range slices.Concat(d.Week, d.Garde) {
		st := states[w.ID]
		if st == nil {
			st = &engine.WordState{}
		}
		out[w.ID] = engine.PHat(c, w, st, ctx)
	}
	return out
}

// roomRequest is the body of POST /run/{id}/room.
type roomRequest struct {
	Index  int           `json:"index"`
	RoomID engine.RoomID `json:"roomID"`
}

// handleRunRoom checks that the chosen room was actually one of the two
// offered at that transition (brief/ENCRE_04 §4 — the server decides, never
// trusts). No Talisman shop pricing exists in [engine.Config] yet (sim.go
// carries the same gap under talismanShopCost's comment), so this endpoint
// validates legality and echoes the choice rather than computing an economy
// effect no Config field backs; wiring the Échoppe/Repos/Encrier rewards
// once that Config exists is left to the ticket that adds it.
func (s *Server) handleRunRoom(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	run, ok := s.ownRun(w, r, sess)
	if !ok {
		return
	}
	req, ok := decodeJSON[roomRequest](w, r)
	if !ok {
		return
	}
	if req.Index < 0 || req.Index >= len(run.Deck.Rooms) {
		writeError(w, http.StatusBadRequest, "index de salle invalide")
		return
	}
	choices := run.Deck.Rooms[req.Index]
	if req.RoomID != choices[0] && req.RoomID != choices[1] {
		writeError(w, http.StatusBadRequest, "cette salle n'est pas proposée à cette transition")
		return
	}
	writeJSON(w, http.StatusOK, roomRequest{Index: req.Index, RoomID: req.RoomID})
}

// maxAttempts bounds a finish request's attempts well past what a run can
// legitimately hold — three manches of engine.Config.WordsPerManche words,
// plus room for a Revanche replaying one manche and Rencontres — so a client
// cannot make the handler build a many-thousand-entry slice before Replay
// ever gets to see it.
const maxAttempts = 200

// maxTalismans bounds a finish request's talismans list the same way; the
// game carries twenty at most (engine.TalismanID's own range).
const maxTalismans = 20

// finishRequest is the body of POST /run/{id}/finish.
type finishRequest struct {
	Attempts  []engine.Attempt    `json:"attempts"`
	Talismans []engine.TalismanID `json:"talismans"`
	Revanche  [3]bool             `json:"revanche"`
	Cahier    bool                `json:"cahier"`
}

// finishResponse is the body of POST /run/{id}/finish.
type finishResponse struct {
	Outcome engine.Outcome `json:"outcome"`
	Events  []engine.Event `json:"events"`
}

// handleRunFinish recomputes the run's outcome from its attempts
// ([engine.Replay], never the client's own score) and, exactly once per run,
// writes it into the child ([engine.Apply]).
//
// The ordering matters (see the package doc, "Idempotence, end to end"):
// Replay runs first because it is pure and can reject a malformed request
// for free; [store.Store.MarkApplied] runs next and is the one atomic
// decision that determines whether Apply's writes happen at all; Apply
// itself runs last and cannot observe a repeat, because MarkApplied already
// refused one.
func (s *Server) handleRunFinish(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	run, ok := s.ownRun(w, r, sess)
	if !ok {
		return
	}
	req, ok := decodeJSON[finishRequest](w, r)
	if !ok {
		return
	}
	if len(req.Attempts) > maxAttempts {
		writeError(w, http.StatusBadRequest, "trop de tentatives")
		return
	}
	if len(req.Talismans) > maxTalismans {
		writeError(w, http.StatusBadRequest, "trop de talismans")
		return
	}

	ctx := r.Context()
	child, err := s.db.ChildByID(ctx, run.ChildID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}
	states, err := s.db.WordStates(ctx, run.ChildID)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}

	ec := child.Engine()
	engineRun := engine.Run{
		ID:        run.ID,
		ChildID:   run.ChildID,
		Deck:      run.Deck,
		Rank:      run.Rank,
		Targets:   run.Targets,
		Talismans: req.Talismans,
		Rooms:     run.Rooms,
		// server/store does not persist the levels a run was started under
		// (unlike Rank, which it does — see store.Run.Rank's doc comment);
		// the child's current level stands in. The two can only differ if
		// another run finished and levelled a Couleur in the narrow window
		// between this run's start and its finish, and this is reported as
		// a known gap alongside encre-qpx.3 rather than worked around here.
		Levels:   ec.Level,
		Attempts: req.Attempts,
		Revanche: req.Revanche,
		Cahier:   req.Cahier,
	}

	out, err := engine.Replay(engineRun, states, s.cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, "run invalide")
		return
	}

	if err := s.db.MarkApplied(ctx, run.ID); err != nil {
		if errors.Is(err, engine.ErrAlreadyApplied) {
			writeError(w, http.StatusConflict, "cette run a déjà été appliquée")
			return
		}
		s.writeStoreError(w, err, "run introuvable")
		return
	}

	now := s.now()
	events, err := engine.Apply(&ec, engineRun, out, states, dayOf(now), weekOf(now), s.cfg)
	if err != nil {
		s.log.Error("applying run", "run", run.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	child.SetEngine(ec)

	if err := s.db.SaveChild(ctx, child); err != nil {
		s.log.Error("saving child after run", "run", run.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	if err := s.db.SaveWordStates(ctx, run.ChildID, states); err != nil {
		s.log.Error("saving word states after run", "run", run.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	writeJSON(w, http.StatusOK, finishResponse{Outcome: out, Events: events})
}

// maxHeartbeatSeconds bounds a single heartbeat call well past the 30-second
// cadence brief/ENCRE_04 §7 asks the client for, so a claimed heartbeat
// cannot credit more play time than a client could plausibly have earned
// since the last one.
const maxHeartbeatSeconds = 120

// heartbeatRequest is the body of POST /run/{id}/heartbeat.
type heartbeatRequest struct {
	Seconds int `json:"seconds"`
}

// heartbeatResponse is the body of POST /run/{id}/heartbeat.
type heartbeatResponse struct {
	RemainingSeconds int `json:"remainingSeconds"`
}

// handleRunHeartbeat records play time toward today's budget, so
// [Server.remainingSeconds] — and the refusal handleRunStart makes from it —
// sees a run that is actually in progress.
func (s *Server) handleRunHeartbeat(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	if _, ok := s.ownRun(w, r, sess); !ok {
		return
	}
	req, ok := decodeJSON[heartbeatRequest](w, r)
	if !ok {
		return
	}
	if req.Seconds <= 0 || req.Seconds > maxHeartbeatSeconds {
		writeError(w, http.StatusBadRequest, "seconds hors bornes")
		return
	}

	ctx := r.Context()
	now := s.now()
	if err := s.db.AddPlayTime(ctx, sess.SubjectID, dayOf(now), req.Seconds); err != nil {
		s.writeStoreError(w, err, "")
		return
	}

	child, err := s.db.ChildByID(ctx, sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}
	remaining, err := s.remainingSeconds(ctx, child)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, heartbeatResponse{RemainingSeconds: remaining})
}
