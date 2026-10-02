package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func reportingPage(ids ...int) json.RawMessage {
	rows := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, map[string]any{"metadata": map[string]any{"campaignId": fmt.Sprintf("%d", id)}})
	}
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{"reportingDataResponse": map[string]any{"row": rows}}})
	return raw
}

func TestCollectOffsetPagesFetchesUntilShortPage(t *testing.T) {
	var offsets []int
	pages := []json.RawMessage{reportingPage(1, 2), reportingPage(3, 4), reportingPage(5)}
	rows, err := collectOffsetPages(2, func(offset, _ int) (json.RawMessage, error) {
		offsets = append(offsets, offset)
		return pages[len(offsets)-1], nil
	}, extractReportingRowsRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || !reflect.DeepEqual(offsets, []int{0, 2, 4}) {
		t.Fatalf("rows=%d offsets=%v", len(rows), offsets)
	}
}

func TestCollectOffsetPagesRejectsStuckPagination(t *testing.T) {
	_, err := collectOffsetPages(2, func(_, _ int) (json.RawMessage, error) {
		return reportingPage(1, 2), nil
	}, extractReportingRowsRaw)
	if err == nil {
		t.Fatal("expected non-advancing pagination to fail")
	}
}

func TestCollectOffsetPagesDiscardsIncompleteResults(t *testing.T) {
	requests := 0
	rows, err := collectOffsetPages(2, func(_, _ int) (json.RawMessage, error) {
		requests++
		if requests == 1 {
			return reportingPage(1, 2), nil
		}
		return nil, fmt.Errorf("second page failed")
	}, extractReportingRowsRaw)
	if err == nil || rows != nil || requests != 2 {
		t.Fatalf("rows=%v err=%v requests=%d", rows, err, requests)
	}
}

func TestCloneReportBodyDoesNotMutateCaller(t *testing.T) {
	base := map[string]any{"selector": map[string]any{"pagination": map[string]int{"offset": 0, "limit": 1}}}
	copy, err := cloneReportBody(base, 25, 50)
	if err != nil {
		t.Fatal(err)
	}
	got := copy["selector"].(map[string]any)["pagination"].(map[string]any)
	if got["offset"] != 25 || got["limit"] != 50 {
		t.Fatalf("unexpected cloned pagination: %#v", got)
	}
	original := base["selector"].(map[string]any)["pagination"].(map[string]int)
	if original["offset"] != 0 || original["limit"] != 1 {
		t.Fatalf("caller body mutated: %#v", original)
	}
}

func TestExtractOffsetPageDryRunSentinel(t *testing.T) {
	sentinel := json.RawMessage(`{"dry_run":true}`)
	rows, err := extractOffsetPage(sentinel, true, extractReportingRowsRaw)
	if err != nil || len(rows) != 0 {
		t.Fatalf("dry-run reporting page: rows=%v err=%v", rows, err)
	}
	rows, err = extractOffsetPage(sentinel, true, extractGenericItemsRaw)
	if err != nil || len(rows) != 0 {
		t.Fatalf("dry-run item page: rows=%v err=%v", rows, err)
	}
	if _, err := extractOffsetPage(sentinel, false, extractReportingRowsRaw); err == nil {
		t.Fatal("live reporting response missing rows must fail")
	}
	if _, err := extractOffsetPage(sentinel, false, extractGenericItemsRaw); err == nil {
		t.Fatal("live item response missing items must fail")
	}
}

func TestPaginatedWorkflowDryRunsDoNotRequireAPIResults(t *testing.T) {
	t.Setenv("APPLE_SEARCH_ADS_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("APPLE_SEARCH_ADS_TOKEN", "test-token")
	for _, args := range [][]string{
		{"keywords", "auto-promote", "--campaign-id", "camp1"},
		{"optimize", "suggest", "--metric", "cpa", "--target", "2", "--campaign-id", "camp1"},
	} {
		t.Run(args[0]+"-"+args[1], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := newRootCmd(&rootFlags{})
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append(args, "--dry-run", "--no-cache", "--config", filepath.Join(t.TempDir(), "missing.toml")))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("dry-run command failed: %v stderr=%q", err, stderr.String())
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("dry-run output is not JSON: %q", stdout.String())
			}
		})
	}
}
