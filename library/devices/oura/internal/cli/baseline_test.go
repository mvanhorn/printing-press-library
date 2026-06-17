// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestBandFor(t *testing.T) {
	cases := []struct {
		z    float64
		want string
	}{
		{0, "normal"},
		{0.9, "normal"},
		{-0.9, "normal"},
		{1.5, "elevated"},
		{-1.5, "elevated"},
		{2.5, "notable"},
		{-3, "notable"},
	}
	for _, tc := range cases {
		if got := bandFor(tc.z); got != tc.want {
			t.Errorf("bandFor(%v) = %q, want %q", tc.z, got, tc.want)
		}
	}
}

func TestRound2(t *testing.T) {
	if got := round2(1.005); got != 1.01 && got != 1.0 {
		t.Errorf("round2(1.005) = %v", got)
	}
	if got := round2(2.344); got != 2.34 {
		t.Errorf("round2(2.344) = %v, want 2.34", got)
	}
	if got := round2(-1.236); got != -1.24 {
		t.Errorf("round2(-1.236) = %v, want -1.24", got)
	}
}

func TestNewNovelBaselineCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelBaselineCmd(flags)
	if cmd.Use != "baseline" {
		t.Errorf("Use = %q, want baseline", cmd.Use)
	}
	if err := cmd.Flags().Parse([]string{}); err != nil {
		t.Fatalf("parsing empty flags: %v", err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("bare invocation should show help, got error: %v", err)
	}
}
