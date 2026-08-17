// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

const testURL = "https://behavior-test.example"

func TestGateBehavior(t *testing.T) {
	t.Run("fails below min-level", func(t *testing.T) {
		home := seedStore(t, sampleRec(testURL, "2026-06-22T10:00:00Z", 1, map[string]string{"a": "fail"}, nil))
		out, _, err := runCLI(t, home, "gate", testURL, "--min-level", "3", "--agent", "--data-source", "local")
		if err == nil || ExitCode(err) != 1 {
			t.Fatalf("want exit 1, got err=%v out=%s", err, out)
		}
		var res map[string]any
		if e := json.Unmarshal([]byte(out), &res); e != nil {
			t.Fatalf("stdout not JSON: %v (%s)", e, out)
		}
		if res["pass"] != false {
			t.Fatalf("pass=%v, want false", res["pass"])
		}
	})
	t.Run("passes at min-level", func(t *testing.T) {
		home := seedStore(t, sampleRec(testURL, "2026-06-22T10:00:00Z", 4, map[string]string{"a": "pass"}, nil))
		if _, _, err := runCLI(t, home, "gate", testURL, "--min-level", "3", "--agent", "--data-source", "local"); err != nil {
			t.Fatalf("want pass, got %v", err)
		}
	})
	t.Run("no-regress detects a regression", func(t *testing.T) {
		home := seedStore(t,
			sampleRec(testURL, "2026-06-20T10:00:00Z", 2, map[string]string{"a": "pass"}, nil),
			sampleRec(testURL, "2026-06-22T10:00:00Z", 2, map[string]string{"a": "fail"}, nil),
		)
		out, _, err := runCLI(t, home, "gate", testURL, "--no-regress", "--agent", "--data-source", "local")
		if err == nil || ExitCode(err) != 1 {
			t.Fatalf("want exit 1 on regression, got err=%v out=%s", err, out)
		}
	})
}

func TestDiffBehavior(t *testing.T) {
	home := seedStore(t,
		sampleRec(testURL, "2026-06-20T10:00:00Z", 1, map[string]string{"a": "fail", "b": "pass"}, nil),
		sampleRec(testURL, "2026-06-22T10:00:00Z", 2, map[string]string{"a": "pass", "b": "pass"}, nil),
	)
	out, _, err := runCLI(t, home, "diff", testURL, "--agent", "--data-source", "local")
	if err != nil {
		t.Fatalf("diff err=%v out=%s", err, out)
	}
	var res struct {
		LevelDelta int `json:"levelDelta"`
		Changes    []struct {
			Check, Change string
		} `json:"changes"`
	}
	if e := json.Unmarshal([]byte(out), &res); e != nil {
		t.Fatalf("not JSON: %v (%s)", e, out)
	}
	if res.LevelDelta != 1 {
		t.Fatalf("levelDelta=%d, want 1", res.LevelDelta)
	}
	found := false
	for _, c := range res.Changes {
		if c.Check == "a" && c.Change == "improved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected check a improved, got %+v", res.Changes)
	}
}

func TestHistoryBehavior(t *testing.T) {
	home := seedStore(t,
		sampleRec(testURL, "2026-06-20T10:00:00Z", 1, map[string]string{"a": "fail"}, nil),
		sampleRec(testURL, "2026-06-22T10:00:00Z", 3, map[string]string{"a": "pass"}, nil),
	)
	out, _, err := runCLI(t, home, "history", testURL, "--agent", "--data-source", "local")
	if err != nil {
		t.Fatalf("history err=%v out=%s", err, out)
	}
	var entries []struct {
		Level int                              `json:"level"`
		Flips []struct{ Check, Change string } `json:"flips"`
	}
	if e := json.Unmarshal([]byte(out), &entries); e != nil {
		t.Fatalf("not JSON: %v (%s)", e, out)
	}
	if len(entries) != 2 {
		t.Fatalf("entries=%d, want 2", len(entries))
	}
	if len(entries[1].Flips) != 1 || entries[1].Flips[0].Check != "a" {
		t.Fatalf("expected one flip on a, got %+v", entries[1].Flips)
	}
}

