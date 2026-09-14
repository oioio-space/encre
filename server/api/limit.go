package api

import (
	"context"
	jsonv2 "encoding/json/v2"

	"github.com/oioio-space/encre/server/store"
)

// defaultDailyLimitMinutes is the default play-time budget (brief/ENCRE_01,
// "15 à 20 minutes par jour"), used whenever a child's daily_limit_json is
// empty or does not parse as [dailyLimit] — a parent who has never opened
// the settings page gets ENCRE_01's default rather than an unlimited child.
const defaultDailyLimitMinutes = 20

// dailyLimit is this package's reading of [store.Child.DailyLimit], which
// server/store keeps as opaque JSON because it has no engine type of its own
// yet. Minutes is the only field this ticket's endpoints need; a settings
// page that adds per-weekday overrides can extend this struct without
// touching the column.
type dailyLimit struct {
	Minutes int `json:"minutes"`
}

// dailyLimitMinutes reads raw as a [dailyLimit] and returns its Minutes, or
// [defaultDailyLimitMinutes] if raw is empty, unparsable, or non-positive.
func dailyLimitMinutes(raw []byte) int {
	var dl dailyLimit
	if len(raw) == 0 {
		return defaultDailyLimitMinutes
	}
	if err := jsonv2.Unmarshal(raw, &dl); err != nil || dl.Minutes <= 0 {
		return defaultDailyLimitMinutes
	}
	return dl.Minutes
}

// remainingSeconds returns how much play time child has left today: the
// daily limit plus any bonus the parent granted, minus what has already been
// played, floored at nothing (a negative budget is reported as negative, so
// callers can tell "just ran out" from "nowhere close").
func (s *Server) remainingSeconds(ctx context.Context, child *store.Child) (int, error) {
	played, bonus, err := s.db.PlayTime(ctx, child.ID, dayOf(s.now()))
	if err != nil {
		return 0, err
	}
	return dailyLimitMinutes(child.DailyLimit)*60 + bonus - played, nil
}
