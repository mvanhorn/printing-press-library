// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func TestSourceSeconds_MicAndSystem(t *testing.T) {
	segs := []granola.TranscriptSegment{
		{Source: "microphone", StartTimestamp: "2026-05-01T10:00:00Z", EndTimestamp: "2026-05-01T10:00:30Z"},
		{Source: "microphone", StartTimestamp: "2026-05-01T10:00:30Z", EndTimestamp: "2026-05-01T10:01:00Z"},
		{Source: "system", StartTimestamp: "2026-05-01T10:01:00Z", EndTimestamp: "2026-05-01T10:01:45Z"},
		{Source: "system", StartTimestamp: "2026-05-01T10:01:45Z", EndTimestamp: "2026-05-01T10:02:30Z"},
	}
	mic, sys := sourceSeconds(segs)
	if mic != 60 {
		t.Errorf("expected mic=60, got %v", mic)
	}
	if sys != 90 {
		t.Errorf("expected sys=90, got %v", sys)
	}
}

func TestAggregateBySources(t *testing.T) {
	segs := []granola.TranscriptSegment{
		{Source: "microphone", StartTimestamp: "2026-05-01T10:00:00Z", EndTimestamp: "2026-05-01T10:00:30Z", Confidence: 0.9},
		{Source: "system", StartTimestamp: "2026-05-01T10:00:30Z", EndTimestamp: "2026-05-01T10:01:00Z", Confidence: 0.8},
	}
	agg := aggregateBySources(segs)
	if agg["microphone_seconds"].(float64) != 30 {
		t.Errorf("mic_seconds wrong: %v", agg["microphone_seconds"])
	}
	if agg["system_seconds"].(float64) != 30 {
		t.Errorf("sys_seconds wrong: %v", agg["system_seconds"])
	}
	avg := agg["confidence_avg"].(float64)
	if avg < 0.84 || avg > 0.86 {
		t.Errorf("expected avg conf ~0.85, got %v", avg)
	}
}
