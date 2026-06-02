// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the hand-authored novel-feature support helpers.

package cli

import (
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		in      string
		wantErr bool
		// approxAgo is the expected age in hours; -1 means skip the age check.
		approxAgo float64
	}{
		{"", false, 24},
		{"24h", false, 24},
		{"90m", false, 1.5},
		{"7d", false, 24 * 7},
		{"2026-06-01", false, -1},
		{"not-a-time", true, -1},
	}
	for _, c := range cases {
		got, err := parseSince(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseSince(%q) expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSince(%q) unexpected error: %v", c.in, err)
			continue
		}
		if c.approxAgo < 0 {
			continue
		}
		ageHours := now.Sub(got).Hours()
		if ageHours < c.approxAgo-1 || ageHours > c.approxAgo+1 {
			t.Errorf("parseSince(%q) age = %.2fh, want ~%.2fh", c.in, ageHours, c.approxAgo)
		}
	}
}

func TestExtractMergeTags(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Hey {FIRST_NAME}, about {COMPANY}", []string{"FIRST_NAME", "COMPANY"}},
		{"no tags here", nil},
		{"dupes {X} and {X} and {Y}", []string{"X", "Y"}},
		{"lowercase {name} ignored", nil},
		{"{A1_B2} alnum ok", []string{"A1_B2"}},
	}
	for _, c := range cases {
		got := extractMergeTags(c.in)
		if len(got) != len(c.want) {
			t.Errorf("extractMergeTags(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("extractMergeTags(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestUpperASCII(t *testing.T) {
	cases := map[string]string{
		"first_name": "FIRST_NAME",
		"Company":    "COMPANY",
		"MIXED_9":    "MIXED_9",
	}
	for in, want := range cases {
		if got := upperASCII(in); got != want {
			t.Errorf("upperASCII(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckAndReadyLabels(t *testing.T) {
	if checkLabel(true) != "ok" || checkLabel(false) != "MISSING" {
		t.Errorf("checkLabel labels incorrect")
	}
	if readyLabel(true) != "READY" || readyLabel(false) != "NOT READY" {
		t.Errorf("readyLabel labels incorrect")
	}
}
