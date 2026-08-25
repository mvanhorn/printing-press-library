// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"strings"
	"testing"
)

// vercelBody is the literal upstream is-agentic.com report body for vercel.com
// (verified, do not alter). The float Earned/Available/Points fields are the
// shape bug most likely to slip — 63.5 / 17.9 / 5 must decode as float64.
const vercelBody = `{"score":86,"score_label":"Strong technical baseline","eligible_checks":33,
 "score_breakdown":{
   "essential":{"earned":63.5,"available":80,"passing":8,"total":11},
   "recommended":{"earned":17.9,"available":20,"passing":17,"total":22},
   "bonus":{"points":5,"positive_signals":38}},
 "issues":[{"id":"agent-friendly-404","name":"Agent-friendly 404s","tier":"essential",
            "result":"failed","details":"...","recommendation":"..."}]}`

func TestParseAgenticReportFloats(t *testing.T) {
	r, err := ParseAgenticReport(json.RawMessage(vercelBody))
	if err != nil {
		t.Fatalf("ParseAgenticReport: %v", err)
	}
	if r.Score != 86 || r.ScoreLabel != "Strong technical baseline" || r.EligibleChecks != 33 {
		t.Fatalf("headline = %d/%q/%d", r.Score, r.ScoreLabel, r.EligibleChecks)
	}
	if r.ScoreBreakdown.Essential.Earned != 63.5 {
		t.Fatalf("essential.earned = %v, want 63.5 (float)", r.ScoreBreakdown.Essential.Earned)
	}
	if r.ScoreBreakdown.Recommended.Earned != 17.9 {
		t.Fatalf("recommended.earned = %v, want 17.9 (float)", r.ScoreBreakdown.Recommended.Earned)
	}
	if r.ScoreBreakdown.Bonus.Points != 5 {
		t.Fatalf("bonus.points = %v, want 5 (float)", r.ScoreBreakdown.Bonus.Points)
	}
	if r.ScoreBreakdown.Essential.Passing != 8 || r.ScoreBreakdown.Essential.Total != 11 {
		t.Fatalf("essential counts = %d/%d", r.ScoreBreakdown.Essential.Passing, r.ScoreBreakdown.Essential.Total)
	}
	if r.ScoreBreakdown.Essential.Available != 80 {
		t.Fatalf("essential.available = %v, want 80", r.ScoreBreakdown.Essential.Available)
	}
}

func TestAgenticStatus(t *testing.T) {
	body := `{"score":50,"score_label":"x","eligible_checks":3,"scanned_at":"2026-06-22T10:00:00Z",
	 "score_breakdown":{},
	 "issues":[
	   {"id":"a","name":"a","tier":"essential","result":"failed","details":"","recommendation":"fix a"},
	   {"id":"b","name":"b","tier":"recommended","result":"partial","details":"","recommendation":"fix b"}]}`
	r, err := ParseAgenticReport(json.RawMessage(body))
	if err != nil {
		t.Fatalf("ParseAgenticReport: %v", err)
	}
	if got := r.AgenticStatus("a"); got != "fail" {
		t.Fatalf("AgenticStatus(a) = %q, want fail", got)
	}
	if got := r.AgenticStatus("b"); got != "partial" {
		t.Fatalf("AgenticStatus(b) = %q, want partial", got)
	}
	if got := r.AgenticStatus("c"); got != "pass" {
		t.Fatalf("AgenticStatus(c) absent = %q, want pass", got)
	}
}

