// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

// TestNovelCategorizeSuggestHelpWires smoke-tests that the categorize suggest command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCategorizeSuggestHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"categorize", "suggest", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("categorize suggest --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "suggest"} {
		if !strings.Contains(help, want) {
			t.Fatalf("categorize suggest --help missing %q in output:\n%s", want, help)
		}
	}
}

type s3Suggestion struct {
	TransactionID       string  `json:"transaction_id"`
	Payee               string  `json:"payee"`
	SuggestedCategoryID string  `json:"suggested_category_id"`
	SuggestedCategory   string  `json:"suggested_category"`
	Confidence          float64 `json:"confidence"`
	HistoryCount        int     `json:"history_count"`
}

type s3ApplyResult struct {
	DryRun  bool `json:"dry_run"`
	Applied []struct {
		TransactionID string         `json:"transaction_id"`
		Method        string         `json:"method"`
		Path          string         `json:"path"`
		Body          map[string]any `json:"body"`
		Status        string         `json:"status"`
	} `json:"applied"`
	FetchFailures []struct {
		TransactionID string `json:"transaction_id"`
		Error         string `json:"error"`
	} `json:"fetch_failures"`
}

func TestCategorizeSuggestKrogerOnly(t *testing.T) {
	s3Fixture(t)
	var rows []s3Suggestion
	s3RunJSON(t, &rows, "categorize", "suggest")
	if len(rows) != 1 {
		t.Fatalf("suggestions = %+v, want only txn-kr-6", rows)
	}
	r := rows[0]
	if r.TransactionID != "txn-kr-6" || r.SuggestedCategoryID != "cat-groceries" || r.SuggestedCategory != "Groceries" || r.Confidence != 0.8 || r.HistoryCount != 5 {
		t.Fatalf("kr-6 = %+v, want Groceries 0.8 over 5", r)
	}
	// Negative: Chipotle (1 categorized row) and AMAZON MKTPLACE (no history) get nothing.
	for _, id := range []string{"txn-ch-2", "txn-dup-a", "txn-dup-b", "txn-am-2"} {
		for _, s := range rows {
			if s.TransactionID == id {
				t.Errorf("%s should have no suggestion at --min-history 2", id)
			}
		}
	}
}

func TestCategorizeSuggestThresholds(t *testing.T) {
	s3Fixture(t)
	var rows []s3Suggestion
	s3RunJSON(t, &rows, "categorize", "suggest", "--min-history", "1")
	if len(rows) != 4 {
		t.Fatalf("--min-history 1 = %+v, want kr-6 + 3 Chipotle rows", rows)
	}
	rows = nil
	s3RunJSON(t, &rows, "categorize", "suggest", "--min-confidence", "0.9")
	if len(rows) != 0 {
		t.Fatalf("--min-confidence 0.9 = %+v, want none", rows)
	}
	if _, _, err := s3Run(t, "categorize", "suggest", "--min-confidence", "1.5"); s3ExitCode(err) != 2 {
		t.Errorf("--min-confidence 1.5: exit %d", s3ExitCode(err))
	}
}

func TestCategorizeSuggestApplyPatchesSidecar(t *testing.T) {
	s3Fixture(t)
	requests := s3Sidecar(t, nil)
	var res s3ApplyResult
	s3RunJSON(t, &res, "categorize", "apply")
	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("sidecar requests = %+v, want 1 PATCH", reqs)
	}
	r := reqs[0]
	if r.Method != "PATCH" || r.Path != "/budgets/fixture-budget/transactions/txn-kr-6" || r.APIKey != "test-key" {
		t.Fatalf("request = %+v", r)
	}
	txn, _ := r.Body["transaction"].(map[string]any)
	if txn == nil || txn["category"] != "cat-groceries" || len(txn) != 1 {
		t.Fatalf("PATCH body = %+v, want {transaction:{category:cat-groceries}}", r.Body)
	}
	if res.DryRun || len(res.Applied) != 1 || res.Applied[0].Status != "applied" || len(res.FetchFailures) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestCategorizeSuggestApplyKeepsPartialFailures(t *testing.T) {
	s3Fixture(t)
	requests := s3Sidecar(t, func(path string) int {
		if strings.HasSuffix(path, "/txn-ch-2") {
			return 400
		}
		return 200
	})
	out, stderr, err := s3Run(t, "categorize", "apply", "--min-history", "1")
	if err != nil {
		t.Fatalf("partial failure should not fail the run: %v\n%s", err, stderr)
	}
	if len(requests()) != 4 {
		t.Fatalf("requests = %d, want 4", len(requests()))
	}
	if !strings.Contains(stderr, "1 of 4 category updates failed") {
		t.Errorf("stderr missing warning: %q", stderr)
	}
	var res s3ApplyResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.FetchFailures) != 1 || res.FetchFailures[0].TransactionID != "txn-ch-2" || len(res.Applied) != 3 {
		t.Errorf("partial result = %+v", res)
	}
}

func TestCategorizeSuggestApplyDryRunSendsNothing(t *testing.T) {
	s3Fixture(t)
	requests := s3Sidecar(t, nil)
	out, _, err := s3Run(t, "categorize", "apply", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(requests()); n != 0 {
		t.Fatalf("dry-run sent %d requests", n)
	}
	var res s3ApplyResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || len(res.Applied) != 1 || res.Applied[0].Path != "/budgets/{budgetSyncId}/transactions/txn-kr-6" || res.Applied[0].Status != "planned" || res.Applied[0].Method != "PATCH" {
		t.Fatalf("dry-run plan = %s", out)
	}
}

func TestCategorizeSuggestNoMirrorEmpty(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "categorize", "suggest")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("no mirror: out=%q err=%v", out, err)
	}
}
