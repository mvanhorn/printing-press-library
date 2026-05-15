// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Table-driven happy-path tests for the pure-logic helpers in
// p1_common.go that the v1.1 novel verbs depend on.

package cli

import (
	"testing"
	"time"
)

func TestResolveWindowTS(t *testing.T) {
	cases := []struct {
		name    string
		window  string
		wantErr bool
	}{
		{"empty defaults to 7d", "", false},
		{"days", "14d", false},
		{"hours", "24h", false},
		{"weeks", "1w", false},
		{"minutes", "30m", false},
		{"bad unit", "5y", true},
		{"garbage", "soon", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, err := resolveWindowTS(tc.window)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got ts %q", tc.window, ts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.window, err)
			}
			// A valid window produces a Slack ts before now.
			if got := slackTSToTime(ts); !got.Before(time.Now().Add(time.Minute)) {
				t.Fatalf("window ts %q parsed to %v, expected a past instant", ts, got)
			}
		})
	}
}

func TestSlackTSRoundTrip(t *testing.T) {
	cases := []string{"17470000.000000", "17470000.001200", "15000000.000000"}
	for _, ts := range cases {
		got := slackTSToTime(ts)
		if got.IsZero() {
			t.Fatalf("slackTSToTime(%q) returned zero time", ts)
		}
		// Re-format and compare whole seconds.
		if back := unixToSlackTS(got); back[:10] != ts[:10] {
			t.Fatalf("round trip %q -> %v -> %q diverged", ts, got, back)
		}
	}
	if !slackTSToTime("not-a-ts").IsZero() {
		t.Fatalf("expected zero time for unparseable ts")
	}
}

func TestRedactSensitive(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		on       bool
		wantSafe []string // substrings that must NOT survive
		wantKeep []string // substrings that MUST survive
	}{
		{
			name:     "strips comp keyword",
			in:       "Discussed the new compensation plan",
			on:       true,
			wantSafe: []string{"compensation"},
			wantKeep: []string{"Discussed", "plan"},
		},
		{
			name:     "strips resignation stem",
			in:       "German renunció ayer",
			on:       true,
			wantSafe: []string{"renunció"},
			wantKeep: []string{"German", "ayer"},
		},
		{
			name:     "off leaves text intact",
			in:       "compensation accelerator pip",
			on:       false,
			wantKeep: []string{"compensation", "accelerator", "pip"},
		},
		{
			name:     "clean text unchanged",
			in:       "Sonria onboarding call went well",
			on:       true,
			wantKeep: []string{"Sonria", "onboarding", "call"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maybeRedact(tc.in, tc.on)
			for _, bad := range tc.wantSafe {
				if containsFold(got, bad) {
					t.Fatalf("redacted output %q still contains %q", got, bad)
				}
			}
			for _, keep := range tc.wantKeep {
				if !containsFold(got, keep) {
					t.Fatalf("redacted output %q dropped expected token %q", got, keep)
				}
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	cases := []struct {
		hay, needle string
		want        bool
	}{
		{"Sonria onboarding", "sonria", true},
		{"PETROAUTOS deal", "petroautos", true},
		{"churnsales channel", "missing", false},
	}
	for _, tc := range cases {
		if got := containsFold(tc.hay, tc.needle); got != tc.want {
			t.Fatalf("containsFold(%q,%q)=%v want %v", tc.hay, tc.needle, got, tc.want)
		}
	}
}
