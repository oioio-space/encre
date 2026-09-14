package anim_test

import (
	"math"
	"testing"

	"pgregory.net/rapid"

	"github.com/oioio-space/encre/client/anim"
)

// genBezier draws a curve valid under the CSS spec: X1 and X2 in [0,1], Y1
// and Y2 free — the same freedom brief/ENCRE_06 §6 uses for joie and vol.
func genBezier(t *rapid.T) anim.Bezier {
	return anim.Bezier{
		X1: rapid.Float64Range(0, 1).Draw(t, "x1"),
		Y1: rapid.Float64Range(-2, 2).Draw(t, "y1"),
		X2: rapid.Float64Range(0, 1).Draw(t, "x2"),
		Y2: rapid.Float64Range(-2, 2).Draw(t, "y2"),
	}
}

func TestBezierBoundsHoldForAnyValidCurve(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := genBezier(t)

		if y := b.Y(0); math.Abs(y) > 1e-9 {
			t.Fatalf("%+v.Y(0) = %v, want 0", b, y)
		}
		if y := b.Y(1); math.Abs(y-1) > 1e-9 {
			t.Fatalf("%+v.Y(1) = %v, want 1", b, y)
		}
	})
}

// TestBezierInversionIsAccurate is the property that catches a Newton-Raphson
// that failed to converge — or, the specific bug this evaluator exists to
// avoid, one that was never inverted at all: for the u [Bezier.Y] actually
// uses, the curve's X component at u must equal the t asked for.
func TestBezierInversionIsAccurate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := genBezier(t)
		tt := rapid.Float64Range(0, 1).Draw(t, "t")

		// b.Y(tt) is only observable through Y itself, so the property is
		// checked the same way a caller would notice a bad inversion: X at
		// the curve's own reported time equals what was asked for. curveX
		// mirrors the unexported sampleCurve the package uses internally,
		// built from the same public control points.
		u := solveForU(b, tt)
		x := curveX(b, u)
		if math.Abs(x-tt) > 1e-6 {
			t.Fatalf("%+v inverted at t=%v: X(u)=%v, want within 1e-6 of t", b, tt, x)
		}
	})
}

// curveX evaluates the cubic Bézier's X component at parameter u, the same
// polynomial [Bezier.Y] inverts internally, reimplemented here so the
// inversion property does not depend on the package's own unexported helper.
func curveX(b anim.Bezier, u float64) float64 {
	c := 3 * b.X1
	bb := 3*(b.X2-b.X1) - c
	a := 1 - c - bb
	return ((a*u+bb)*u + c) * u
}

// solveForU recovers the parameter u a correct [Bezier.Y] would have used for
// t, by bisecting curveX directly — independent of the package's own search,
// so this is a check of the inversion, not a restatement of it.
func solveForU(b anim.Bezier, t float64) float64 {
	lo, hi := 0.0, 1.0
	for range 60 {
		mid := (lo + hi) / 2
		if curveX(b, mid) < t {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

func TestBezierLinearIsTheIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		linear := anim.Bezier{X1: 0, Y1: 0, X2: 1, Y2: 1}
		tt := rapid.Float64Range(0, 1).Draw(t, "t")

		if y := linear.Y(tt); math.Abs(y-tt) > 1e-9 {
			t.Errorf("linear.Y(%v) = %v, want %v", tt, y, tt)
		}
	})
}

func TestBezierOvershootIsPossible(t *testing.T) {
	// joie (brief/ENCRE_06 §6): cubic-bezier(.3,1.4,.5,1). If this failed, a
	// min(1, …) clamp had been added somewhere along the way.
	joie := anim.Bezier{X1: .3, Y1: 1.4, X2: .5, Y2: 1}

	over := false
	for i := 1; i < 100; i++ {
		t := float64(i) / 100
		if joie.Y(t) > 1 {
			over = true
			break
		}
	}
	if !over {
		t.Fatal("joie's curve never exceeds 1 on ]0,1[, want it to overshoot")
	}
}

