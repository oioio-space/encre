// Package anim holds the timings and curves of the game's motion — brief/
// ENCRE_06 §6's table of `@keyframes` (the shipped storyboards), reconciled
// with brief/ENCRE_02 §12 (the intent it was written from) where the two
// disagree: ENCRE_06 §2 already overrides ENCRE_02 on the theme and the
// keyboard for the same reason — §6's numbers are what a developer can watch
// loop in the delivered boards, §12's were the plan before they existed.
//
// This package carries the numbers and the curve evaluator, and nothing that
// sequences them: [Bezier.Y] and [Steps.Y] compute one eased value for one
// t, but nothing here owns a clock or a tween. That is because juice.json
// has to be reloadable at runtime (see [Reload]) — its curves are
// serialisable data, not compiled closures — and because no evaluated
// timeline exists yet for the same numbers to disagree about: that arrives
// with the run screen.
package anim

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Millis is a duration expressed in whole milliseconds in JSON, the unit
// brief/ENCRE_02 §12 and ENCRE_06 §6 are both written in. A hand-edited
// juice.json therefore reads "80", not "80000000".
type Millis int64

// Duration converts m to a [time.Duration].
func (m Millis) Duration() time.Duration { return time.Duration(m) * time.Millisecond }

// MillisOf converts d to a [Millis], truncating anything finer than a
// millisecond.
func MillisOf(d time.Duration) Millis { return Millis(d / time.Millisecond) }

// The CSS easing keywords brief/ENCRE_06 §6 names its curves with, and the
// keys [Curve.Easing] resolves by name. CurveSteps1 is the one step function
// the brief uses (`cligne`); a curve named "steps(N)" for another N resolves
// too (see [Curve.Easing]), even though N is always 1 today.
const (
	CurveEase      = "ease"
	CurveEaseIn    = "ease-in"
	CurveEaseOut   = "ease-out"
	CurveEaseInOut = "ease-in-out"
	CurveLinear    = "linear"
	CurveSteps1    = "steps(1)"
)

// The named cubic-bezier curves of the CSS Easing Functions spec, the control
// points [CurveEase] through [CurveLinear] resolve to.
var (
	Ease      = Bezier{X1: 0.25, Y1: 0.1, X2: 0.25, Y2: 1}
	EaseIn    = Bezier{X1: 0.42, Y1: 0, X2: 1, Y2: 1}
	EaseOut   = Bezier{X1: 0, Y1: 0, X2: 0.58, Y2: 1}
	EaseInOut = Bezier{X1: 0.42, Y1: 0, X2: 0.58, Y2: 1}
	Linear    = Bezier{X1: 0, Y1: 0, X2: 1, Y2: 1}
)

// namedBeziers resolves a CSS easing keyword to its control points.
var namedBeziers = map[string]Bezier{
	CurveEase:      Ease,
	CurveEaseIn:    EaseIn,
	CurveEaseOut:   EaseOut,
	CurveEaseInOut: EaseInOut,
	CurveLinear:    Linear,
}

// Easing evaluates an eased value for t in [0,1]. [Bezier] and [Steps] both
// implement it.
type Easing interface {
	Y(t float64) float64
}

// bezierEpsilon bounds both the Newton-Raphson and bisection searches
// [Bezier.solveCurveX] runs — small enough that a curve whose X and Y
// components happen to be the same polynomial (the "linear" easing) returns
// Y(t) == t to within 1e-9, large enough that the search terminates.
const bezierEpsilon = 1e-11

// Bezier is a CSS cubic-bezier(x1,y1,x2,y2) easing curve: the cubic Bézier
// curve through P0=(0,0), P1=(X1,Y1), P2=(X2,Y2), P3=(1,1), parameterised by
// t in [0,1].
//
// X1 and X2 are the two control points' abscissas. The CSS spec requires
// both in [0,1], so the curve is a function of time — [Bezier.Validate]
// enforces it. Y1 and Y2 have no such bound: `joie` (brief/ENCRE_06 §6)
// overshoots to 1.4, and `vol`'s rebound past 108% needs the same freedom, so
// Y is never clamped here.
type Bezier struct {
	X1, Y1, X2, Y2 float64
}

