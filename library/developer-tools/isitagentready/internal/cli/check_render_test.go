// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

// agenticRawReport builds a raw is-agentic report body for renderer tests.
func agenticRawReport(t *testing.T) json.RawMessage {
	t.Helper()
	rep := store.AgenticReport{
		Target:         "https://render-test.example",
		Score:          73,
		ScoreLabel:     "Mostly ready",
		ScannedAt:      "2026-08-20T09:00:00Z",
		EligibleChecks: 31,
		ScoreBreakdown: store.AgenticBreakdown{
			Essential:   store.AgenticTier{Earned: 41.5, Available: 55, Passing: 8, Total: 11},
			Recommended: store.AgenticTier{Earned: 17.9, Available: 40, Passing: 12, Total: 20},
			Bonus:       store.AgenticBonus{Points: 3.5, PositiveSignals: 2},
		},
		Issues: []store.AgenticIssue{
			{ID: "llms-txt", Name: "llms.txt present", Tier: "essential", Result: "failed",
				Details: "no /llms.txt", Recommendation: "publish an llms.txt"},
			{ID: "mcp-server-card", Name: "MCP server card", Tier: "recommended", Result: "partial"},
		},
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshal is-agentic report: %v", err)
	}
	return b
}

// TestRenderScanForRoutesIsAgenticToNativeRenderer is the regression test for
// `check <url> --source is-agentic` in HUMAN mode. The bug: check passed the
// is-agentic body to the level-based renderer, and store.ParseReport unmarshals
// it WITHOUT error into an all-zero Report — so the command printed an empty
// URL, "Level 0" and zero checks instead of the score and issues.
//
// The live dogfood matrix missed this because it only ran `report --source
// is-agentic`, never `check`. Assert on rendered CONTENT (the real score and a
// real issue id must appear, "Level 0" must not), not on output length: a
// length assertion passes happily on the all-zero level summary.
func TestRenderScanForRoutesIsAgenticToNativeRenderer(t *testing.T) {
	raw := agenticRawReport(t)

	t.Run("is-agentic human output uses the score renderer", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&buf)
		flags := &rootFlags{source: store.SourceIsAgentic, plain: true}
		if err := renderScanFor(cmd, flags, raw, true); err != nil {
			t.Fatalf("renderScanFor: %v", err)
		}
		got := buf.String()
		for _, want := range []string{
			"https://render-test.example",
			"Score: 73",
			"Mostly ready",
			"Eligible checks: 31",
			"Essential: 8/11 passing",
			"llms-txt",            // a real issue id must be listed
			"mcp-server-card",     // ...including the partial one
			"publish an llms.txt", // ...and its recommendation
		} {
			if !strings.Contains(got, want) {
				t.Errorf("is-agentic human output missing %q\n--- got ---\n%s", want, got)
			}
		}
		// The level renderer's fingerprint must be absent entirely. Match on
		// its literal phrasings, not bare "0 pass" — that substring also
		// occurs inside the legitimate "12/20 passing" tier line.
		for _, bad := range []string{"Level 0", "Level ", "neutral (of", "Checks:"} {
			if strings.Contains(got, bad) {
				t.Errorf("is-agentic human output leaked level-renderer text %q\n--- got ---\n%s", bad, got)
			}
		}
	})

	t.Run("isitagentready human output still uses the level renderer", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&buf)
		flags := &rootFlags{source: store.SourceIsItAgentReady, plain: true}
		levelRaw := rawRep("https://level-test.example", "2026-08-20T09:00:00Z", 3,
			map[string]string{"sitemap": "pass", "llmsTxt": "fail"}, nil)
		if err := renderScanFor(cmd, flags, levelRaw, true); err != nil {
			t.Fatalf("renderScanFor: %v", err)
		}
		got := buf.String()
		for _, want := range []string{"https://level-test.example", "Level 3", "1 pass", "1 fail"} {
			if !strings.Contains(got, want) {
				t.Errorf("isitagentready human output missing %q\n--- got ---\n%s", want, got)
			}
		}
	})

	t.Run("machine output stays raw for both sources", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&buf)
		flags := &rootFlags{source: store.SourceIsAgentic, asJSON: true}
		if err := renderScanFor(cmd, flags, raw, false); err != nil {
			t.Fatalf("renderScanFor: %v", err)
		}
		var back store.AgenticReport
		if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
			t.Fatalf("machine output is not the raw report: %v (%s)", err, buf.String())
		}
		if back.Score != 73 || back.Target != "https://render-test.example" {
			t.Fatalf("machine output lost fields: score=%d target=%q", back.Score, back.Target)
		}
	})
}
