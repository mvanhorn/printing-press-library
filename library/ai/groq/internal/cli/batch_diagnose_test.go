// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelBatchDiagnoseHelpWires smoke-tests that the batch diagnose command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBatchDiagnoseHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"batch", "diagnose", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("batch diagnose --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "diagnose"} {
		if !strings.Contains(help, want) {
			t.Fatalf("batch diagnose --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestTabulateBatchResults(t *testing.T) {
	results := []byte(`{"id":"a","custom_id":"r1","response":{"status_code":200,"body":{"text":"ok"}}}
{"id":"b","custom_id":"r2","response":{"status_code":429,"body":{"error":{"message":"rate limited","type":"rate_limit_error"}}}}
{"id":"c","custom_id":"r3","response":{"status_code":500,"body":{"error":{"message":"server error","type":"server_error"}}}}
{"id":"d","custom_id":"r4","response":{"status_code":200,"body":{"text":"ok2"}}}`)
	batch := struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		InputFileID  string `json:"input_file_id"`
		OutputFileID string `json:"output_file_id"`
	}{ID: "batch_1", Status: "completed", InputFileID: "in", OutputFileID: "out"}

	report, err := tabulateBatchResults(batch, results)
	if err != nil {
		t.Fatalf("tabulateBatchResults error = %v", err)
	}
	if report.TotalLines != 4 {
		t.Fatalf("TotalLines = %d, want 4", report.TotalLines)
	}
	if report.StatusCounts["200"] != 2 || report.StatusCounts["429"] != 1 || report.StatusCounts["500"] != 1 {
		t.Fatalf("status counts = %v, want 200x2 429x1 500x1", report.StatusCounts)
	}
	if report.RetryWorthy != 2 {
		t.Fatalf("RetryWorthy = %d, want 2 (429 + 500)", report.RetryWorthy)
	}
	if len(report.ErrorSummary) == 0 || report.ErrorSummary[0].Count != 1 {
		t.Fatalf("error summary = %v, want count 1 entries", report.ErrorSummary)
	}
}

func TestTabulateBatchResultsNullResponseError(t *testing.T) {
	results := []byte(`{"id":"a","custom_id":"r1","response":{"status_code":200,"body":{"text":"ok"}}}
{"id":"b","custom_id":"r2","response":null,"error":{"type":"server_error","message":"upstream failed"}}`)
	batch := struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		InputFileID  string `json:"input_file_id"`
		OutputFileID string `json:"output_file_id"`
	}{ID: "b1", Status: "completed"}
	report, err := tabulateBatchResults(batch, results)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if report.StatusCounts["200"] != 1 {
		t.Fatalf("status counts = %v, want one 200", report.StatusCounts)
	}
	if report.StatusCounts["500"] != 1 {
		t.Fatalf("status counts = %v, want one 500 (derived from null response + error)", report.StatusCounts)
	}
	if report.RetryWorthy != 1 {
		t.Fatalf("RetryWorthy = %d, want 1", report.RetryWorthy)
	}
}