// Validate reports whether b's control points keep X1 and X2 in [0,1], the
// CSS spec's requirement for the curve to be a function of time. Y1 and Y2
// are never rejected: overshoot past 1 is a deliberate feature of some of
// brief/ENCRE_06 §6's curves, not a mistake to catch.
func (b Bezier) Validate() error {
	if b.X1 < 0 || b.X1 > 1 {
		return fmt.Errorf("bezier: x1 = %v, want in [0,1]", b.X1)
	}
	if b.X2 < 0 || b.X2 > 1 {
		return fmt.Errorf("bezier: x2 = %v, want in [0,1]", b.X2)
	}
	return nil
}

// Y returns the eased value at time t, t in [0,1], clamped at the ends.
//
// t is an abscissa in time, not the Bézier curve's own parameter: this finds
// the parameter u with X(u) = t by inverting the curve's X component — the
// step a naive port easily drops, evaluating Y at t directly instead — and
// only then evaluates Y(u). The inversion is WebKit's UnitBezier algorithm,
// also implemented in Gaëtan Renaudeau's bezier-easing
// (https://github.com/gre/bezier-easing): Newton-Raphson first, falling back
// to bisection when the derivative is too small to converge.
func (b Bezier) Y(t float64) float64 {
	switch {
	case t <= 0:
		return 0
	case t >= 1:
		return 1
	}
	u := b.solveCurveX(t)
	return sampleCurve(b.Y1, b.Y2, u)
}

// sampleCurve evaluates, at parameter u, the cubic Bézier component whose
// control points are 0, p1, p2, 1 — the shared shape of both X(u) and Y(u),
// since P0 and P3 are always (0,0) and (1,1) for a CSS easing curve.
func sampleCurve(p1, p2, u float64) float64 {
	c := 3 * p1
	b := 3*(p2-p1) - c
	a := 1 - c - b
	return ((a*u+b)*u + c) * u
}

// sampleCurveDerivative is the derivative of [sampleCurve] with respect to u,
// which Newton-Raphson divides by to step toward the root.
func sampleCurveDerivative(p1, p2, u float64) float64 {
	c := 3 * p1
	b := 3*(p2-p1) - c
	a := 1 - c - b
	return (3*a*u+2*b)*u + c
}

// solveCurveX finds u such that sampleCurve(X1, X2, u) == x, for x in (0,1).
func (b Bezier) solveCurveX(x float64) float64 {
	// Newton-Raphson: a handful of iterations converges to bezierEpsilon for
	// every curve the CSS spec allows (X1, X2 in [0,1]), since X(u) is then
	// monotonic in u.
	u := x
	for range 12 {
		fx := sampleCurve(b.X1, b.X2, u) - x
		if math.Abs(fx) < bezierEpsilon {
			return u
		}
		dx := sampleCurveDerivative(b.X1, b.X2, u)
		if math.Abs(dx) < bezierEpsilon {
			break
		}
		u -= fx / dx
	}

	// Bisection: the fallback for the derivative going flat, which a purely
	// Newton-Raphson port has no recovery from.
	lo, hi := 0.0, 1.0
	u = x
	if u < lo {
		return lo
	}
	if u > hi {
		return hi
	}
	for hi-lo > bezierEpsilon {
		fx := sampleCurve(b.X1, b.X2, u)
		if math.Abs(fx-x) < bezierEpsilon {
			return u
		}
		if x > fx {
			lo = u
		} else {
			hi = u
		}
		u = (lo + hi) / 2
	}
	return u
}

// Steps is a CSS step(n) easing: held at each step's value, then a jump at
// the step boundary — jump-end semantics, the CSS default. `cligne` (brief/
// ENCRE_06 §6) is the one animation that uses one: steps(1), held for the
// whole cycle bar the boundary itself.
//
// It is its own type, evaluated on its own, rather than a special case of
// [Bezier]: a step function is not a cubic curve, and folding it into
// [Bezier.Y] would make that method lie about what it computes.
type Steps struct {
	// Count is the number of steps, n in steps(n). Positive.
	Count int
}

// Y returns the step function's value at t, t in [0,1]: floor(t·Count)/Count,
// snapping to 1 at t == 1.
func (s Steps) Y(t float64) float64 {
	switch {
	case t <= 0:
		return 0
	case t >= 1:
		return 1
	default:
		return math.Floor(t*float64(s.Count)) / float64(s.Count)
	}
}

