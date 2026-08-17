// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDietTrendAnalysis(t *testing.T) {
	// Watched difficulty climbs 10 -> 40 across four dates; the unrated video
	// is watched too but must be excluded from the analysis.
	seedNovelStore(t,
		[]string{
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v1', '{}', 'easy one', 'beginner', 10)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v2', '{}', 'easy two', 'beginner', 20)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v3', '{}', 'harder', 'intermediate', 30)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v4', '{}', 'hardest', 'intermediate', 40)`,
			`INSERT INTO videos (id, data, title, level, difficulty) VALUES ('v5', '{}', 'unrated', 'beginner', NULL)`,
		},
		[]string{
			`INSERT INTO playlist (id, data, added_date, video_id) VALUES ('p1', '{}', '2026-01-01', 'v1')`,
			`INSERT INTO playlist (id, data, added_date, video_id) VALUES ('p2', '{}', '2026-01-02', 'v2')`,
			`INSERT INTO playlist (id, data, added_date, video_id) VALUES ('p3', '{}', '2026-01-03', 'v3')`,
			`INSERT INTO playlist (id, data, added_date, video_id) VALUES ('p4', '{}', '2026-01-04', 'v4')`,
			`INSERT INTO playlist (id, data, added_date, video_id) VALUES ('p5', '{}', '2026-01-05', 'v5')`,
		},
	)

	flags := &rootFlags{asJSON: true}
	out := runNovelCmd(t, newNovelDietCmd(flags), []string{"--window", "0"})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if n := got["watched_in_window"].(float64); n != 4 {
		t.Errorf("watched_in_window = %v, want 4 (unrated video must be excluded)", n)
	}
	// n=4 -> thirds of 1: early avg 10, recent avg 40, delta +30 -> rising.
	if trend := got["trend"]; trend != "rising" {
		t.Errorf("trend = %v, want rising (output: %s)", trend, out)
	}
	if d := got["delta"].(float64); d != 30 {
		t.Errorf("delta = %v, want 30", d)
	}
	if a := got["avg_difficulty"].(float64); a != 25 {
		t.Errorf("avg_difficulty = %v, want 25", a)
	}

	human := runNovelCmd(t, newNovelDietCmd(&rootFlags{plain: true}), []string{"--window", "0"})
	if !strings.Contains(human, "across all time") || strings.Contains(human, "last 0 days") {
		t.Errorf("all-time human output has the wrong window label: %q", human)
	}
}

func TestDietEmptyWindow(t *testing.T) {
	seedNovelStore(t, nil, nil)

	flags := &rootFlags{asJSON: true}
	out := runNovelCmd(t, newNovelDietCmd(flags), []string{"--window", "30d"})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if trend := got["trend"]; trend != "none" {
		t.Errorf("trend = %v, want none", trend)
	}
	if hint, _ := got["hint"].(string); hint == "" {
		t.Errorf("empty-window payload missing remediation hint: %s", out)
	}
}
