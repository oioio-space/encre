package assets_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/encre/client/anim"
	"github.com/oioio-space/encre/client/assets"
)

func TestJuiceJSONMatchesTheGoDefaults(t *testing.T) {
	got, err := anim.LoadJuice(assets.JuiceJSON)
	if err != nil {
		t.Fatalf("LoadJuice(assets.JuiceJSON): %v", err)
	}
	if diff := cmp.Diff(anim.DefaultJuice(), got); diff != "" {
		t.Errorf("LoadJuice(assets.JuiceJSON) vs anim.DefaultJuice(): mismatch (-want +got):\n%s", diff)
	}
}

// TestJuiceJSONComparisonCatchesAChangedValue guards the test above: if a
// value drifted between juice.json and DefaultJuice, this is the mechanism
// that must notice — the only thing standing between the brief and the
// render (coordinator review, 2026-09-13).
func TestJuiceJSONComparisonCatchesAChangedValue(t *testing.T) {
	got, err := anim.LoadJuice(assets.JuiceJSON)
	if err != nil {
		t.Fatalf("LoadJuice(assets.JuiceJSON): %v", err)
	}
	want := anim.DefaultJuice()
	want.Bave.Duration = anim.MillisOf(want.Bave.Duration.Duration() + time.Millisecond)

	if diff := cmp.Diff(want, got); diff == "" {
		t.Fatal("cmp.Diff did not notice a tampered Bave.Duration")
	}
}