// parseSteps parses a "steps(N)" curve name, reporting whether name was one.
func parseSteps(name string) (Steps, bool) {
	inner, ok := strings.CutPrefix(name, "steps(")
	if !ok {
		return Steps{}, false
	}
	inner, ok = strings.CutSuffix(inner, ")")
	if !ok {
		return Steps{}, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(inner))
	if err != nil || n <= 0 {
		return Steps{}, false
	}
	return Steps{Count: n}, true
}

// Curve is the easing curve an [Anim] plays with, read from JSON as either
// of the two forms brief/ENCRE_06 §6 needs: a name (`"ease-out"`, or the
// `"steps(1)"` of `cligne`), or the four control-point numbers of a
// cubic-bezier(x1,y1,x2,y2) as a bare JSON array — `joie` and `vol` overshoot
// 1 and so have no name of their own. See [Curve.UnmarshalJSON].
//
// Exactly one of Name and Bezier is set; [Curve.Easing] resolves whichever
// one to something [Curve.Validate] can check and a future caller can
// evaluate.
type Curve struct {
	// Name is a curve keyword: one of the CurveEase… constants, or
	// "steps(N)". Empty when Bezier is set directly instead.
	Name string
	// Bezier is the curve's four control points, when it is given directly
	// rather than by name. Nil when Name is set instead.
	Bezier *Bezier
}

// NamedCurve returns the curve named name (see the CurveEase… constants, and
// [parseSteps] for "steps(N)").
func NamedCurve(name string) Curve { return Curve{Name: name} }

// BezierCurve returns the cubic-bezier(x1,y1,x2,y2) curve of those four
// control points, in that order.
func BezierCurve(x1, y1, x2, y2 float64) Curve {
	return Curve{Bezier: &Bezier{X1: x1, Y1: y1, X2: x2, Y2: y2}}
}

// Easing resolves c to the [Easing] it names: a [Bezier] (by name, or given
// directly) or a [Steps] (from a "steps(N)" name).
func (c Curve) Easing() (Easing, error) {
	switch {
	case c.Name != "" && c.Bezier != nil:
		return nil, errors.New("curve sets both a name and a bezier; exactly one is expected")
	case c.Bezier != nil:
		return *c.Bezier, nil
	case c.Name == "":
		return nil, errors.New("curve names neither a standard easing nor a cubic-bezier")
	default:
		if s, ok := parseSteps(c.Name); ok {
			return s, nil
		}
		if b, ok := namedBeziers[c.Name]; ok {
			return b, nil
		}
		return nil, fmt.Errorf("curve names an unknown easing %q", c.Name)
	}
}

// Validate reports whether c names exactly one well-formed easing: a known
// name, a step count, or a bezier whose X1 and X2 both fall in [0,1].
func (c Curve) Validate() error {
	easing, err := c.Easing()
	if err != nil {
		return err
	}
	if b, ok := easing.(Bezier); ok {
		return b.Validate()
	}
	return nil
}

// UnmarshalJSON reads a curve from either of the two forms brief/ENCRE_06
// §6's curves need: a JSON string (a name), or a JSON array of four numbers
// (a cubic-bezier's control points, x1,y1,x2,y2 in order).
func (c *Curve) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		*c = Curve{Name: name}
		return nil
	}

	var points []float64
	if err := json.Unmarshal(data, &points); err != nil {
		return fmt.Errorf("curve: not a name or a bezier: %w", err)
	}
	if len(points) != 4 {
		return fmt.Errorf("curve: bezier has %d number(s), want exactly 4 (x1,y1,x2,y2)", len(points))
	}
	*c = Curve{Bezier: &Bezier{X1: points[0], Y1: points[1], X2: points[2], Y2: points[3]}}
	return nil
}

// MarshalJSON writes c back in whichever of the two forms
// [Curve.UnmarshalJSON] reads: a name, or an array of four numbers.
func (c Curve) MarshalJSON() ([]byte, error) {
	if c.Bezier != nil {
		return json.Marshal([4]float64{c.Bezier.X1, c.Bezier.Y1, c.Bezier.X2, c.Bezier.Y2})
	}
	return json.Marshal(c.Name)
}

// Anim is one row of brief/ENCRE_06 §6's table: how long a `@keyframes`
// animation takes, and the curve it plays with.
type Anim struct {
	// Duration is how long one cycle takes, even when the animation itself
	// loops.
	Duration Millis `json:"duration_ms,omitzero"`
	// Curve is the easing it plays with.
	Curve Curve `json:"curve"`
}

