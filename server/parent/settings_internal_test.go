package parent

import "testing"

func TestDecodeChildSettingsDefaultsOnEmpty(t *testing.T) {
	settings, err := decodeChildSettings(nil)
	if err != nil {
		t.Fatalf("decodeChildSettings(nil): %v", err)
	}
	if settings.Keyboard != keyboardABC {
		t.Errorf("decodeChildSettings(nil).Keyboard = %q, want %q", settings.Keyboard, keyboardABC)
	}
	if !settings.Cahier {
		t.Errorf("decodeChildSettings(nil).Cahier = false, want true")
	}
	if settings.Rules["sosies.a_a"] {
		t.Errorf("decodeChildSettings(nil).Rules[sosies.a_a] = true, want false")
	}
}

func TestDecodeChildSettingsRejectsInvalidJSON(t *testing.T) {
	if _, err := decodeChildSettings([]byte("not json")); err == nil {
		t.Errorf("decodeChildSettings with invalid JSON: got nil error, want one")
	}
}

func TestDecodeDailyLimitDefaultsOnEmpty(t *testing.T) {
	limit, err := decodeDailyLimit(nil)
	if err != nil {
		t.Fatalf("decodeDailyLimit(nil): %v", err)
	}
	if limit.MinutesPerDay != defaultMinutesPerDay {
		t.Errorf("decodeDailyLimit(nil).MinutesPerDay = %d, want %d", limit.MinutesPerDay, defaultMinutesPerDay)
	}
}

func TestDecodeDailyLimitRejectsInvalidJSON(t *testing.T) {
	if _, err := decodeDailyLimit([]byte("nope")); err == nil {
		t.Errorf("decodeDailyLimit with invalid JSON: got nil error, want one")
	}
}

func TestRuleEnabledFallsBackToGroupDefault(t *testing.T) {
	settings := childSettings{Rules: map[string]bool{"masquees.ou": false}}
	if ruleEnabled(settings, "masquees.ou", true) {
		t.Errorf("ruleEnabled: explicit false entry was overridden by the group default")
	}
	if !ruleEnabled(settings, "masquees.oi", true) {
		t.Errorf("ruleEnabled: missing entry did not fall back to the group default")
	}
}
