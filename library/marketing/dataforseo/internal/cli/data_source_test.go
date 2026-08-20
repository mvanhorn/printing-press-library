// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
)

func TestExtractStoreItemsUnwrapsDataForSEOTaskResults(t *testing.T) {
	raw := json.RawMessage(`{
		"status_code": 20000,
		"tasks": [{
			"status_code": 20000,
			"result": [{"items": [
				{"keyword":"tree service","search_volume":120},
				{"keyword":"stump grinding","search_volume":80}
			]}]
		}]
	}`)

	items := extractStoreItems(raw)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %s", len(items), items)
	}
}

func TestExtractStoreItemsTreatsEmptyItemsAsAuthoritative(t *testing.T) {
	raw := json.RawMessage(`{
		"status_code": 20000,
		"tasks": [{
			"status_code": 20000,
			"result": [{"location_code": 2840, "items": []}]
		}]
	}`)

	if items := extractStoreItems(raw); len(items) != 0 {
		t.Fatalf("extractStoreItems returned phantom result: %s", items)
	}
}

func TestExtractStoreItemsCarriesSearchVolumeDimensionsOntoItems(t *testing.T) {
	raw := json.RawMessage(`{
		"status_code": 20000,
		"tasks": [{"result": [{
			"location_code": 2840,
			"location_name": "United States",
			"language_code": "en",
			"language_name": "English",
			"search_partners": true,
			"items": [{"keyword":"tree service","search_volume":120}]
		}]}]
	}`)

	items := extractStoreItems(raw)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	var item map[string]any
	if err := json.Unmarshal(items[0], &item); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"location_code": float64(2840), "location_name": "United States",
		"language_code": "en", "language_name": "English", "search_partners": true,
	} {
		if got := item[key]; got != want {
			t.Errorf("item[%q] = %#v, want %#v", key, got, want)
		}
	}
}

func TestWriteThroughAPIResponsePreservesKeywordObservations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	raw := json.RawMessage(`{"status_code":20000,"tasks":[{"status_code":20000,"result":[{"keyword":"tree service","search_volume":100}]}]}`)
	if err := WriteThroughAPIResponse(context.Background(), "/v3/keywords_data/google_ads/search_volume/live", raw); err != nil {
		t.Fatal(err)
	}
	raw = json.RawMessage(`{"status_code":20000,"tasks":[{"status_code":20000,"result":[{"keyword":"tree service","search_volume":125}]}]}`)
	if err := WriteThroughAPIResponse(context.Background(), "/v3/keywords_data/google_ads/search_volume/live", raw); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(home, ".local", "share", "dataforseo-pp-cli", "data.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("local database not created: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var historyCount int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resource_history WHERE resource_type = ? AND id = ?`, "keywords-data", "tree service").Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if historyCount != 2 {
		t.Fatalf("history count = %d, want 2", historyCount)
	}
	var source string
	if err := db.DB().QueryRow(`SELECT source FROM resource_history WHERE resource_type = ? AND id = ? LIMIT 1`, "keywords-data", "tree service").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "/v3/keywords_data/google_ads/search_volume/live" {
		t.Fatalf("history source = %q", source)
	}
}

func TestWriteThroughAPIResponseSkipsTaskReadinessEnvelopes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	raw := json.RawMessage(`{"status_code":20000,"tasks":[{"status_code":20000,"result":[{"id":"task-1"}]}]}`)
	if err := WriteThroughAPIResponse(context.Background(), "/v3/serp/google/organic/tasks_ready", raw); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(home, ".local", "share", "dataforseo-pp-cli", "data.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("readiness polling created local database: %v", err)
	}
}

func TestWriteThroughAPIResponseReturnsStoreOpenErrors(t *testing.T) {
	home := t.TempDir()
	blocker := filepath.Join(home, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", blocker)

	err := WriteThroughAPIResponse(context.Background(), "/v3/keywords_data/google_ads/search_volume/live", json.RawMessage(`[{"keyword":"tree service","search_volume":100}]`))
	if err == nil {
		t.Fatal("writeThroughAPIResponse error = nil, want store open error")
	}
	if !strings.Contains(err.Error(), "opening response store") {
		t.Fatalf("writeThroughAPIResponse error = %v", err)
	}
}

func TestNormalizeAPIResponseSourceStripsTaskIDAndQuery(t *testing.T) {
	got := normalizeAPIResponseSource(" /v3/keywords_data/google_ads/search_volume/task_get/task-123/?debug=1 ")
	want := "/v3/keywords_data/google_ads/search_volume/task_get"
	if got != want {
		t.Fatalf("normalizeAPIResponseSource = %q, want %q", got, want)
	}
}

func TestResourceTypeForAPIPath(t *testing.T) {
	cases := map[string]string{
		"/v3/keywords_data/google_ads/search_volume/live": "keywords-data",
		"/v3/serp/google/organic/live/advanced":           "serp-results",
		"/v3/backlinks/summary/live":                      "backlinks",
		"/v3/ai_optimization/llm_mentions/search/live":    "ai-mentions",
		"/v3/content_analysis/search/live":                "content-analysis",
	}
	for path, want := range cases {
		if got := resourceTypeForAPIPath(path); got != want {
			t.Errorf("resourceTypeForAPIPath(%q) = %q, want %q", path, got, want)
		}
	}
}