// Validate reports whether a has a duration and a well-formed curve.
func (a Anim) Validate() error {
	if a.Duration == 0 {
		return errors.New("duration is zero, and every value of the config is load-bearing")
	}
	return a.Curve.Validate()
}

// Juice carries every timing and curve of the game's motion: brief/ENCRE_06
// §6's table of `@keyframes`, plus the handful of brief/ENCRE_02 §12 rules §6
// does not restate because they are not a fixed animation — the score
// counter's duration follows log(score), not a single number.
//
// The zero value is not usable: every field here is load-bearing, so a config
// missing one is refused by [Juice.Validate] rather than silently animating at
// zero. Use [DefaultJuice], or [LoadJuice] over it.
type Juice struct {
	// HitstopTrap freezes the frame on a trap letter, and HitstopLegendary on
	// a legendary one (ENCRE_02 §12; ENCRE_06 §6 has no entry for either).
	HitstopTrap      Millis `json:"hitstop_trap_ms,omitzero"`
	HitstopLegendary Millis `json:"hitstop_legendary_ms,omitzero"`

	// SquashScale is how far a struck card shrinks, over SquashDuration, and
	// ReturnDuration is how long it takes to settle back to its own size from
	// the rebound. ReboundScale is the 108% overshoot both the struck card
	// (here) and the flying letter of Vol share — ENCRE_06 §6 cites the same
	// number for both rather than repeating it. Neither §12 nor §6 gives a
	// curve for this one.
	SquashScale    float64 `json:"squash_scale,omitzero"`
	SquashDuration Millis  `json:"squash_duration_ms,omitzero"`
	ReboundScale   float64 `json:"rebound_scale,omitzero"`
	ReturnDuration Millis  `json:"return_duration_ms,omitzero"`

	// ShakeAmplitude is the screen shake in pixels per unit of Mult, and
	// ShakeMaxAmplitude the cap on it — the formula ENCRE_06 §6's `tremble`
	// row cites straight from ENCRE_02 §12. Tremble is that row's own timing:
	// ENCRE_06 §6 corrects it to 500 ms, superseding the 200 ms of ENCRE_02
	// §12 (§6 is the shipped storyboard).
	ShakeAmplitude    float64 `json:"shake_amplitude,omitzero"`
	ShakeMaxAmplitude float64 `json:"shake_max_amplitude,omitzero"`
	Tremble           Anim    `json:"tremble"`

	// Droplet is `jetons` (ENCRE_06 §6): the arc one droplet of tokens takes
	// into the counter. ENCRE_06 §6 corrects it to 1.8 s staggered by 100 ms,
	// superseding ENCRE_02 §12's 400 ms / 40 ms. DropletPerTokens — one
	// droplet per ten tokens — and the rule that the counter only climbs as
	// each droplet lands are unchanged: both sections agree on them.
	Droplet          Anim   `json:"jetons"`
	DropletStagger   Millis `json:"jetons_stagger_ms,omitzero"`
	DropletPerTokens int    `json:"droplet_per_tokens,omitzero"`

	// CounterMinDuration and CounterMaxDuration bound the counter's own
	// duration, which grows with log(score) between the two — ENCRE_02 §12's
	// rule, which ENCRE_06 §6 does not override: its 1.8 s `compteur` row is
	// the fixed value of the storyboard demo, not the runtime formula, kept
	// here as CounterStoryboard for traceability against that board.
	// CounterArrivalScale is the scale, 1.18, a landing droplet snaps the
	// counter to before it settles back.
	CounterMinDuration  Millis  `json:"counter_min_duration_ms,omitzero"`
	CounterMaxDuration  Millis  `json:"counter_max_duration_ms,omitzero"`
	CounterStoryboard   Anim    `json:"compteur"`
	CounterArrivalScale float64 `json:"counter_arrival_scale,omitzero"`

	// BossLastLetterSlowFactor is the speed the boss's last letter is typed
	// at — half speed — and BossLastLetterSlowDuration how long that lasts
	// (ENCRE_02 §12; ENCRE_06 §6 has no entry for it).
	BossLastLetterSlowFactor   float64 `json:"boss_last_letter_slow_factor,omitzero"`
	BossLastLetterSlowDuration Millis  `json:"boss_last_letter_slow_duration_ms,omitzero"`

	// BrouillonShowDuration is how long Le Brouillon shows the word before
	// wiping it (brief/ENCRE_01 §13: "le mot apparaît 1 seconde puis
	// disparaît"). Neither ENCRE_02 §12 nor ENCRE_06 §6 carries it — it is not
	// one of their `@keyframes` — but it is exactly the kind of number this
	// package exists to hold rather than let a scene hardcode.
	BrouillonShowDuration Millis `json:"brouillon_show_duration_ms,omitzero"`

	// CardIdle is `flotte` (ENCRE_06 §6): a resting card's drift, corrected to
	// 3.4 s at 3 px amplitude, superseding ENCRE_02 §12's 2 px over 2 s.
	// Desynchronised card to card is a per-card phase offset, not a value
	// carried here.
	CardIdle          Anim    `json:"flotte"`
	CardIdleAmplitude float64 `json:"card_idle_amplitude,omitzero"`

	// CardFlip is the Rencontre's retournement (brief/ENCRE_06 §5): 300 ms,
	// scaleX from 1 to CardFlipMinScaleX and back, symmetric about the
	// midpoint. CardFlipEdgeColor is the `#6E5738` slab the card's own edge
	// shows while it is thin, roughly at that midpoint — the parchment's
	// underside, not the verso's own colour.
	CardFlip          Anim    `json:"retournement"`
	CardFlipMinScaleX float64 `json:"retournement_min_scale_x,omitzero"`
	// CardFlipEdgeColor is a "#RRGGBB" string rather than a colour type: this
	// package draws nothing, and a colour type would need one just to parse
	// this one field back out.
	CardFlipEdgeColor string `json:"retournement_edge_color,omitzero"`

	// Bave is what appears with — the ink mask spreading to fill it — and
	// Sechage is what leaves with, fading to Parchemin. ENCRE_06 §6 corrects
	// Bave's duration to 120 ms, superseding ENCRE_02 §12's 150 ms; its 200 ms
	// for Sechage matches both sections, now carrying the curve only §6 gives.
	Bave    Anim `json:"bave"`
	Sechage Anim `json:"sechage"`

	// EnluminureDuration is the length of the Enluminure in its entirety —
	// both sections agree on 6 s. ENCRE_06 §6 breaks it into 8 beats (ralenti,
	// silence, effacement, cursive ×2, cercle des créatures, Phalène, sceau
	// brisé) with no single curve of its own, so none is carried here.
	EnluminureDuration Millis `json:"enluminure_duration_ms,omitzero"`

	// The animations of brief/ENCRE_06 §6 with no counterpart in ENCRE_02
	// §12: a creature at rest (Respire), its blink (Cligne), its joy jump
	// (Joie), a mastery label rising off a card (Etiquette), the boss seal
	// cracking (Craque), the candle flame flickering at rest and at high
	// combo (VacilleRepos, VacilleHautCombo), the bet's three gold droplets
	// bouncing (Sautille, staggered by SautilleStagger), the seal key
	// pulsing once the word is complete (PulseValide), a typed letter's
	// flight from key to word — its 108% rebound is ReboundScale, shared
	// with the struck card above (Vol), and the atelier window's living ink,
	// looping (Encre).
	Respire          Anim   `json:"respire"`
	Cligne           Anim   `json:"cligne"`
	Joie             Anim   `json:"joie"`
	Etiquette        Anim   `json:"etiquette"`
	Craque           Anim   `json:"craque"`
	VacilleRepos     Anim   `json:"vacille_repos"`
	VacilleHautCombo Anim   `json:"vacille_haut_combo"`
	Sautille         Anim   `json:"sautille"`
	SautilleStagger  Millis `json:"sautille_stagger_ms,omitzero"`
	PulseValide      Anim   `json:"pulse_valide"`
	Vol              Anim   `json:"vol"`
	Encre            Anim   `json:"encre"`
}

