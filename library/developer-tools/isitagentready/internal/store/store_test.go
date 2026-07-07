// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// rawReport builds a raw scan-report JSON with the given top-level level and a
// flat checks map (checkID -> status) placed under a single "test" category,
// plus optional next-level requirements (checkID -> prompt).
func rawReport(url, scannedAt string, level int, levelName string, checks map[string]string, reqs map[string]string) json.RawMessage {
	cat := map[string]Check{}
	for id, st := range checks {
		cat[id] = Check{Status: st, Message: id + " " + st}
	}
	rep := Report{
		URL:       url,
		ScannedAt: scannedAt,
		Level:     level,
		LevelName: levelName,
		Checks:    map[string]map[string]Check{"discovery": cat},
	}
	for id, prompt := range reqs {
		rep.NextLevel.Requirements = append(rep.NextLevel.Requirements, Requirement{
			Check: id, Description: "fix " + id, Prompt: prompt, SkillURL: "https://isitagentready.com/.well-known/agent-skills/" + id + "/SKILL.md",
		})
	}
	b, _ := json.Marshal(rep)
	return b
}

func rec(url, at string, level int, checks map[string]string, reqs map[string]string) ScanRecord {
	return ScanRecord{URL: url, ScannedAt: at, Level: level, LevelName: "L", Raw: rawReport(url, at, level, "L", checks, reqs)}
}

func TestParseReportAndFailingChecks(t *testing.T) {
	raw := rawReport("https://ex.com", "2026-06-22T10:00:00Z", 2, "Bot-Aware",
		map[string]string{"robotsTxt": "pass", "sitemap": "fail", "dnsAid": "neutral", "mcpServerCard": "fail"}, nil)
	rep, err := ParseReport(raw)
	if err != nil {
		t.Fatalf("ParseReport: %v", err)
	}
	if rep.Level != 2 || rep.LevelName != "Bot-Aware" {
		t.Fatalf("level/name = %d/%q", rep.Level, rep.LevelName)
	}
	failing := rep.FailingChecks()
	want := []string{"mcpServerCard", "sitemap"}
	if len(failing) != len(want) || failing[0] != want[0] || failing[1] != want[1] {
		t.Fatalf("FailingChecks = %v, want %v", failing, want)
	}
	pass, fail, neutral, total := rep.Counts()
	if pass != 1 || fail != 2 || neutral != 1 || total != 4 {
		t.Fatalf("Counts = %d/%d/%d/%d", pass, fail, neutral, total)
	}
}

func TestParseReportSiteError(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://x","scannedAt":"t","siteError":{"httpStatus":403,"statusText":"Forbidden","server":"cloudflare"}}`)
	rep, err := ParseReport(raw)
	if err != nil {
		t.Fatalf("ParseReport: %v", err)
	}
	if rep.SiteError == nil || rep.SiteError.HTTPStatus != 403 {
		t.Fatalf("siteError not parsed: %+v", rep.SiteError)
	}
}