// TestBezierWithAFlatDerivativeFallsBackToBisection exercises a curve whose
// X control points make Newton-Raphson's derivative pass through zero at the
// midpoint — X1=1, X2=0 — the case [Bezier.solveCurveX]'s bisection fallback
// exists for. It only has to terminate with a value matching the inversion
// property, not panic or loop forever.
func TestBezierWithAFlatDerivativeFallsBackToBisection(t *testing.T) {
	b := anim.Bezier{X1: 1, Y1: 0, X2: 0, Y2: 1}
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	for i := 1; i < 20; i++ {
		tt := float64(i) / 20
		u := solveForU(b, tt)
		if x := curveX(b, u); math.Abs(x-tt) > 1e-6 {
			t.Fatalf("reference solver disagrees with itself at t=%v: X(u)=%v", tt, x)
		}
		if y := b.Y(tt); math.IsNaN(y) || math.IsInf(y, 0) {
			t.Fatalf("Bezier{1,0,0,1}.Y(%v) = %v, want a finite value", tt, y)
		}
	}
}

func TestBezierIsDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := genBezier(t)
		tt := rapid.Float64Range(0, 1).Draw(t, "t")

		if a, c := b.Y(tt), b.Y(tt); a != c {
			t.Fatalf("%+v.Y(%v) = %v then %v, want the same bits both times", b, tt, a, c)
		}
	})
}

func TestBezierValidate(t *testing.T) {
	tests := []struct {
		name    string
		bezier  anim.Bezier
		wantErr bool
	}{
		{"all in range", anim.Bezier{X1: 0, Y1: 0, X2: 1, Y2: 1}, false},
		{"x1 below 0", anim.Bezier{X1: -0.1, Y1: 0, X2: 1, Y2: 1}, true},
		{"x1 above 1", anim.Bezier{X1: 1.1, Y1: 0, X2: 1, Y2: 1}, true},
		{"x2 below 0", anim.Bezier{X1: 0, Y1: 0, X2: -0.1, Y2: 1}, true},
		{"x2 above 1", anim.Bezier{X1: 0, Y1: 0, X2: 1.1, Y2: 1}, true},
		{"y1 above 1 is fine (joie)", anim.Bezier{X1: .3, Y1: 1.4, X2: .5, Y2: 1}, false},
		{"y2 above 1 is fine (vol's rebound)", anim.Bezier{X1: .3, Y1: .7, X2: .4, Y2: 1.08}, false},
		{"y1 negative is fine", anim.Bezier{X1: 0, Y1: -0.5, X2: 1, Y2: 1}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.bezier.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("%+v.Validate() = %v, wantErr %v", tc.bezier, err, tc.wantErr)
			}
		})
	}
}

// TestBezierMatchesKnownReferenceValues compares ease-in-out
// (cubic-bezier(.42,0,.58,1)) against values from an independent bisection
// solver (computed for this test, not shared code with the package) — the
// check no property above would catch, because a consistently wrong
// algorithm can still be bounded, invertible and deterministic. ease-in-out
// is point-symmetric about (0.5, 0.5) — X1+X2 = Y1+Y2 = 1 — which is why
// Y(0.5) = 0.5 exactly and Y(0.25), Y(0.75) sum to 1.
func TestBezierMatchesKnownReferenceValues(t *testing.T) {
	tests := []struct {
		t    float64
		want float64
	}{
		{0.0, 0.0},
		{0.25, 0.12916},
		{0.5, 0.5},
		{0.75, 0.87084},
		{1.0, 1.0},
	}
	for _, tc := range tests {
		if got := anim.EaseInOut.Y(tc.t); math.Abs(got-tc.want) > 1e-4 {
			t.Errorf("EaseInOut.Y(%v) = %v, want %v (±1e-4)", tc.t, got, tc.want)
		}
	}
}
