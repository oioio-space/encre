package anim_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/encre/client/anim"
)

// bezierCmp lets cmp.Diff compare unexported Bezier fields, and dereference
// the *Bezier pointer Curve carries so two curves naming the same control
// points compare equal regardless of pointer identity.
var bezierCmp = cmp.Comparer(func(a, b anim.Bezier) bool { return a == b })

func TestDefaultJuiceMatchesBriefENCRE06Section6(t *testing.T) {
	j := anim.DefaultJuice()

	cases := []struct {
		name string
		got  any
		want any
	}{
		// brief/ENCRE_02 §12 only: no ENCRE_06 §6 entry, no curve.
		{"HitstopTrap", j.HitstopTrap.Duration(), 80 * time.Millisecond},
		{"ScoreSilence", j.ScoreSilence.Duration(), 500 * time.Millisecond},
		{"HitstopLegendary", j.HitstopLegendary.Duration(), 150 * time.Millisecond},
		{"SquashScale", j.SquashScale, 0.85},
		{"SquashDuration", j.SquashDuration.Duration(), 60 * time.Millisecond},
		{"ReboundScale", j.ReboundScale, 1.08},
		{"ReturnDuration", j.ReturnDuration.Duration(), 180 * time.Millisecond},
		{"BossLastLetterSlowFactor", j.BossLastLetterSlowFactor, 0.5},
		{"BossLastLetterSlowDuration", j.BossLastLetterSlowDuration.Duration(), 400 * time.Millisecond},
		{"EnluminureDuration", j.EnluminureDuration.Duration(), 6 * time.Second},
		{"CounterMinDuration", j.CounterMinDuration.Duration(), 300 * time.Millisecond},
		{"CounterMaxDuration", j.CounterMaxDuration.Duration(), 3 * time.Second},
		{"CounterArrivalScale", j.CounterArrivalScale, 1.18},
		{"DropletPerTokens", j.DropletPerTokens, 10},
		{"ShakeAmplitude", j.ShakeAmplitude, 2.0},
		{"ShakeMaxAmplitude", j.ShakeMaxAmplitude, 12.0},

		// The four values brief/ENCRE_06 §6 corrects over ENCRE_02 §12.
		{"Tremble.Duration", j.Tremble.Duration.Duration(), 500 * time.Millisecond},
		{"Tremble.Curve", j.Tremble.Curve, anim.NamedCurve(anim.CurveEaseOut)},
		{"Droplet.Duration (jetons)", j.Droplet.Duration.Duration(), 1800 * time.Millisecond},
		{"Droplet.Curve (jetons)", j.Droplet.Curve, anim.NamedCurve(anim.CurveEaseIn)},
		{"DropletStagger (jetons)", j.DropletStagger.Duration(), 100 * time.Millisecond},
		{"BrouillonShowDuration", j.BrouillonShowDuration.Duration(), 1 * time.Second},
		{"CardIdle.Duration (flotte)", j.CardIdle.Duration.Duration(), 3400 * time.Millisecond},
		{"CardIdle.Curve (flotte)", j.CardIdle.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"CardIdleAmplitude (flotte)", j.CardIdleAmplitude, 3.0},
		{"CardFlip.Duration (retournement)", j.CardFlip.Duration.Duration(), 300 * time.Millisecond},
		{"CardFlip.Curve (retournement)", j.CardFlip.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"CardFlipMinScaleX (retournement)", j.CardFlipMinScaleX, 0.06},
		{"CardFlipEdgeColor (retournement)", j.CardFlipEdgeColor, "#6E5738"},
		{"Bave.Duration", j.Bave.Duration.Duration(), 120 * time.Millisecond},
		{"Bave.Curve", j.Bave.Curve, anim.NamedCurve(anim.CurveEaseOut)},

		// Not a conflict: 1.8s is the storyboard's fixed reference value,
		// kept alongside the log(score) bounds above, not instead of them.
		{"CounterStoryboard.Duration (compteur)", j.CounterStoryboard.Duration.Duration(), 1800 * time.Millisecond},
		{"CounterStoryboard.Curve (compteur)", j.CounterStoryboard.Curve, anim.NamedCurve(anim.CurveEaseOut)},

		// Unchanged between the two sections, now carrying §6's curve.
		{"Sechage.Duration", j.Sechage.Duration.Duration(), 200 * time.Millisecond},
		{"Sechage.Curve", j.Sechage.Curve, anim.NamedCurve(anim.CurveEaseIn)},

		// The animations of ENCRE_06 §6 with no ENCRE_02 §12 counterpart.
		{"Respire.Duration", j.Respire.Duration.Duration(), 3200 * time.Millisecond},
		{"Respire.Curve", j.Respire.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"Cligne.Duration", j.Cligne.Duration.Duration(), 4600 * time.Millisecond},
		{"Cligne.Curve", j.Cligne.Curve, anim.NamedCurve(anim.CurveSteps1)},
		{"Joie.Duration", j.Joie.Duration.Duration(), 900 * time.Millisecond},
		{"Joie.Curve", j.Joie.Curve, anim.BezierCurve(.3, 1.4, .5, 1)},
		{"Etiquette.Duration", j.Etiquette.Duration.Duration(), 1800 * time.Millisecond},
		{"Etiquette.Curve", j.Etiquette.Curve, anim.NamedCurve(anim.CurveEaseOut)},
		{"Craque.Duration", j.Craque.Duration.Duration(), 1800 * time.Millisecond},
		{"Craque.Curve", j.Craque.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"VacilleRepos.Duration", j.VacilleRepos.Duration.Duration(), 1200 * time.Millisecond},
		{"VacilleRepos.Curve", j.VacilleRepos.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"VacilleHautCombo.Duration", j.VacilleHautCombo.Duration.Duration(), 700 * time.Millisecond},
		{"VacilleHautCombo.Curve", j.VacilleHautCombo.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"Sautille.Duration", j.Sautille.Duration.Duration(), 1200 * time.Millisecond},
		{"Sautille.Curve", j.Sautille.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"SautilleStagger", j.SautilleStagger.Duration(), 140 * time.Millisecond},
		{"PulseValide.Duration", j.PulseValide.Duration.Duration(), 1400 * time.Millisecond},
		{"PulseValide.Curve", j.PulseValide.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
		{"Vol.Duration", j.Vol.Duration.Duration(), 400 * time.Millisecond},
		{"Vol.Curve", j.Vol.Curve, anim.BezierCurve(.3, .7, .4, 1)},
		{"Encre.Duration", j.Encre.Duration.Duration(), 40 * time.Second},
		{"Encre.Curve", j.Encre.Curve, anim.NamedCurve(anim.CurveEaseInOut)},
	}
	for _, c := range cases {
		if diff := cmp.Diff(c.want, c.got, bezierCmp); diff != "" {
			t.Errorf("%s: mismatch (-want +got):\n%s", c.name, diff)
		}
	}
}

// TestJuiceJSONFieldIsCheckedAgainstItsBrief013 guards the guard above: if
// this failed to notice a changed value, the table it drives would be
// worthless. It flips one field DefaultJuice sets and checks the comparison
// this file relies on actually reports it.
func TestDefaultJuiceComparisonCatchesAChangedValue(t *testing.T) {
	j := anim.DefaultJuice()
	tampered := j.Tremble.Duration.Duration()
	j.Tremble.Duration = anim.MillisOf(tampered + time.Millisecond)

	if diff := cmp.Diff(anim.DefaultJuice().Tremble, j.Tremble, bezierCmp); diff == "" {
		t.Fatal("cmp.Diff did not notice a tampered Tremble.Duration")
	}
}

func TestDefaultJuiceValidates(t *testing.T) {
	if err := anim.DefaultJuice().Validate(); err != nil {
		t.Errorf("DefaultJuice().Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsAZeroScalarField(t *testing.T) {
	j := anim.DefaultJuice()
	j.EnluminureDuration = 0

	if err := j.Validate(); err == nil {
		t.Error("Validate() with EnluminureDuration zeroed = nil, want an error")
	}
}

func TestValidateRejectsAnAnimWithAZeroDuration(t *testing.T) {
	j := anim.DefaultJuice()
	j.Tremble.Duration = 0

	if err := j.Validate(); err == nil {
		t.Error("Validate() with Tremble.Duration zeroed = nil, want an error")
	}
}

func TestValidateRejectsACurveWithNeitherNameNorBezier(t *testing.T) {
	j := anim.DefaultJuice()
	j.Vol.Curve = anim.Curve{}

	if err := j.Validate(); err == nil {
		t.Error("Validate() with Vol.Curve empty = nil, want an error")
	}
}

func TestValidateRejectsACurveWithBothNameAndBezier(t *testing.T) {
	j := anim.DefaultJuice()
	j.Vol.Curve = anim.BezierCurve(.3, .7, .4, 1)
	j.Vol.Curve.Name = anim.CurveEaseOut

	if err := j.Validate(); err == nil {
		t.Error("Validate() with both Name and Bezier set = nil, want an error")
	}
}

func TestValidateRejectsAnUnknownCurveName(t *testing.T) {
	j := anim.DefaultJuice()
	j.Vol.Curve = anim.NamedCurve("bounce")

	if err := j.Validate(); err == nil {
		t.Error(`Validate() with Vol.Curve named "bounce" = nil, want an error`)
	}
}

func TestLoadJuicePartialJSONOnlyOverridesWhatItNames(t *testing.T) {
	j, err := anim.LoadJuice([]byte(`{"hitstop_trap_ms": 999}`))
	if err != nil {
		t.Fatalf("LoadJuice: %v", err)
	}

	want := anim.DefaultJuice()
	want.HitstopTrap = anim.MillisOf(999 * time.Millisecond)
	if diff := cmp.Diff(want, j, bezierCmp); diff != "" {
		t.Errorf("LoadJuice with a partial document: mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadJuicePartialJSONOverridesOneAnimWithoutTouchingItsSiblings(t *testing.T) {
	j, err := anim.LoadJuice([]byte(`{"vol": {"duration_ms": 999, "curve": "ease-in"}}`))
	if err != nil {
		t.Fatalf("LoadJuice: %v", err)
	}

	want := anim.DefaultJuice()
	want.Vol = anim.Anim{Duration: 999, Curve: anim.NamedCurve(anim.CurveEaseIn)}
	if diff := cmp.Diff(want, j, bezierCmp); diff != "" {
		t.Errorf("LoadJuice naming only vol: mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadJuiceAcceptsABezierGivenAsFourNumbers(t *testing.T) {
	j, err := anim.LoadJuice([]byte(`{"joie": {"duration_ms": 900, "curve": [0.3, 1.4, 0.5, 1]}}`))
	if err != nil {
		t.Fatalf("LoadJuice: %v", err)
	}

	want := anim.BezierCurve(.3, 1.4, .5, 1)
	if diff := cmp.Diff(want, j.Joie.Curve, bezierCmp); diff != "" {
		t.Errorf("Joie.Curve: mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadJuiceInvalidJSONReturnsAnErrorAndTheDefaultIsUnaffected(t *testing.T) {
	before := anim.DefaultJuice()

	_, err := anim.LoadJuice([]byte(`{not json`))
	if err == nil {
		t.Fatal("LoadJuice on invalid JSON = nil error, want one")
	}
	if diff := cmp.Diff(before, anim.DefaultJuice(), bezierCmp); diff != "" {
		t.Errorf("DefaultJuice() changed after a failed LoadJuice: mismatch (-before +after):\n%s", diff)
	}
}

func TestLoadJuiceRejectsADocumentThatZeroesALoadBearingField(t *testing.T) {
	_, err := anim.LoadJuice([]byte(`{"enluminure_duration_ms": 0}`))
	if err == nil {
		t.Fatal("LoadJuice zeroing enluminure_duration_ms = nil error, want one")
	}
}

func TestLoadJuiceRejectsADocumentThatZeroesAnAnimDuration(t *testing.T) {
	_, err := anim.LoadJuice([]byte(`{"tremble": {"duration_ms": 0, "curve": "ease-out"}}`))
	if err == nil {
		t.Fatal("LoadJuice zeroing tremble.duration_ms = nil error, want one")
	}
}

func TestLoadJuiceRejectsAMalformedCurve(t *testing.T) {
	_, err := anim.LoadJuice([]byte(`{"vol": {"duration_ms": 400, "curve": [0.3, 0.7, 0.4]}}`))
	if err == nil {
		t.Fatal("LoadJuice with a 3-number bezier = nil error, want one")
	}
}
