// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored regression tests for Crossref response envelopes.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
)

func TestExtractSearchResultsCrossrefMessageItems(t *testing.T) {
	got := extractSearchResults(json.RawMessage(`{"message":{"items":[{"DOI":"10.1000/example","title":["Example"]}]}}`))
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
	var work map[string]json.RawMessage
	if err := json.Unmarshal(got[0], &work); err != nil {
		t.Fatal(err)
	}
	if string(work["DOI"]) != `"10.1000/example"` {
		t.Fatalf("DOI = %s, want %q", work["DOI"], "10.1000/example")
	}
}

func TestWorksSearchAllReadsCrossrefMessagePages(t *testing.T) {
	var offsets []string
	var offsetsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/works" || r.URL.Query().Get("rows") != "2" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		offset := r.URL.Query().Get("offset")
		offsetsMu.Lock()
		offsets = append(offsets, offset)
		offsetsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch offset {
		case "0":
			_, _ = w.Write([]byte(`{"message":{"items":[{"DOI":"10.1000/one"},{"DOI":"10.1000/two"}]}}`))
		case "2":
			_, _ = w.Write([]byte(`{"message":{"items":[{"DOI":"10.1000/three"}]}}`))
		default:
			http.Error(w, fmt.Sprintf("unexpected offset %q", offset), http.StatusBadRequest)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("RETRACTION_CHECKER_BASE_URL", server.URL)
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--config", filepath.Join(home, "missing.toml"), "--data-source", "live", "--no-cache", "--json", "works", "search", "--query", "retraction", "--rows", "2", "--all"})
	if err := root.Execute(); err != nil {
		t.Fatalf("works search --all: %v; stderr %s", err, stderr.String())
	}
	offsetsMu.Lock()
	defer offsetsMu.Unlock()
	if len(offsets) != 2 || offsets[0] != "0" || offsets[1] != "2" {
		t.Fatalf("page offsets = %v, want 0 and 2", offsets)
	}
	var result struct {
		Results []struct {
			DOI string `json:"DOI"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("parse works search result: %v; stdout %s", err, stdout.String())
	}
	if len(result.Results) != 3 || result.Results[0].DOI != "10.1000/one" || result.Results[2].DOI != "10.1000/three" {
		t.Fatalf("works search results = %+v, want all three works", result.Results)
	}
}

func TestExtractPageItemsCrossrefMessageItems(t *testing.T) {
	items, _, _ := extractPageItems(json.RawMessage(`{"message":{"items":[{"DOI":"10.1000/example","title":["Example"]}]}}`), "cursor")
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	var work map[string]any
	if err := json.Unmarshal(items[0], &work); err != nil {
		t.Fatal(err)
	}
	if got := extractID("works", work); got != "10.1000/example" {
		t.Fatalf("work ID = %q, want %q", got, "10.1000/example")
	}
}
