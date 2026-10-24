// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestSimilarSpotifyInPageOrder(t *testing.T) {
	f := newS2AFake(t)
	rows := s2aJSONArray(t, f.mustOK("similar", "324684580", "--json").stdout)
	if len(rows) != 10 {
		t.Fatalf("similar = %d rows, want 10", len(rows))
	}
	for i, r := range rows {
		if s2aNum(t, r["position"]) != float64(i+1) {
			t.Fatalf("row %d position %v", i, r["position"])
		}
		if r["appId"] == "324684580" || r["appId"] == "" || r["title"] == "" {
			t.Fatalf("row %d = %v", i, r)
		}
	}
	if rows[0]["appId"] != "912390687" || !strings.HasPrefix(rows[0]["title"].(string), "TREBEL") {
		t.Fatalf("first similar = %v", rows[0])
	}
	if rows[3]["appId"] != "1017492454" {
		t.Fatalf("fourth similar = %v, want YouTube Music 1017492454", rows[3])
	}
	three := s2aJSONArray(t, f.mustOK("similar", "324684580", "--limit", "3", "--json").stdout)
	if len(three) != 3 || three[2]["appId"] != rows[2]["appId"] {
		t.Fatalf("--limit 3 = %v", three)
	}
	// similar writes no snapshots (contract); the capture log still has the fetches.
	st := f.openStore()
	snaps, _ := store.AppSnapshots(context.Background(), st, "324684580", "", s2aFixedNow.AddDate(-10, 0, 0))
	caps, _ := store.Captures(context.Background(), st, store.CaptureFilter{})
	if len(snaps) != 0 || len(caps) != 2 {
		t.Fatalf("snapshots %d (want 0), captures %d (want 2)", len(snaps), len(caps))
	}
}

func TestSimilarNoShelfAndNegative(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("similar", "361309726", "--json")
	if strings.TrimSpace(res.stdout) != "[]" || !strings.Contains(res.stderr, "no \"You Might Also Like\" shelf") {
		t.Fatalf("Pages has no similar shelf: stdout %s stderr %s", res.stdout, res.stderr)
	}
	if res := f.run("similar", "1"); res.code != 3 {
		t.Fatalf("similar 1 exit = %d, want 3", res.code)
	}
	if res := f.run("similar", "324684580", "--limit", "-2"); res.code != 2 || res.requests != 0 {
		t.Fatalf("--limit -2 exit %d requests %d", res.code, res.requests)
	}
}
