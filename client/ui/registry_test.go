package ui_test

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/oioio-space/encre/client/ui"
)

func TestSetBitmapFaceRefusesAFaceMissingAnAccent(t *testing.T) {
	f := gapFace{Face: bitmapfont.Face, absent: map[rune]bool{'é': true}}
	reg := ui.NewRegistry()

	err := reg.SetBitmapFace(ui.Plume, f, "café")
	if err == nil {
		t.Fatal("SetBitmapFace with a missing é = nil error, want one")
	}
	if !strings.ContainsRune(err.Error(), 'é') {
		t.Errorf("error = %q, want it to name the missing rune é", err)
	}
	if _, ok := reg.Face(ui.Plume); ok {
		t.Error("Face(Plume) reports a face after a refused SetBitmapFace")
	}
}

func TestSetBitmapFaceAcceptsTheBitmapPlaceholder(t *testing.T) {
	reg := ui.NewRegistry()

	if err := reg.SetBitmapFace(ui.Plume, bitmapfont.Face, ui.RequiredRunes(ui.AZERTY)); err != nil {
		t.Fatalf("SetBitmapFace with the placeholder: %v", err)
	}
	if _, ok := reg.Face(ui.Plume); !ok {
		t.Error("Face(Plume) reports no face after SetBitmapFace succeeded")
	}
}

func TestFaceReportsFalseForAnUnregisteredRole(t *testing.T) {
	reg := ui.NewRegistry()

	if _, ok := reg.Face(ui.Greffe); ok {
		t.Error("Face(Greffe) on an empty Registry = ok, want false")
	}
}

func TestNewDefaultRegistryCoversAllThreeFacesOfBriefENCRE02Section5(t *testing.T) {
	reg, err := ui.NewDefaultRegistry(ui.RequiredRunes(ui.AZERTY))
	if err != nil {
		t.Fatalf("NewDefaultRegistry: %v", err)
	}

	for _, role := range []ui.FaceRole{ui.Plume, ui.Greffe, ui.Cursive} {
		if _, ok := reg.Face(role); !ok {
			t.Errorf("Face(%v) reports no face from NewDefaultRegistry", role)
		}
	}
}

func TestFaceRoleStringNamesEachRole(t *testing.T) {
	for _, role := range []ui.FaceRole{ui.Plume, ui.Greffe, ui.Cursive} {
		if role.String() == "" {
			t.Errorf("FaceRole(%d).String() is empty", role)
		}
	}
}
