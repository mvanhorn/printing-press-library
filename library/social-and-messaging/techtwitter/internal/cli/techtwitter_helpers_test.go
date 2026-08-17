package cli

import (
	"testing"
	"time"
)

func TestTTEngagement(t *testing.T) {
	cases := []struct {
		name                             string
		like, retweet, comment, bookmark int
		want                             int
	}{
		{"zero", 0, 0, 0, 0, 0},
		{"likes only", 10, 0, 0, 0, 10},
		{"weighted", 1, 2, 3, 4, 1 + 4 + 9 + 16}, // like + 2r + 3c + 4b
		{"bookmark heavy", 0, 0, 0, 5, 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ttEngagement(c.like, c.retweet, c.comment, c.bookmark); got != c.want {
				t.Fatalf("ttEngagement(%d,%d,%d,%d) = %d, want %d", c.like, c.retweet, c.comment, c.bookmark, got, c.want)
			}
		})
	}
}

func TestTTParseWindow(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", ttDefaultWindow, false},
		{"24h", 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"0h", 0, true},
		{"banana", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ttParseWindow(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ttParseWindow(%q) expected error, got %v", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ttParseWindow(%q) unexpected error: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ttParseWindow(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestResolveDatePrefix(t *testing.T) {
	if got := resolveDatePrefix("latest"); got != "" {
		t.Fatalf("resolveDatePrefix(latest) = %q, want empty", got)
	}
	if got := resolveDatePrefix(""); got != "" {
		t.Fatalf("resolveDatePrefix(empty) = %q, want empty", got)
	}
	if got := resolveDatePrefix("2026-06-07"); got != "2026-06-07" {
		t.Fatalf("resolveDatePrefix(date) = %q, want passthrough", got)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if got := resolveDatePrefix("today"); got != today {
		t.Fatalf("resolveDatePrefix(today) = %q, want %q", got, today)
	}
	yest := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if got := resolveDatePrefix("YESTERDAY"); got != yest {
		t.Fatalf("resolveDatePrefix(YESTERDAY) = %q, want %q (case-insensitive)", got, yest)
	}
}

func TestTTPriorSnapshotWithin(t *testing.T) {
	// times DESC: newest first.
	times := []string{
		"2026-06-14T12:00:00Z", // latest
		"2026-06-13T12:00:00Z", // within a 7d window
		"2026-06-08T12:00:00Z", // within 7d window (oldest in-window)
		"2026-06-01T12:00:00Z", // outside 7d window
	}
	latest := "2026-06-14T12:00:00Z"

	// 7d window cutoff: oldest within-window snapshot wins (window momentum).
	cutoff7d := "2026-06-07T12:00:00Z"
	if got := ttPriorSnapshotWithin(times, latest, cutoff7d); got != "2026-06-08T12:00:00Z" {
		t.Fatalf("prior(7d) = %q, want oldest in-window 2026-06-08", got)
	}

	// No prior at all.
	if got := ttPriorSnapshotWithin([]string{latest}, latest, cutoff7d); got != "" {
		t.Fatalf("prior(single) = %q, want empty", got)
	}

	// Nothing within a tight window: fall back to the most recent older one.
	cutoffTight := "2026-06-14T11:00:00Z"
	if got := ttPriorSnapshotWithin(times, latest, cutoffTight); got != "2026-06-13T12:00:00Z" {
		t.Fatalf("prior(tight) = %q, want most-recent prior 2026-06-13", got)
	}
}

func TestTTEvidenceKinds(t *testing.T) {
	for _, k := range []string{"auto", "what-changed", "arguments", "read-list", "narrative-alert"} {
		if !ttEvidenceKinds[k] {
			t.Fatalf("expected kind %q to be valid", k)
		}
	}
	if ttEvidenceKinds["launches"] {
		t.Fatalf("launches should not be an offline evidence kind (products not stored)")
	}
}