// DefaultJuice returns the timings and curves of brief/ENCRE_06 §6, corrected
// over brief/ENCRE_02 §12 where the two disagree (see the field comments on
// [Juice]). The game runs on these without any file present; [LoadJuice] and
// [Reload] only ever override them.
func DefaultJuice() Juice {
	return Juice{
		HitstopTrap:      80,
		HitstopLegendary: 150,

		SquashScale:    0.85,
		SquashDuration: 60,
		ReboundScale:   1.08,
		ReturnDuration: 180,

		ShakeAmplitude:    2,
		ShakeMaxAmplitude: 12,
		Tremble:           Anim{Duration: 500, Curve: NamedCurve(CurveEaseOut)},

		Droplet:          Anim{Duration: 1800, Curve: NamedCurve(CurveEaseIn)},
		DropletStagger:   100,
		DropletPerTokens: 10,

		CounterMinDuration:  300,
		CounterMaxDuration:  3000,
		CounterStoryboard:   Anim{Duration: 1800, Curve: NamedCurve(CurveEaseOut)},
		CounterArrivalScale: 1.18,

		BossLastLetterSlowFactor:   0.5,
		BossLastLetterSlowDuration: 400,

		BrouillonShowDuration: 1000,

		CardIdle:          Anim{Duration: 3400, Curve: NamedCurve(CurveEaseInOut)},
		CardIdleAmplitude: 3,

		CardFlip:          Anim{Duration: 300, Curve: NamedCurve(CurveEaseInOut)},
		CardFlipMinScaleX: 0.06,
		CardFlipEdgeColor: "#6E5738",

		Bave:    Anim{Duration: 120, Curve: NamedCurve(CurveEaseOut)},
		Sechage: Anim{Duration: 200, Curve: NamedCurve(CurveEaseIn)},

		EnluminureDuration: 6000,

		Respire:          Anim{Duration: 3200, Curve: NamedCurve(CurveEaseInOut)},
		Cligne:           Anim{Duration: 4600, Curve: NamedCurve(CurveSteps1)},
		Joie:             Anim{Duration: 900, Curve: BezierCurve(.3, 1.4, .5, 1)},
		Etiquette:        Anim{Duration: 1800, Curve: NamedCurve(CurveEaseOut)},
		Craque:           Anim{Duration: 1800, Curve: NamedCurve(CurveEaseInOut)},
		VacilleRepos:     Anim{Duration: 1200, Curve: NamedCurve(CurveEaseInOut)},
		VacilleHautCombo: Anim{Duration: 700, Curve: NamedCurve(CurveEaseInOut)},
		Sautille:         Anim{Duration: 1200, Curve: NamedCurve(CurveEaseInOut)},
		SautilleStagger:  140,
		PulseValide:      Anim{Duration: 1400, Curve: NamedCurve(CurveEaseInOut)},
		Vol:              Anim{Duration: 400, Curve: BezierCurve(.3, .7, .4, 1)},
		Encre:            Anim{Duration: 40000, Curve: NamedCurve(CurveEaseInOut)},
	}
}

