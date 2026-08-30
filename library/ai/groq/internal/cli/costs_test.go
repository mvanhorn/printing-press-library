// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/groq/internal/store"
)

// TestNovelCostsHelpWires smoke-tests that the costs command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCostsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"costs", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("costs --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "costs"} {
		if !strings.Contains(help, want) {
			t.Fatalf("costs --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeCostsAggregation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	defer db.Close()

	chat1 := []byte(`{"id":"chatcmpl-1","model":"openai/gpt-oss-20b","created":1700000000,"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
	chat2 := []byte(`{"id":"chatcmpl-2","model":"openai/gpt-oss-20b","created":1700000000,"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
	chat3 := []byte(`{"id":"chatcmpl-3","model":"qwen/qwen3.6-27b","created":1700000000,"usage":{"prompt_tokens":200,"completion_tokens":100,"total_tokens":300}}`)
	if err := db.UpsertChat(chat1); err != nil {
		t.Fatalf("upsert chat1: %v", err)
	}
	if err := db.UpsertChat(chat2); err != nil {
		t.Fatalf("upsert chat2: %v", err)
	}
	if err := db.UpsertChat(chat3); err != nil {
		t.Fatalf("upsert chat3: %v", err)
	}

	report, err := computeCosts(context.Background(), db, time.Time{}, "model", map[string]modelPrice{})
	if err != nil {
		t.Fatalf("computeCosts error = %v", err)
	}
	if report.TotalRuns != 3 || report.TotalTokens != 600 {
		t.Fatalf("total runs/tokens = %d/%d, want 3/600", report.TotalRuns, report.TotalTokens)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 model groups", len(report.Rows))
	}
	var gptCost, qwenCost float64
	for _, r := range report.Rows {
		switch r.Model {
		case "openai/gpt-oss-20b":
			gptCost = r.CostUSD
			if r.Runs != 2 || r.TotalTokens != 300 {
				t.Fatalf("gpt-oss row = %+v, want 2 runs/300 tokens", r)
			}
		case "qwen/qwen3.6-27b":
			qwenCost = r.CostUSD
		}
	}
	wantGpt := 200*0.000000075 + 100*0.00000030
	if diff := gptCost - wantGpt; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("gpt-oss cost = %v, want ~%v", gptCost, wantGpt)
	}
	if qwenCost <= 0 {
		t.Fatalf("qwen cost = %v, want > 0 (builtin price)", qwenCost)
	}
}