func TestNormalizeAndMatchURL(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://example.com", "example.com", true},
		{"https://example.com/", "http://example.com", true},
		{"https://Example.com", "example.com", true},
		{"https://example.com", "https://other.com", false},
	}
	for _, c := range cases {
		if got := MatchURL(c.a, c.b); got != c.want {
			t.Errorf("MatchURL(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestHistoryLatestAndLatestPerURL(t *testing.T) {
	recs := []ScanRecord{
		rec("https://a.com", "2026-06-20T10:00:00Z", 1, map[string]string{"x": "fail"}, nil),
		rec("https://a.com", "2026-06-22T10:00:00Z", 3, map[string]string{"x": "pass"}, nil),
		rec("https://b.com", "2026-06-21T10:00:00Z", 2, map[string]string{"y": "pass"}, nil),
	}
	hist := HistoryFor(recs, "a.com")
	if len(hist) != 2 || hist[0].Level != 1 || hist[1].Level != 3 {
		t.Fatalf("HistoryFor ordering wrong: %+v", hist)
	}
	latest, ok := Latest(recs, "https://a.com/")
	if !ok || latest.Level != 3 {
		t.Fatalf("Latest = %+v ok=%v", latest, ok)
	}
	per := LatestPerURL(recs)
	if len(per) != 2 {
		t.Fatalf("LatestPerURL count = %d, want 2", len(per))
	}
	if per[0].URL != "https://a.com" || per[0].Level != 3 {
		t.Fatalf("LatestPerURL[0] = %+v", per[0])
	}
}

func TestDiffChecks(t *testing.T) {
	from, _ := ParseReport(rawReport("u", "t1", 2, "L", map[string]string{
		"keep": "pass", "regress": "pass", "improve": "fail", "gone": "pass",
	}, nil))
	to, _ := ParseReport(rawReport("u", "t2", 2, "L", map[string]string{
		"keep": "pass", "regress": "fail", "improve": "pass", "added": "fail",
	}, nil))
	got := map[string]string{}
	for _, tr := range DiffChecks(from, to) {
		got[tr.Check] = tr.Change
	}
	want := map[string]string{"keep": "unchanged", "regress": "regressed", "improve": "improved", "gone": "removed", "added": "added"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("transition[%s] = %q, want %q", k, got[k], v)
		}
	}
}

func TestEvaluateGate(t *testing.T) {
	pass2, _ := ParseReport(rawReport("u", "t1", 2, "Bot-Aware", map[string]string{"a": "pass", "b": "pass"}, nil))
	now1, _ := ParseReport(rawReport("u", "t2", 1, "Basic", map[string]string{"a": "fail", "b": "pass"}, nil))
	siteErr, _ := ParseReport(json.RawMessage(`{"url":"u","level":0,"siteError":{"httpStatus":403,"statusText":"Forbidden"}}`))

	t.Run("below min-level fails", func(t *testing.T) {
		r := EvaluateGate(now1, nil, 3, false, false)
		if r.Pass {
			t.Fatalf("expected fail, got %+v", r)
		}
	})
	t.Run("meets min-level passes", func(t *testing.T) {
		r := EvaluateGate(pass2, nil, 2, false, false)
		if !r.Pass {
			t.Fatalf("expected pass, got %+v", r)
		}
	})
	t.Run("no-regress detects regression", func(t *testing.T) {
		r := EvaluateGate(now1, pass2, 0, true, false)
		if r.Pass || len(r.Regressions) != 1 || r.Regressions[0] != "a" {
			t.Fatalf("expected regression on a, got %+v", r)
		}
	})
	t.Run("siteError does not fail without strict", func(t *testing.T) {
		r := EvaluateGate(siteErr, pass2, 3, true, false)
		if !r.Pass || !r.SiteError {
			t.Fatalf("expected pass+siteError, got %+v", r)
		}
	})
	t.Run("siteError fails with strict", func(t *testing.T) {
		r := EvaluateGate(siteErr, pass2, 3, true, true)
		if r.Pass {
			t.Fatalf("expected fail under strict, got %+v", r)
		}
	})
}

func TestOpenAdvice(t *testing.T) {
	recs := []ScanRecord{
		rec("https://a.com", "2026-06-22T10:00:00Z", 1, map[string]string{"robotsTxt": "fail"},
			map[string]string{"robotsTxt": "Create /robots.txt"}),
		rec("https://b.com", "2026-06-22T10:00:00Z", 4, map[string]string{"mcpServerCard": "fail"},
			map[string]string{"mcpServerCard": "Add an MCP server card"}),
	}
	all := OpenAdvice(LatestPerURL(recs), "", "")
	if len(all) != 2 {
		t.Fatalf("OpenAdvice all = %d, want 2", len(all))
	}
	bySite := OpenAdvice(LatestPerURL(recs), "a.com", "")
	if len(bySite) != 1 || bySite[0].Check != "robotsTxt" {
		t.Fatalf("OpenAdvice by site = %+v", bySite)
	}
	byCheck := OpenAdvice(LatestPerURL(recs), "", "mcpServerCard")
	if len(byCheck) != 1 || byCheck[0].URL != "https://b.com" {
		t.Fatalf("OpenAdvice by check = %+v", byCheck)
	}
}

func TestRankRecords(t *testing.T) {
	recs := []ScanRecord{
		rec("https://good.com", "t", 4, map[string]string{"a": "pass", "b": "pass"}, nil),
		rec("https://bad.com", "t", 0, map[string]string{"a": "fail", "b": "fail", "c": "fail"}, nil),
		rec("https://mid.com", "t", 2, map[string]string{"a": "pass", "b": "fail"}, nil),
	}
	byLevel := RankRecords(recs, "level")
	if byLevel[0].Level != 0 || byLevel[2].Level != 4 {
		t.Fatalf("RankRecords level worst-first wrong: %d..%d", byLevel[0].Level, byLevel[2].Level)
	}
	byFailing := RankRecords(recs, "failing")
	if NormalizeURL(byFailing[0].URL) != "bad.com" {
		t.Fatalf("RankRecords failing[0] = %s, want bad.com", byFailing[0].URL)
	}
}

func TestBuildHistory(t *testing.T) {
	hist := []ScanRecord{
		rec("u", "2026-06-20T10:00:00Z", 1, map[string]string{"a": "fail", "b": "pass"}, nil),
		rec("u", "2026-06-22T10:00:00Z", 2, map[string]string{"a": "pass", "b": "pass"}, nil),
	}
	entries := BuildHistory(hist, "")
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}
	if len(entries[0].Flips) != 0 {
		t.Fatalf("first entry should have no flips, got %+v", entries[0].Flips)
	}
	if len(entries[1].Flips) != 1 || entries[1].Flips[0].Check != "a" || entries[1].Flips[0].Change != "improved" {
		t.Fatalf("second entry flips wrong: %+v", entries[1].Flips)
	}
}

