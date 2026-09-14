package api

import "time"

// secondsPerDay and secondsPerWeek turn a [time.Time] into the epoch-day and
// epoch-week units server/store and engine key their own state on.
const (
	secondsPerDay  = 24 * 60 * 60
	secondsPerWeek = 7 * secondsPerDay
)

// dayOf returns the epoch day t falls in: days since the Unix epoch, the
// unit [server/store.Store.AddPlayTime] and [server/store.Store.PlayTime]
// key play time on, and engine.Record's day parameter.
// #nosec G115 -- an epoch day from a wall-clock time is five figures; int32
// overflows in the year 5,881,580.
func dayOf(t time.Time) int32 { return int32(t.Unix() / secondsPerDay) }

// weekOf returns the epoch week t falls in: the unit BuildDeck's weekNo and
// engine.Apply's week parameter use for rank, level and curse cooldowns.
// #nosec G115 -- an epoch week from a wall-clock time is four figures; see dayOf.
func weekOf(t time.Time) int32 { return int32(t.Unix() / secondsPerWeek) }