func TestAgenticOpenItems(t *testing.T) {
	body := `{"target":"https://vercel.com","score_label":"Strong technical baseline","score":86,"eligible_checks":33,
	 "score_breakdown":{},"issues":[
	   {"id":"agent-friendly-404","name":"Agent-friendly 404s","tier":"essential","result":"failed","details":"no 404 page","recommendation":"Add an agent-friendly 404"},
	   {"id":"mcp-server","name":"MCP server","tier":"recommended","result":"partial","details":"","recommendation":"Expose an MCP server"}]}`
	r, err := ParseAgenticReport(json.RawMessage(body))
	if err != nil {
		t.Fatalf("ParseAgenticReport: %v", err)
	}
	items := AgenticOpenItems(r)
	if len(items) != 2 {
		t.Fatalf("AgenticOpenItems = %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.Source != SourceIsAgentic {
			t.Fatalf("Source = %q, want is-agentic", it.Source)
		}
		if it.Score == nil || *it.Score != 86 {
			t.Fatalf("Score = %v, want 86", it.Score)
		}
		if it.ScoreLabel != "Strong technical baseline" {
			t.Fatalf("ScoreLabel = %q", it.ScoreLabel)
		}
		if it.Level != 0 || it.LevelName != "" {
			t.Fatalf("is-agentic must leave Level 0/empty, got %d/%q", it.Level, it.LevelName)
		}
	}
	if items[0].Check != "agent-friendly-404" || items[0].Prompt != "Add an agent-friendly 404" {
		t.Fatalf("item[0] = %+v", items[0])
	}
	if items[0].Description != "Agent-friendly 404s — no 404 page" {
		t.Fatalf("Description with details = %q", items[0].Description)
	}
	if items[1].Check != "mcp-server" || items[1].Description != "MCP server" {
		t.Fatalf("item[1] = %+v", items[1])
	}
}

func TestFailingIssuesSorted(t *testing.T) {
	body := `{"target":"x","score":10,"score_label":"x","eligible_checks":2,"score_breakdown":{},
	 "issues":[
	   {"id":"z","tier":"recommended","result":"failed"},
	   {"id":"a","tier":"essential","result":"failed"},
	   {"id":"p","tier":"essential","result":"pass"}]}`
	r, err := ParseAgenticReport(json.RawMessage(body))
	if err != nil {
		t.Fatalf("ParseAgenticReport: %v", err)
	}
	issues := r.FailingIssues()
	if len(issues) != 2 {
		t.Fatalf("FailingIssues = %d, want 2 (pass excluded)", len(issues))
	}
	if issues[0].ID != "a" || issues[1].ID != "z" {
		t.Fatalf("FailingIssues order = %s,%s; want essential-then-id", issues[0].ID, issues[1].ID)
	}
}

