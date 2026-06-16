package service

import (
	"testing"
	"time"
)

func TestRandomSlowDelayClampsToTouchPieRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		delay := randomSlowDelay(-30, 600)
		if delay < 0 || delay > 60*time.Second {
			t.Fatalf("delay out of range: %s", delay)
		}
	}

	if got := randomSlowDelay(0, 0); got != 0 {
		t.Fatalf("expected zero config to keep zero delay, got %s", got)
	}
	if got := randomSlowDelay(90, 120); got != 60*time.Second {
		t.Fatalf("expected high config to clamp to 60s, got %s", got)
	}
}
