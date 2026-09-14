package parent

import (
	"testing"
	"time"
)

func TestServerNowUsesClockWhenSet(t *testing.T) {
	fixed := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	s := &Server{clock: func() time.Time { return fixed }}
	if got := s.now(); !got.Equal(fixed) {
		t.Errorf("now() with a clock set: got %v, want %v", got, fixed)
	}

	s2 := &Server{}
	if got := s2.now(); got.IsZero() {
		t.Errorf("now() with no clock set: got zero time, want time.Now()")
	}
}