func TestBuildCrossRefVerdicts(t *testing.T) {
	mkIar := func(statuses map[string]string) *Report {
		rep, _ := ParseReport(rawReport("https://x.com", "2026-06-22T10:00:00Z", 2, "L", statuses, nil))
		return rep
	}
	mkAg := func(m map[string]string, scannedAt string) *AgenticReport {
		ag := &AgenticReport{Target: "https://x.com", Score: 50, ScoreLabel: "x", ScannedAt: scannedAt}
		for id, res := range m {
			ag.Issues = append(ag.Issues, AgenticIssue{ID: id, Tier: "essential", Result: res})
		}
		return ag
	}

	cases := []struct {
		name    string
		iar     *Report
		ag      *AgenticReport
		label   string // cross-pair label we assert on
		verdict string
	}{
		// sitemap: iar pass (2), ag fail (0) -> diff 2 -> disagree
		{"disagree", mkIar(map[string]string{"sitemap": "pass"}), mkAg(map[string]string{"sitemap": "failed"}, "2026-06-22T10:00:00Z"), "Sitemap published", "disagree"},
		// markdownNegotiation: iar neutral (1), ag pass (2) -> diff 1 -> partial
		{"partial", mkIar(map[string]string{"markdownNegotiation": "neutral"}), mkAg(map[string]string{"markdown-negotiation-vary": "pass"}, "2026-06-22T10:00:00Z"), "Markdown content negotiation", "partial"},
		// mcpServerCard: iar pass (2), ag partial (1) -> diff 1 -> partial
		{"partial2", mkIar(map[string]string{"mcpServerCard": "pass"}), mkAg(map[string]string{"mcp-server": "partial"}, "2026-06-22T10:00:00Z"), "MCP server manifest", "partial"},
		// oauthDiscovery: iar fail (0), ag fail (0) -> diff 0 -> agree
		{"agree", mkIar(map[string]string{"oauthDiscovery": "fail"}), mkAg(map[string]string{"oauth-support": "failed"}, "2026-06-22T10:00:00Z"), "OAuth 2.0 discovery", "agree"},
		// mcpServerCard: iar lacks the check (so its status is "-") while ag
		// has the pair -> the is-agentic side has data but isitagentready does
		// not, so verdict is unknown.
		{"unknown", mkIar(map[string]string{"sitemap": "pass"}), mkAg(map[string]string{"mcp-server": "failed"}, "2026-06-22T10:00:00Z"), "MCP server manifest", "unknown"},
		// ag nil -> all unknown
		{"ag-nil", mkIar(map[string]string{"sitemap": "pass"}), nil, "Sitemap published", "unknown"},
		// iar nil -> all unknown
		{"iar-nil", nil, mkAg(map[string]string{"sitemap": "failed"}, "2026-06-22T10:00:00Z"), "Sitemap published", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := BuildCrossRef("https://x.com", c.iar, c.ag)
			for _, o := range res.Overlap {
				if o.Label == c.label {
					if o.Verdict != c.verdict {
						t.Fatalf("verdict for %q = %q, want %q (statuses %q / %q)", o.Label, o.Verdict, c.verdict, o.IsItAgentReadyStatus, o.IsAgenticStatus)
					}
					return
				}
			}
			t.Fatalf("overlap pair %q not found in %+v", c.label, res.Overlap)
		})
	}
}

func TestBuildCrossRefNotes(t *testing.T) {
	iar, _ := ParseReport(rawReport("https://x.com", "2026-06-22T10:00:00Z", 2, "L", map[string]string{"sitemap": "pass"}, nil))
	ag := &AgenticReport{Target: "https://x.com", Score: 50, ScoreLabel: "x", ScannedAt: "2026-06-20T10:00:00Z"}
	res := BuildCrossRef("https://x.com", iar, ag)

	joined := strings.Join(res.Notes, "\n")
	// The not-comparable note must always be present and must actually name
	// why the two scales differ, not merely be a long string.
	for _, want := range []string{"Level 0-5", "score 0-100", "never merged"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("scale note missing %q; notes = %q", want, res.Notes)
		}
	}
	// 2026-06-22 vs 2026-06-20 = 48h apart -> staleness note naming the gap.
	if !strings.Contains(joined, "48h") {
		t.Fatalf("expected a staleness note naming the 48h gap, got %q", res.Notes)
	}
}

// TestBuildCrossRefNotesFreshNoStaleness pins the negative case: scans close in
// time must NOT produce a staleness note, otherwise the note is noise.
func TestBuildCrossRefNotesFreshNoStaleness(t *testing.T) {
	iar, _ := ParseReport(rawReport("https://x.com", "2026-06-22T10:00:00Z", 2, "L", map[string]string{"sitemap": "pass"}, nil))
	ag := &AgenticReport{Target: "https://x.com", Score: 50, ScoreLabel: "x", ScannedAt: "2026-06-22T12:00:00Z"}
	res := BuildCrossRef("https://x.com", iar, ag)
	for _, n := range res.Notes {
		if strings.Contains(n, "apart") {
			t.Fatalf("did not expect a staleness note for a 2h gap, got %q", n)
		}
	}
}

