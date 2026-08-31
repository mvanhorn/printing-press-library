// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/internal/cliutil/testenv"
)

// TestNovelQueueEstimateHelpWires smoke-tests that the queue estimate command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelQueueEstimateHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"queue", "estimate", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("queue estimate --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "estimate"} {
		if !strings.Contains(help, want) {
			t.Fatalf("queue estimate --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelQueueEstimateJoinsLocalQueueWithLiveBalance(t *testing.T) {
	testenv.Isolate(t, cliutil.StateDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credits" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"remainingCredits": 100, "planTier": "pro"}`))
	}))
	defer server.Close()
	t.Setenv("FLOW_BASE_URL", server.URL)
	t.Setenv("FLOW_SESSION_TOKEN", "test-token")

	dir := t.TempDir()
	queuePath := filepath.Join(dir, "queue.json")
	queue := promptQueue{
		GeneratedFrom: "recap_script.json",
		Shots: []queueShot{
			{Index: 0, EstimatedCredits: 15},
			{Index: 1, EstimatedCredits: 15},
		},
	}
	raw, err := json.Marshal(queue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queuePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"queue", "estimate", queuePath, "--api-key", "fake-browser-key", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("queue estimate execute error = %v, output:\n%s", err, out.String())
	}

	var result queueEstimateResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if result.Shots != 2 || result.EstimatedCredits != 30 {
		t.Fatalf("expected 2 shots / 30 credits, got %+v", result)
	}
	if result.RemainingCredits == nil || *result.RemainingCredits != 100 {
		t.Fatalf("expected remaining_credits=100 from the live balance, got %+v", result)
	}
	if result.Fits == nil || !*result.Fits {
		t.Fatalf("expected fits=true (30 <= 100), got %+v", result)
	}
}
