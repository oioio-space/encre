package game

import (
	"errors"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/ui"
)

// SceneID names one screen of brief/ENCRE_04 §10.
type SceneID int

// The scenes of brief/ENCRE_04 §10: the main loop from Boot to Atelier and
// back, the four panels the Atelier opens, and the Revanche branch off Boss.
const (
	Boot SceneID = iota
	Atelier
	Garde
	Run
	Salle
	Boss
	Revanche
	Enluminure
	Cahier
	Recap
	Bestiaire
	Fioles
	Exploits
	FinDeTemps
)

// String names the scene, for diagnostics and error messages.
func (s SceneID) String() string {
	switch s {
	case Boot:
		return "Boot"
	case Atelier:
		return "Atelier"
	case Garde:
		return "Garde"
	case Run:
		return "Run"
	case Salle:
		return "Salle"
	case Boss:
		return "Boss"
	case Revanche:
		return "Revanche"
	case Enluminure:
		return "Enluminure"
	case Cahier:
		return "Cahier"
	case Recap:
		return "Récap"
	case Bestiaire:
		return "Bestiaire"
	case Fioles:
		return "Fioles"
	case Exploits:
		return "Exploits"
	case FinDeTemps:
		return "FinDeTemps"
	default:
		return fmt.Sprintf("SceneID(%d)", int(s))
	}
}

// transitions is the map of brief/ENCRE_04 §10, forward edges only: a scene
// reached by Push (the four Atelier panels, Revanche) returns to where it came
// from by Pop rather than by an edge back, and a scene reached by Replace (the
// run's own sequence) has none — the game does not step backward through a run.
//
// Every scene a run can be playing in also reaches Recap directly. The diagram
// of ENCRE_04 §10 draws only the winning line, but ENCRE_01 §3 is explicit that
// a missed target ends the run there and then, and losing is the half that
// happens more often. A child must never be left mid-run with no way out.
var transitions = map[SceneID][]SceneID{
	Boot:       {Atelier},
	Atelier:    {Garde, Bestiaire, Fioles, Exploits, FinDeTemps},
	Garde:      {Run},
	Run:        {Salle, Recap},
	Salle:      {Run, Boss, Recap},
	Boss:       {Enluminure, Revanche, Recap},
	Enluminure: {Cahier},
	Cahier:     {Recap},
	Recap:      {Atelier},
}

// ErrIllegalTransition reports a move the map of scenes of brief/ENCRE_04 §10
// does not allow, or a Pop with nothing left to return to.
var ErrIllegalTransition = errors.New("game: illegal scene transition")

// Director is the pure state machine behind the scene graph: a stack of
// SceneID, with no dependency on Ebitengine or on what a scene draws, so the
// map of brief/ENCRE_04 §10 can be tested exhaustively without a window. Stage
// pairs it with the scenes that actually render.
type Director struct {
	stack []SceneID
}

// NewDirector returns a Director starting at initial.
func NewDirector(initial SceneID) *Director {
	return &Director{stack: []SceneID{initial}}
}

// Current returns the scene on top of the stack.
func (d *Director) Current() SceneID {
	return d.stack[len(d.stack)-1]
}

func (d *Director) canGo(target SceneID) bool {
	for _, next := range transitions[d.Current()] {
		if next == target {
			return true
		}
	}
	return false
}

// Replace moves the current scene to target, in place: the stack does not
// grow, so nothing is left to Pop back to. This is the run's own sequence,
// Run → Salle → … → Récap → Atelier, which never steps backward.
func (d *Director) Replace(target SceneID) error {
	if !d.canGo(target) {
		return fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, d.Current(), target)
	}
	d.stack[len(d.stack)-1] = target
	return nil
}

// Push opens target above the current scene, to be closed by Pop: the four
// panels the Atelier opens, and the Revanche Boss offers.
func (d *Director) Push(target SceneID) error {
	if !d.canGo(target) {
		return fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, d.Current(), target)
	}
	d.stack = append(d.stack, target)
	return nil
}

// Pop closes the current scene and returns to the one Push opened it from. It
// refuses to pop the last scene on the stack: there is nothing to return to.
func (d *Director) Pop() error {
	if len(d.stack) < 2 {
		return fmt.Errorf("%w: cannot pop the last scene (%s)", ErrIllegalTransition, d.Current())
	}
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}

// Game is what a Scene receives on every Update: the Director, so a scene can
// move the game to the next one, and the Juice every scene animates by, so
// none of them reads brief/ENCRE_02 §12's timings through a different path.
type Game struct {
	*Director
	// Juice is the timings a scene animates by (brief/ENCRE_02 §12).
	Juice anim.Juice
}

// NewGame returns a Game starting at initial, animating by juice.
func NewGame(initial SceneID, juice anim.Juice) *Game {
	return &Game{Director: NewDirector(initial), Juice: juice}
}

// Scene is one screen of the game: it updates its own state and draws itself
// into dst, laid out by screen. Enter and Leave are not part of the interface
// because nothing built on T23 needs them yet; a scene that does can be given
// them without touching this one.
type Scene interface {
	Update(g *Game) error
	Draw(dst *ebiten.Image, screen ui.Screen)
}

// Stage runs a Game against the concrete Scenes registered for its SceneIDs.
// It is the only piece here that knows about Ebitengine; the state machine it
// drives is Director, tested on its own.
type Stage struct {
	*Game
	scenes map[SceneID]Scene
}

// NewStage returns a Stage running g against scenes, keyed by the SceneID each
// one draws.
func NewStage(g *Game, scenes map[SceneID]Scene) *Stage {
	return &Stage{Game: g, scenes: scenes}
}

// Update runs the current scene's Update, or reports that none is registered
// for it — a programming error, not a player-reachable one, but one that must
// not pass silently.
func (s *Stage) Update() error {
	scene, ok := s.scenes[s.Current()]
	if !ok {
		return fmt.Errorf("game: no scene registered for %s", s.Current())
	}
	return scene.Update(s.Game)
}

// Draw draws the current scene into dst, laid out by screen. It does nothing
// when no scene is registered for the current SceneID: Update will already
// have reported that error this frame.
func (s *Stage) Draw(dst *ebiten.Image, screen ui.Screen) {
	if scene, ok := s.scenes[s.Current()]; ok {
		scene.Draw(dst, screen)
	}
}