// TestBuildCrossRefStalenessIsBidirectional pins that an is-agentic report
// NEWER than the isitagentready one still trips the staleness note — the gap
// matters in both directions.
func TestBuildCrossRefStalenessIsBidirectional(t *testing.T) {
	iar, _ := ParseReport(rawReport("https://x.com", "2026-06-20T10:00:00Z", 2, "L", map[string]string{"sitemap": "pass"}, nil))
	ag := &AgenticReport{Target: "https://x.com", Score: 50, ScoreLabel: "x", ScannedAt: "2026-06-22T10:00:00Z"}
	res := BuildCrossRef("https://x.com", iar, ag)
	if !strings.Contains(strings.Join(res.Notes, "\n"), "48h") {
		t.Fatalf("expected a staleness note when is-agentic is the newer scan, got %q", res.Notes)
	}
}

func TestFilterSourceAndSourceOrDefault(t *testing.T) {
	recs := []ScanRecord{
		{URL: "https://old.com", ScannedAt: "t", Level: 2, LevelName: "L", Raw: json.RawMessage(`{}`), Source: ""}, // pre-amend
		{URL: "https://ag.com", Source: SourceIsAgentic, ScannedAt: "t", Score: intPtr(70), Raw: json.RawMessage(`{}`)},
	}
	iar := FilterSource(recs, SourceIsItAgentReady)
	if len(iar) != 1 || NormalizeURL(iar[0].URL) != "old.com" {
		t.Fatalf("isitagentready filter = %+v", iar)
	}
	ag := FilterSource(recs, SourceIsAgentic)
	if len(ag) != 1 || NormalizeURL(ag[0].URL) != "ag.com" {
		t.Fatalf("is-agentic filter = %+v", ag)
	}
	// SourceOrDefault backfills empty to isitagentready.
	if recs[0].SourceOrDefault() != SourceIsItAgentReady {
		t.Fatalf("SourceOrDefault empty = %q", recs[0].SourceOrDefault())
	}
	if recs[1].SourceOrDefault() != SourceIsAgentic {
		t.Fatalf("SourceOrDefault is-agentic = %q", recs[1].SourceOrDefault())
	}
}

func intPtr(v int) *int { return &v }

func TestAssertSameSource(t *testing.T) {
	a := ScanRecord{URL: "a", Source: SourceIsItAgentReady}
	b := ScanRecord{URL: "b", Source: SourceIsItAgentReady}
	if err := AssertSameSource(a, b); err != nil {
		t.Fatalf("same source errored: %v", err)
	}
	c := ScanRecord{URL: "c", Source: SourceIsAgentic}
	if err := AssertSameSource(a, c); err == nil {
		t.Fatalf("different sources did not error")
	}
}

// agenticRec builds an is-agentic ScanRecord for store-level ranking tests.
func agenticRec(url, at string, score int, issues []struct {
	ID, Tier, Result string
}) ScanRecord {
	ag := &AgenticReport{Target: url, Score: score, ScoreLabel: "L", ScannedAt: at}
	for _, is := range issues {
		ag.Issues = append(ag.Issues, AgenticIssue{ID: is.ID, Tier: is.Tier, Result: is.Result})
	}
	b, _ := json.Marshal(ag)
	return ScanRecord{URL: url, Source: SourceIsAgentic, ScannedAt: at, Score: &score, Raw: b}
}

