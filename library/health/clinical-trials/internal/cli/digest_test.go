// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"testing"
)

// TestNovelDigestHelpWires smoke-tests that the digest command resolves at
// runtime and renders --help without error.
func TestNovelDigestHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"digest", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("digest --help error = %v (novel command not wired correctly?)", err)
	}
}

func TestBuildDigest(t *testing.T) {
	trials := []Trial{
		{NCTID: "NCT1", Status: "RECRUITING", LastUpdate: "2026-01-01"},
		{NCTID: "NCT2", Status: "ENROLLING_BY_INVITATION", LastUpdate: "2026-03-01"},
		{NCTID: "NCT3", Status: "TERMINATED", LastUpdate: "2026-02-01"},
		{NCTID: "NCT4", Status: "WITHDRAWN", LastUpdate: "2026-05-01"},
		{NCTID: "NCT5", Status: "COMPLETED", LastUpdate: "2026-04-01"},
		{NCTID: "NCT6", Status: "SUSPENDED", LastUpdate: "2026-06-01"},
	}

	t.Run("counts recruiting and stopped subsets", func(t *testing.T) {
		v := buildDigest("cancer", trials, 10)
		if v.Total != 6 {
			t.Errorf("Total = %d, want 6", v.Total)
		}
		// RECRUITING + ENROLLING_BY_INVITATION only.
		if v.Recruiting != 2 {
			t.Errorf("Recruiting = %d, want 2", v.Recruiting)
		}
		// TERMINATED + WITHDRAWN + SUSPENDED.
		if len(v.RecentlyTerminated) != 3 {
			t.Errorf("RecentlyTerminated = %d, want 3", len(v.RecentlyTerminated))
		}
	})

	t.Run("newest sorted by last update descending", func(t *testing.T) {
		v := buildDigest("cancer", trials, 10)
		wantOrder := []string{"NCT6", "NCT4", "NCT5", "NCT2", "NCT3", "NCT1"}
		if len(v.Newest) != len(wantOrder) {
			t.Fatalf("Newest len = %d, want %d", len(v.Newest), len(wantOrder))
		}
		for i, want := range wantOrder {
			if v.Newest[i].NCTID != want {
				t.Errorf("Newest[%d] = %s, want %s", i, v.Newest[i].NCTID, want)
			}
		}
	})

	t.Run("recently-stopped sorted by last update descending", func(t *testing.T) {
		v := buildDigest("cancer", trials, 10)
		wantOrder := []string{"NCT6", "NCT4", "NCT3"} // suspended, withdrawn, terminated
		if len(v.RecentlyTerminated) != len(wantOrder) {
			t.Fatalf("RecentlyTerminated len = %d, want %d", len(v.RecentlyTerminated), len(wantOrder))
		}
		for i, want := range wantOrder {
			if v.RecentlyTerminated[i].NCTID != want {
				t.Errorf("RecentlyTerminated[%d] = %s, want %s", i, v.RecentlyTerminated[i].NCTID, want)
			}
		}
	})

	t.Run("limit caps both lists", func(t *testing.T) {
		v := buildDigest("cancer", trials, 2)
		if len(v.Newest) != 2 {
			t.Errorf("Newest len = %d, want 2 (capped)", len(v.Newest))
		}
		if len(v.RecentlyTerminated) != 2 {
			t.Errorf("RecentlyTerminated len = %d, want 2 (capped)", len(v.RecentlyTerminated))
		}
		// Cap keeps the newest, so NCT6 and NCT4 lead both lists.
		if v.Newest[0].NCTID != "NCT6" || v.Newest[1].NCTID != "NCT4" {
			t.Errorf("Newest after cap = %s,%s, want NCT6,NCT4", v.Newest[0].NCTID, v.Newest[1].NCTID)
		}
	})

	t.Run("no stopped trials leaves empty list", func(t *testing.T) {
		only := []Trial{
			{NCTID: "A", Status: "RECRUITING", LastUpdate: "2026-01-01"},
			{NCTID: "B", Status: "COMPLETED", LastUpdate: "2026-02-01"},
		}
		v := buildDigest("x", only, 5)
		if len(v.RecentlyTerminated) != 0 {
			t.Errorf("RecentlyTerminated = %v, want empty", v.RecentlyTerminated)
		}
		if v.Recruiting != 1 {
			t.Errorf("Recruiting = %d, want 1", v.Recruiting)
		}
	})
}

func TestTopDigestTrials(t *testing.T) {
	trials := []Trial{
		{NCTID: "A", LastUpdate: "2026-01-01"},
		{NCTID: "B", LastUpdate: "2026-03-01"},
		{NCTID: "C", LastUpdate: "2026-02-01"},
	}

	t.Run("limit larger than input returns all", func(t *testing.T) {
		got := topDigestTrials(append([]Trial(nil), trials...), 10)
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		if got[0].NCTID != "B" {
			t.Errorf("first = %s, want B (newest)", got[0].NCTID)
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		got := topDigestTrials(nil, 5)
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}