func TestOpenAdviceBehavior(t *testing.T) {
	home := seedStore(t,
		sampleRec("https://a.example", "2026-06-22T10:00:00Z", 1, map[string]string{"robotsTxt": "fail"}, map[string]string{"robotsTxt": "Create /robots.txt"}),
		sampleRec("https://b.example", "2026-06-22T10:00:00Z", 4, map[string]string{"mcpServerCard": "fail"}, map[string]string{"mcpServerCard": "Add an MCP server card"}),
	)
	out, _, err := runCLI(t, home, "open-advice", "--agent")
	if err != nil {
		t.Fatalf("open-advice err=%v out=%s", err, out)
	}
	var items []struct{ URL, Check, Prompt string }
	if e := json.Unmarshal([]byte(out), &items); e != nil {
		t.Fatalf("not JSON: %v (%s)", e, out)
	}
	if len(items) != 2 {
		t.Fatalf("items=%d, want 2", len(items))
	}
	// --check filter
	out2, _, err := runCLI(t, home, "open-advice", "--check", "mcpServerCard", "--agent")
	if err != nil {
		t.Fatalf("open-advice --check err=%v", err)
	}
	var filtered []struct{ Check string }
	_ = json.Unmarshal([]byte(out2), &filtered)
	if len(filtered) != 1 || filtered[0].Check != "mcpServerCard" {
		t.Fatalf("filtered=%+v, want one mcpServerCard", filtered)
	}
}

func TestCompareBehavior(t *testing.T) {
	home := seedStore(t,
		sampleRec("https://a.example", "2026-06-22T10:00:00Z", 1, map[string]string{"robotsTxt": "pass", "mcpServerCard": "fail"}, nil),
		sampleRec("https://b.example", "2026-06-22T10:00:00Z", 4, map[string]string{"robotsTxt": "pass", "mcpServerCard": "pass"}, nil),
	)
	out, _, err := runCLI(t, home, "compare", "https://a.example", "https://b.example", "--agent", "--data-source", "local")
	if err != nil {
		t.Fatalf("compare err=%v out=%s", err, out)
	}
	var res struct {
		Sites  []struct{ URL string } `json:"sites"`
		Checks []struct {
			Check    string            `json:"check"`
			Statuses map[string]string `json:"statuses"`
		} `json:"checks"`
	}
	if e := json.Unmarshal([]byte(out), &res); e != nil {
		t.Fatalf("not JSON: %v (%s)", e, out)
	}
	if len(res.Sites) != 2 {
		t.Fatalf("sites=%d, want 2", len(res.Sites))
	}
	for _, c := range res.Checks {
		if c.Check == "mcpServerCard" {
			if c.Statuses["a.example"] != "fail" || c.Statuses["b.example"] != "pass" {
				t.Fatalf("mcpServerCard statuses=%v", c.Statuses)
			}
		}
	}
}

func TestBatchBehaviorNonNetwork(t *testing.T) {
	home := seedStore(t)
	t.Run("invalid rank is a usage error", func(t *testing.T) {
		_, _, err := runCLI(t, home, "batch", "/nonexistent.txt", "--rank", "bogus", "--agent")
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("want usage error exit 2, got %v", err)
		}
	})
	t.Run("dry-run does not scan", func(t *testing.T) {
		if _, _, err := runCLI(t, home, "batch", "urls.txt", "--dry-run"); err != nil {
			t.Fatalf("dry-run err=%v", err)
		}
	})
}

func TestCheckLocalAndAdvice(t *testing.T) {
	home := seedStore(t, sampleRec(testURL, "2026-06-22T10:00:00Z", 1, map[string]string{"robotsTxt": "fail"}, map[string]string{"robotsTxt": "Create /robots.txt at the root."}))

	t.Run("check --data-source local renders stored scan", func(t *testing.T) {
		out, _, err := runCLI(t, home, "check", testURL, "--agent", "--data-source", "local")
		if err != nil {
			t.Fatalf("check err=%v out=%s", err, out)
		}
		var rep map[string]any
		if e := json.Unmarshal([]byte(out), &rep); e != nil {
			t.Fatalf("not JSON: %v (%s)", e, out)
		}
		if rep["url"] != testURL {
			t.Fatalf("url=%v, want %s", rep["url"], testURL)
		}
	})

	t.Run("advice surfaces the fix prompt", func(t *testing.T) {
		out, _, err := runCLI(t, home, "advice", testURL, "--agent", "--data-source", "local")
		if err != nil {
			t.Fatalf("advice err=%v out=%s", err, out)
		}
		if !strings.Contains(out, "robotsTxt") || !strings.Contains(out, "Create /robots.txt") {
			t.Fatalf("advice missing prompt: %s", out)
		}
	})

	t.Run("advice --copy prints a plain block", func(t *testing.T) {
		out, _, err := runCLI(t, home, "advice", testURL, "--copy", "--data-source", "local")
		if err != nil {
			t.Fatalf("advice --copy err=%v", err)
		}
		if !strings.Contains(out, "Create /robots.txt") {
			t.Fatalf("copy block missing prompt: %s", out)
		}
	})

	t.Run("report --only-failing keeps only failures", func(t *testing.T) {
		out, _, err := runCLI(t, home, "report", testURL, "--only-failing", "--agent", "--data-source", "local")
		if err != nil {
			t.Fatalf("report err=%v out=%s", err, out)
		}
		if !strings.Contains(out, "robotsTxt") {
			t.Fatalf("report missing failing check: %s", out)
		}
	})
}