func TestEvaluateAgenticGate(t *testing.T) {
	mk := func(score int, issues ...AgenticIssue) *AgenticReport {
		return &AgenticReport{Target: "https://x.com", Score: score, ScoreLabel: "x", Issues: issues}
	}
	t.Run("below score fails", func(t *testing.T) {
		res := EvaluateAgenticGate(mk(50), nil, 60, false)
		if res.Pass {
			t.Fatalf("expected fail for score 50 < min 60, got %+v", res)
		}
		if res.Level != 0 {
			t.Fatalf("is-agentic gate must leave Level 0, got %d", res.Level)
		}
		if res.Score == nil || *res.Score != 50 {
			t.Fatalf("Score = %v, want 50", res.Score)
		}
	})
	t.Run("meets score passes", func(t *testing.T) {
		res := EvaluateAgenticGate(mk(80), nil, 60, false)
		if !res.Pass {
			t.Fatalf("expected pass, got %+v", res)
		}
	})
	t.Run("no-regress catches a passing->failing check", func(t *testing.T) {
		// prev has no "x" issue -> passing. latest has "x" failed -> regressed.
		prev := mk(70)
		latest := mk(70, AgenticIssue{ID: "x", Tier: "essential", Result: "failed"})
		res := EvaluateAgenticGate(latest, prev, 0, true)
		if res.Pass {
			t.Fatalf("expected fail under no-regress, got %+v", res)
		}
		if len(res.Regressions) != 1 || res.Regressions[0] != "x" {
			t.Fatalf("Regressions = %v, want [x]", res.Regressions)
		}
	})
	t.Run("no-regress ignores an improving check", func(t *testing.T) {
		prev := mk(70, AgenticIssue{ID: "x", Tier: "essential", Result: "failed"})
		latest := mk(80) // x now absent -> passing
		res := EvaluateAgenticGate(latest, prev, 0, true)
		if !res.Pass {
			t.Fatalf("expected pass (x improved), got %+v", res)
		}
	})
}

func TestDiffAgenticIssues(t *testing.T) {
	fromNoIssues := &AgenticReport{Target: "u", Issues: nil}
	fromFail := &AgenticReport{Target: "u", Issues: []AgenticIssue{{ID: "x", Tier: "essential", Result: "failed"}}}
	toPass := &AgenticReport{Target: "u", Issues: nil}

	// failed -> absent: x goes from failing to passing -> improved.
	tr := DiffAgenticIssues(fromFail, toPass)
	foundImproved := false
	for _, c := range tr {
		if c.Check == "x" && c.Change == "improved" {
			foundImproved = true
		}
	}
	if !foundImproved {
		t.Fatalf("expected x improved (failed -> absent), got %+v", tr)
	}

	// absent -> failed: x goes from passing to failing -> regressed.
	tr2 := DiffAgenticIssues(fromNoIssues, fromFail)
	foundRegressed := false
	for _, c := range tr2 {
		if c.Check == "x" && c.Change == "regressed" {
			foundRegressed = true
		}
	}
	if !foundRegressed {
		t.Fatalf("expected x regressed (absent -> failed), got %+v", tr2)
	}
}

func TestRankRecordsAgenticByScore(t *testing.T) {
	recs := []ScanRecord{
		agenticRec("https://mid.example", "t", 60, nil),
		agenticRec("https://bad.example", "t", 40, nil),
		agenticRec("https://good.example", "t", 80, nil),
	}
	ranked := RankRecords(recs, "level") // "level" maps to score asc worst-first for agentic
	if ranked[0].URL != "https://bad.example" || ranked[1].URL != "https://mid.example" || ranked[2].URL != "https://good.example" {
		t.Fatalf("RankRecords(agentic, level) order = %v, %v, %v; want bad, mid, good",
			ranked[0].URL, ranked[1].URL, ranked[2].URL)
	}
}

func TestRankRecordsAgenticByFailing(t *testing.T) {
	// bad has 2 non-passing, mid 1, good 0.
	mkIss := func(ids []string) []struct{ ID, Tier, Result string } {
		var out []struct{ ID, Tier, Result string }
		for _, id := range ids {
			out = append(out, struct{ ID, Tier, Result string }{id, "essential", "failed"})
		}
		return out
	}
	recs := []ScanRecord{
		agenticRec("https://mid.example", "t", 60, mkIss([]string{"a"})),
		agenticRec("https://good.example", "t", 90, nil),
		agenticRec("https://bad.example", "t", 40, mkIss([]string{"a", "b"})),
	}
	ranked := RankRecords(recs, "failing")
	if ranked[0].URL != "https://bad.example" || ranked[1].URL != "https://mid.example" || ranked[2].URL != "https://good.example" {
		t.Fatalf("RankRecords(agentic, failing) order = %v, %v, %v",
			ranked[0].URL, ranked[1].URL, ranked[2].URL)
	}
}
