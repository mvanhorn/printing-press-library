// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

// agenticRecCLI builds an is-agentic ScanRecord for the CLI store (used to seed
// a crossref where the is-agentic side is present).
func agenticRecCLI(url, at string, score int, issues [][3]string) store.ScanRecord {
	ag := &store.AgenticReport{Target: url, Score: score, ScoreLabel: "L", ScannedAt: at}
	for _, is := range issues {
		ag.Issues = append(ag.Issues, store.AgenticIssue{ID: is[0], Tier: is[1], Result: is[2]})
	}
	b, _ := json.Marshal(ag)
	return store.ScanRecord{URL: url, Source: store.SourceIsAgentic, ScannedAt: at, Score: &score, Raw: b}
}

func TestCrossrefWithOneScannerMissing(t *testing.T) {
	// Seed only the isitagentready side; the is-agentic local lookup 404s.
	home := seedStore(t, sampleRec(testURL, "2026-06-22T10:00:00Z", 2, map[string]string{"sitemap": "pass"}, nil))
	out, _, err := runCLI(t, home, "crossref", testURL, "--agent", "--data-source", "local")
	if err != nil {
		t.Fatalf("crossref with one scanner missing must exit 0, got %v", err)
	}
	var res struct {
		URL            string         `json:"url"`
		IsItAgentReady map[string]any `json:"isitagentready"`
		IsAgentic      map[string]any `json:"isAgentic"`
		Notes          []string       `json:"notes"`
	}
	if e := json.Unmarshal([]byte(out), &res); e != nil {
		t.Fatalf("not JSON: %v (%s)", e, out)
	}
	if res.URL != testURL {
		t.Fatalf("url=%v", res.URL)
	}
	if res.IsItAgentReady == nil {
		t.Fatalf("isitagentready should be populated, got nil")
	}
	if res.IsAgentic != nil {
		t.Fatalf("isAgentic should be null (no local is-agentic scan), got %v", res.IsAgentic)
	}
	// The unavailable reason must be surfaced in notes.
	found := false
	for _, n := range res.Notes {
		if strings.Contains(n, "is-agentic unavailable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an is-agentic unavailable note, got %v", res.Notes)
	}
}

func TestCrossrefWithBothScannersMissing(t *testing.T) {
	// Empty store + local data-source -> both scanners have no data.
	home := seedStore(t)
	_, _, err := runCLI(t, home, "crossref", testURL, "--agent", "--data-source", "local")
	if err == nil || ExitCode(err) == 0 {
		t.Fatalf("crossref with both scanners missing must exit non-zero, got %v", err)
	}
}

func TestCrossrefHelpWires(t *testing.T) {
	home := seedStore(t)
	out, _, err := runCLI(t, home, "crossref", "--help")
	if err != nil {
		t.Fatalf("crossref --help error = %v (%s)", err, out)
	}
}
