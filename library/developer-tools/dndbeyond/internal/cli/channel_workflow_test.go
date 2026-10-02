// Copyright 2026 Matthew Martin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowArchiveFailureReturnsNonzeroAndAccurateJSON(t *testing.T) {
	home := withTempLearnHome(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "synthetic archive failure", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	t.Setenv("DNDBEYOND_BASE_URL", server.URL)

	stdout, stderr, err := runRootArgs(t,
		"workflow", "archive",
		"--json",
		"--rate-limit", "0",
		"--db", filepath.Join(home, "archive.db"),
	)
	if err == nil || !strings.Contains(err.Error(), "archive incomplete: 1 of 1 resources failed") {
		t.Fatalf("archive error = %v, want incomplete archive error (stderr=%q)", err, stderr)
	}

	var summary struct {
		ResourcesTotal  int `json:"resources_total"`
		ResourcesSynced int `json:"resources_synced"`
		ResourcesFailed int `json:"resources_failed"`
		FailedResources []struct {
			Resource string `json:"resource"`
			Error    string `json:"error"`
		} `json:"failed_resources"`
		TotalItems int `json:"total_items"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode archive summary: %v (stdout=%q)", err, stdout)
	}
	if summary.ResourcesTotal != 1 || summary.ResourcesSynced != 0 || summary.ResourcesFailed != 1 || summary.TotalItems != 0 {
		t.Fatalf("inaccurate archive summary: %+v", summary)
	}
	if len(summary.FailedResources) != 1 || summary.FailedResources[0].Resource != "pages" || !strings.Contains(summary.FailedResources[0].Error, "HTTP 500") {
		t.Fatalf("failed resource details = %+v, want pages HTTP 500", summary.FailedResources)
	}
}
