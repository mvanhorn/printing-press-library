// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowArchiveReportsPartialFailureAndExitsNonZero(t *testing.T) {
	home := withTempLearnHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/results/categories", "/results/seasons":
			_, _ = w.Write([]byte(`[]`))
		case "/riders":
			http.Error(w, `riders unavailable`, http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("MOTOGP_BASE_URL", server.URL)

	dbPath := filepath.Join(home, "archive.db")
	stdout, stderr, err := runRootArgs(t, "--no-cache", "--json", "workflow", "archive", "--db", dbPath)
	if err == nil {
		t.Fatal("workflow archive returned success after a resource failed")
	}
	if !strings.Contains(err.Error(), "archive incomplete: 1 of 3 resources failed") {
		t.Fatalf("error = %q, want accurate partial-failure count (stderr=%q)", err, stderr)
	}

	var summary struct {
		ResourcesTotal  int `json:"resources_total"`
		ResourcesSynced int `json:"resources_synced"`
		ResourcesFailed int `json:"resources_failed"`
		FailedResources []struct {
			Resource string `json:"resource"`
			Error    string `json:"error"`
		} `json:"failed_resources"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode archive summary: %v (stdout=%q)", err, stdout)
	}
	if summary.ResourcesTotal != 3 || summary.ResourcesSynced != 2 || summary.ResourcesFailed != 1 {
		t.Fatalf("archive counts = total:%d synced:%d failed:%d, want 3/2/1", summary.ResourcesTotal, summary.ResourcesSynced, summary.ResourcesFailed)
	}
	if len(summary.FailedResources) != 1 || summary.FailedResources[0].Resource != "riders" || summary.FailedResources[0].Error == "" {
		t.Fatalf("failed_resources = %#v, want one riders failure with an error", summary.FailedResources)
	}
}

func TestWorkflowArchiveRejectsNonJSONSuccessResponse(t *testing.T) {
	home := withTempLearnHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/riders" {
			_, _ = w.Write([]byte("not JSON"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	t.Setenv("MOTOGP_BASE_URL", server.URL)
	stdout, _, err := runRootArgs(t, "--no-cache", "--json", "workflow", "archive", "--db", filepath.Join(home, "archive.db"))
	if err == nil {
		t.Fatal("non-JSON 200 response produced a successful archive")
	}
	var summary struct {
		ResourcesFailed int `json:"resources_failed"`
		FailedResources []struct {
			Resource string `json:"resource"`
			Error    string `json:"error"`
		} `json:"failed_resources"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ResourcesFailed != 1 || len(summary.FailedResources) != 1 || summary.FailedResources[0].Resource != "riders" || !strings.Contains(summary.FailedResources[0].Error, "non_json_200_body") {
		t.Fatalf("non-JSON archive summary = %#v", summary)
	}
}

func TestArchiveSyncOutcomeCountsStoredItemsOnFailure(t *testing.T) {
	for _, result := range []syncResult{
		{Count: 7, Err: errors.New("later fetch failed")},
		{Count: 3, Warn: errors.New("partial access")},
		{Count: 2, IncompleteReason: "stuck_cursor"},
	} {
		count, failure := archiveSyncOutcome(result)
		if count != result.Count || failure == nil {
			t.Fatalf("result %#v summarized as count=%d failure=%v", result, count, failure)
		}
	}
}