// validator is a field that checks its own well-formedness, rather than
// simply being refused for being the zero value — an [Anim] or a [Curve],
// whose zero-ness is not "every field zero" but "no curve named" or
// "no duration set".
type validator interface{ Validate() error }

// Validate reports the first field left at zero, or the first [Anim] or
// [Curve] field that is not well-formed.
//
// Every number in Juice is load-bearing — a zeroed Tremble.Duration makes the
// screen shake animate for no time, a zeroed CounterMaxDuration makes the
// score counter snap instead of count — so none of them has a meaningful
// zero. Rather than list the fields, it walks them the way
// [engine.Config.Validate] does, so a field introduced without a default is
// caught the first time this runs.
func (j Juice) Validate() error {
	v := reflect.ValueOf(j)
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		f := v.Field(i)
		if val, ok := f.Interface().(validator); ok {
			if err := val.Validate(); err != nil {
				return fmt.Errorf("juice: %s: %w", name, err)
			}
			continue
		}
		if f.IsZero() {
			return fmt.Errorf("juice: %s is zero, and every value of the config is load-bearing", name)
		}
	}
	return nil
}

// LoadJuice reads a partial or complete juice.json over [DefaultJuice]: a
// field the document does not name keeps its default, and a field it does
// name replaces it. It refuses a document that leaves a load-bearing field at
// zero or a curve malformed, or that does not parse; either way the caller's
// own configuration, held wherever it lives, is untouched — LoadJuice never
// mutates anything, it only ever returns a new value or an error.
func LoadJuice(raw []byte) (Juice, error) {
	j := DefaultJuice()
	if err := json.Unmarshal(raw, &j); err != nil {
		return Juice{}, fmt.Errorf("reading juice.json: %w", err)
	}
	if err := j.Validate(); err != nil {
		return Juice{}, err
	}
	return j, nil
}
