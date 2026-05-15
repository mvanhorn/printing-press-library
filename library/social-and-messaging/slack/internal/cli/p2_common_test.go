// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestRenderSubteamMentions(t *testing.T) {
	handles := map[string]string{
		"S012ABC": "csm-team",
		"S099XYZ": "leadership",
	}
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"inline handle", "ping <!subteam^S012ABC|@csm-team> now", "ping @csm-team now"},
		{"inline handle no at", "<!subteam^S012ABC|csm-team>", "@csm-team"},
		{"id resolved from map", "owner <!subteam^S099XYZ>", "owner @leadership"},
		{"unknown id degrades to id", "<!subteam^S404NONE>", "@S404NONE"},
		{"no subteam token untouched", "plain text here", "plain text here"},
		{"multiple tokens", "<!subteam^S012ABC> and <!subteam^S099XYZ>", "@csm-team and @leadership"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderSubteamMentions(tc.in, handles); got != tc.want {
				t.Errorf("renderSubteamMentions(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestWindowBounds(t *testing.T) {
	since, until, err := windowBounds("7d")
	if err != nil {
		t.Fatalf("windowBounds(7d) errored: %v", err)
	}
	if since == "" {
		t.Errorf("expected a non-empty since bound for 7d")
	}
	if until == "" {
		t.Errorf("expected a non-empty until bound")
	}
	if since >= until {
		t.Errorf("since %q should be lexically before until %q", since, until)
	}

	// Empty window => open-ended lower bound.
	since, until, err = windowBounds("")
	if err != nil {
		t.Fatalf("windowBounds(\"\") errored: %v", err)
	}
	if since != "" {
		t.Errorf("empty window should yield an empty since bound, got %q", since)
	}
	if until == "" {
		t.Errorf("empty window should still yield an until bound")
	}

	// Malformed window => error.
	if _, _, err := windowBounds("banana"); err == nil {
		t.Errorf("expected an error for a malformed window value")
	}
}

func TestTSToTime(t *testing.T) {
	got := tsToTime("17000000.001200")
	want := time.Unix(17000000, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("tsToTime = %v, want %v", got, want)
	}
	if !tsToTime("").IsZero() {
		t.Errorf("empty ts should yield the zero time")
	}
	if !tsToTime("not-a-ts").IsZero() {
		t.Errorf("malformed ts should yield the zero time")
	}
}
