package engine_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/oioio-space/encre/engine"
)

func TestDefaultConfigCarriesTheValuesOfTheSpec(t *testing.T) {
	// brief/ENCRE_04_spec_technique.md §4. The three K values are the ones the
	// balance simulation settled on after nine versions; changing them changes
	// who keeps playing, so they are pinned here rather than tuned in passing.
	cfg := engine.DefaultConfig()

	tests := []struct {
		name      string
		got, want any
	}{
		{"WordsPerManche", cfg.WordsPerManche, 6},
		{"K", cfg.K, [3]float64{2.2, 5.5, 14}},
		{"RankMult", cfg.RankMult, [6]float64{1, 1.3, 1.7, 2.2, 2.8, 3.5}},
		{"RankUpWins", cfg.RankUpWins, 8},
		{"RankUpMinWeeks", cfg.RankUpMinWeeks, 3},
		{"RankDownFails", cfg.RankDownFails, 3},
		{"KindnessStep", cfg.KindnessStep, 0.03},
		{"KindnessFloor", cfg.KindnessFloor, 0.85},
		{"BlindTokens", cfg.BlindTokens, 2},
		{"BlindMult", cfg.BlindMult, 3.0},
		{"ChipPerTrap", cfg.ChipPerTrap, 10.0},
		{"LevelMax", cfg.LevelMax, 10},
		{"GoldDays", cfg.GoldDays, 3},
		{"GoldMinSpanDays", cfg.GoldMinSpanDays, int32(7)},
		{"TarnishWeeks", cfg.TarnishWeeks, int32(4)},
		{"CurseFails", cfg.CurseFails, 3},
		{"RevancheWindow", cfg.RevancheWindow, 0.85},
		{"GardeSlots", cfg.GardeSlots, 3},
		{"CahierBonus", cfg.CahierBonus, 50.0},
		{"TargetPHat", cfg.TargetPHat, 0.85},
	}
	for _, tt := range tests {
		if !reflect.DeepEqual(tt.got, tt.want) {
			t.Errorf("DefaultConfig().%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	if err := engine.DefaultConfig().Validate(); err != nil {
		t.Errorf("DefaultConfig().Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsEveryFieldLeftAtZero(t *testing.T) {
	// A property rather than a list: zero each field of the default in turn and
	// require the error to name it. A field added later without a default is
	// caught by this test the day it appears, which a hand-written list would
	// not do.
	base := engine.DefaultConfig()
	v := reflect.ValueOf(base)

	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		t.Run(name, func(t *testing.T) {
			cfg := base
			field := reflect.ValueOf(&cfg).Elem().Field(i)
			field.Set(reflect.Zero(field.Type()))

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() with %s at zero = nil, want an error", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("Validate() error = %q, want it to name %s", err, name)
			}
		})
	}
}

func TestConfigSurvivesAJSONRoundTrip(t *testing.T) {
	// ENCRE_04 §4 has the config loaded from JSON, so the tuning of the first
	// month can happen without a rebuild.
	want := engine.DefaultConfig()

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := engine.LoadConfig(raw)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestLoadConfigRefusesAnIncompleteDocument(t *testing.T) {
	// Loading a partial config would silently zero whatever it omits, and a
	// zeroed K makes every target unreachable.
	if _, err := engine.LoadConfig([]byte(`{"WordsPerManche": 6}`)); err == nil {
		t.Error("LoadConfig on a partial document returned no error, want one")
	}
}

func TestLoadConfigRefusesWhatIsNotJSON(t *testing.T) {
	if _, err := engine.LoadConfig([]byte("pas du json")); err == nil {
		t.Error("LoadConfig on junk returned no error, want one")
	}
}
