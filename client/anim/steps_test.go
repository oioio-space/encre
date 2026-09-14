package anim_test

import (
	"encoding/json"
	"testing"

	"github.com/oioio-space/encre/client/anim"
)

func TestSteps1HoldsThenJumps(t *testing.T) {
	s := anim.Steps{Count: 1}

	tests := []struct {
		t    float64
		want float64
	}{
		{0, 0},
		{0.01, 0},
		{0.5, 0},
		{0.99, 0},
		{1, 1},
	}
	for _, tc := range tests {
		if got := s.Y(tc.t); got != tc.want {
			t.Errorf("Steps{1}.Y(%v) = %v, want %v", tc.t, got, tc.want)
		}
	}
}

func TestStepsWithMoreThanOneStep(t *testing.T) {
	s := anim.Steps{Count: 4}

	tests := []struct {
		t    float64
		want float64
	}{
		{0, 0},
		{0.24, 0},
		{0.26, 0.25},
		{0.74, 0.5},
		{0.76, 0.75},
		{1, 1},
	}
	for _, tc := range tests {
		if got := s.Y(tc.t); got != tc.want {
			t.Errorf("Steps{4}.Y(%v) = %v, want %v", tc.t, got, tc.want)
		}
	}
}

func TestCurveResolvesSteps1ByName(t *testing.T) {
	c := anim.NamedCurve(anim.CurveSteps1)

	easing, err := c.Easing()
	if err != nil {
		t.Fatalf("Easing(): %v", err)
	}
	steps, ok := easing.(anim.Steps)
	if !ok {
		t.Fatalf("Easing() = %T, want anim.Steps", easing)
	}
	if steps.Count != 1 {
		t.Errorf("steps.Count = %d, want 1", steps.Count)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestCurveResolvesANamedBezier(t *testing.T) {
	c := anim.NamedCurve(anim.CurveEaseOut)

	easing, err := c.Easing()
	if err != nil {
		t.Fatalf("Easing(): %v", err)
	}
	if _, ok := easing.(anim.Bezier); !ok {
		t.Fatalf("Easing() = %T, want anim.Bezier", easing)
	}
}

func TestCurveEasingRejectsAnUnknownName(t *testing.T) {
	c := anim.NamedCurve("bounce")

	if _, err := c.Easing(); err == nil {
		t.Error(`Easing() with name "bounce" = nil error, want one`)
	}
}

func TestCurveEasingRejectsAMalformedStepsName(t *testing.T) {
	tests := []string{"steps(", "steps(abc)", "steps(0)", "steps(-1)", "steps)1("}
	for _, name := range tests {
		c := anim.NamedCurve(name)
		if _, err := c.Easing(); err == nil {
			t.Errorf("Easing() with name %q = nil error, want one", name)
		}
	}
}

func TestCurveMarshalJSONRoundTripsANamedCurve(t *testing.T) {
	c := anim.NamedCurve(anim.CurveEaseOut)

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(data), `"ease-out"`; got != want {
		t.Errorf("Marshal(%v) = %s, want %s", c, got, want)
	}

	var got anim.Curve
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Name != c.Name {
		t.Errorf("round-tripped Name = %q, want %q", got.Name, c.Name)
	}
}

func TestCurveMarshalJSONRoundTripsABezierCurve(t *testing.T) {
	c := anim.BezierCurve(.3, 1.4, .5, 1)

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(data), `[0.3,1.4,0.5,1]`; got != want {
		t.Errorf("Marshal(%v) = %s, want %s", c, got, want)
	}

	var got anim.Curve
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if *got.Bezier != *c.Bezier {
		t.Errorf("round-tripped Bezier = %+v, want %+v", *got.Bezier, *c.Bezier)
	}
}

func TestCurveValidateBoundsTable(t *testing.T) {
	tests := []struct {
		name    string
		curve   anim.Curve
		wantErr bool
	}{
		{"named ease-out", anim.NamedCurve(anim.CurveEaseOut), false},
		{"steps(1)", anim.NamedCurve(anim.CurveSteps1), false},
		{"bezier x1 in range, y1 overshoots (joie)", anim.BezierCurve(.3, 1.4, .5, 1), false},
		{"bezier x1 out of range", anim.BezierCurve(-0.1, 0, 1, 1), true},
		{"bezier x2 out of range", anim.BezierCurve(0, 0, 1.1, 1), true},
		{"empty curve", anim.Curve{}, true},
		{"unknown name", anim.NamedCurve("bounce"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.curve.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
