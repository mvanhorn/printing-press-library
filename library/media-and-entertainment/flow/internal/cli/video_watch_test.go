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

// TestNovelVideoWatchHelpWires smoke-tests that the video watch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelVideoWatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"video", "watch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("video watch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "watch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("video watch --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelVideoWatchBatchChecksSubmittedShots(t *testing.T) {
	testenv.Isolate(t, cliutil.StateDir)

	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/video:batchCheckAsyncVideoGenerationStatus" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"op-1","state":"RUNNING","progressPercent":40,"creditsCost":15}]`))
	}))
	defer server.Close()
	t.Setenv("FLOW_BASE_URL", server.URL)
	t.Setenv("FLOW_SESSION_TOKEN", "test-token")

	dir := t.TempDir()
	batchPath := filepath.Join(dir, "queue.json")
	queue := promptQueue{Shots: []queueShot{
		{Index: 0, JobName: "op-1"},
		{Index: 1}, // no job_name yet -- should be skipped, not sent
	}}
	raw, err := json.Marshal(queue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(batchPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"video", "watch", "--batch", batchPath, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("video watch execute error = %v, output:\n%s", err, out.String())
	}

	names, _ := gotBody["names"].([]any)
	if len(names) != 1 || names[0] != "op-1" {
		t.Fatalf("expected only op-1 sent to the batch endpoint, got %+v", gotBody)
	}
	if !strings.Contains(out.String(), "op-1") || !strings.Contains(out.String(), "RUNNING") {
		t.Fatalf("expected output to include op-1's status, got:\n%s", out.String())
	}
}
