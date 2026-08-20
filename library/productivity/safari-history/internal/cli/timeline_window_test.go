package cli

import (
	"testing"
	"time"
)

// A bare calendar date must scope to that single day, not run to now.
func TestTimelineWindowSingleDay(t *testing.T) {
	start, end, err := timelineWindow("2026-05-01", "")
	if err != nil {
		t.Fatalf("timelineWindow: %v", err)
	}
	wantStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %s, want %s", start, wantStart)
	}
	if d := end.Sub(start); d != 24*time.Hour {
		t.Fatalf("window = %s, want 24h (single day)", d)
	}
}

// Relative expressions keep the "N ago → now" range.
func TestTimelineWindowRelative(t *testing.T) {
	start, end, err := timelineWindow("7d", "")
	if err != nil {
		t.Fatalf("timelineWindow: %v", err)
	}
	if d := end.Sub(start); d < 6*24*time.Hour || d > 8*24*time.Hour {
		t.Fatalf("relative window = %s, want ~7d", d)
	}
	if time.Since(end) > time.Minute {
		t.Fatalf("relative window end = %s, want ~now", end)
	}
}
