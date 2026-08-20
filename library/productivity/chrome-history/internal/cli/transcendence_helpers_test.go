package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
)

func TestSplitSessionsAndDwellCap(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []source.VisitRow{
		{VisitID: 1, VisitTime: base, VisitDuration: 0},
		{VisitID: 2, VisitTime: base.Add(10 * time.Minute), VisitDuration: 0},
		{VisitID: 3, VisitTime: base.Add(80 * time.Minute), VisitDuration: 45 * time.Minute},
	}
	gap := 30 * time.Minute
	sessions := splitSessions(events, gap)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	dwell := estimateDwellMicros(sessions[0], gap)
	if len(dwell) != 2 {
		t.Fatalf("expected 2 dwell rows, got %d", len(dwell))
	}
	if dwell[0] != int64((10 * time.Minute).Microseconds()) {
		t.Fatalf("unexpected derived dwell: %d", dwell[0])
	}
	dwell2 := estimateDwellMicros(sessions[1], gap)
	if dwell2[0] != int64((30 * time.Minute).Microseconds()) {
		t.Fatalf("expected capped dwell, got %d", dwell2[0])
	}
}
