package game_test

import (
	"errors"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/game"
	"github.com/oioio-space/encre/client/ui"
)

var errWantedByTest = errors.New("boom")

// recordingScene counts its own calls, so a test can tell a Stage reached it
// without needing a real window to draw into.
type recordingScene struct {
	updates, draws int
	updateErr      error
}

func (s *recordingScene) Update(*game.Game) error {
	s.updates++
	return s.updateErr
}

func (s *recordingScene) Draw(*ebiten.Image, ui.Screen) {
	s.draws++
}

func TestStageUpdateAndDrawReachTheSceneOfTheCurrentSceneID(t *testing.T) {
	scene := &recordingScene{}
	g := game.NewGame(game.Boot, anim.DefaultJuice())
	stage := game.NewStage(g, map[game.SceneID]game.Scene{game.Boot: scene})

	if err := stage.Update(); err != nil {
		t.Fatalf("stage.Update(): %v", err)
	}
	stage.Draw(nil, ui.Screen{})

	if scene.updates != 1 {
		t.Errorf("scene.updates = %d, want 1", scene.updates)
	}
	if scene.draws != 1 {
		t.Errorf("scene.draws = %d, want 1", scene.draws)
	}
}

func TestStageUpdateReportsAMissingSceneRatherThanPanicking(t *testing.T) {
	g := game.NewGame(game.Boot, anim.DefaultJuice())
	stage := game.NewStage(g, map[game.SceneID]game.Scene{})

	if err := stage.Update(); err == nil {
		t.Fatal("stage.Update() with no scene registered = nil error, want one")
	}
}

func TestStageDrawDoesNothingWhenNoSceneIsRegistered(t *testing.T) {
	g := game.NewGame(game.Boot, anim.DefaultJuice())
	stage := game.NewStage(g, map[game.SceneID]game.Scene{})

	// Must not panic: Update will already have reported the missing scene.
	stage.Draw(nil, ui.Screen{})
}

func TestStageUpdatePropagatesTheSceneError(t *testing.T) {
	wantErr := errWantedByTest
	scene := &recordingScene{updateErr: wantErr}
	g := game.NewGame(game.Boot, anim.DefaultJuice())
	stage := game.NewStage(g, map[game.SceneID]game.Scene{game.Boot: scene})

	if err := stage.Update(); !errors.Is(err, wantErr) {
		t.Errorf("stage.Update() = %v, want %v", err, wantErr)
	}
}