func TestBuildCompare(t *testing.T) {
	a, _ := ParseReport(rawReport("https://a.com", "t", 1, "L", map[string]string{"robotsTxt": "pass", "mcpServerCard": "fail"}, nil))
	b, _ := ParseReport(rawReport("https://b.com", "t", 4, "L", map[string]string{"robotsTxt": "pass", "mcpServerCard": "pass"}, nil))
	res := BuildCompare([]*Report{a, b})
	if len(res.Sites) != 2 || len(res.Checks) != 2 {
		t.Fatalf("compare shape: %d sites, %d checks", len(res.Sites), len(res.Checks))
	}
	var mcp *CompareRow
	for i := range res.Checks {
		if res.Checks[i].Check == "mcpServerCard" {
			mcp = &res.Checks[i]
		}
	}
	if mcp == nil || mcp.Statuses["a.com"] != "fail" || mcp.Statuses["b.com"] != "pass" {
		t.Fatalf("mcpServerCard row wrong: %+v", mcp)
	}
}

func TestAppendLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scans.jsonl")
	if got, err := Load(path); err != nil || got != nil {
		t.Fatalf("Load missing store = %v, %v", got, err)
	}
	r1 := rec("https://a.com", "2026-06-22T10:00:00Z", 1, map[string]string{"x": "fail"}, nil)
	r2 := rec("https://a.com", "2026-06-22T11:00:00Z", 3, map[string]string{"x": "pass"}, nil)
	if err := Append(path, r1); err != nil {
		t.Fatalf("Append r1: %v", err)
	}
	if err := Append(path, r2); err != nil {
		t.Fatalf("Append r2: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 || got[0].Level != 1 || got[1].Level != 3 {
		t.Fatalf("round trip = %+v", got)
	}
}
