// Copyright 2026 rowdy and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/devonthink/internal/store"
)

func TestRecordsSearchStrategyKeepsSmartGroupLive(t *testing.T) {
	if got := recordsSearchStrategy("Project Scope"); got != "live" {
		t.Fatalf("strategy = %q, want live", got)
	}
	if got := recordsSearchStrategy(""); got != "auto" {
		t.Fatalf("unscoped strategy = %q, want auto", got)
	}
}

func configureMockSmartGroupMCP(t *testing.T, searchFails bool) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Params.Name != "search_records" {
			t.Errorf("unexpected MCP request: %v, tool %q", err, request.Params.Name)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		query, _ := request.Params.Arguments["query"].(string)
		text := `[{"uuid":"sg-1","name":"Project Scope","kind":"smartgroup"}]`
		if query != "kind:smartgroup" {
			if query != "invoice" || request.Params.Arguments["group_uuid"] != "sg-1" {
				t.Errorf("scoped MCP search arguments were not preserved: %#v", request.Params.Arguments)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if searchFails {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			text = `[{"uuid":"scoped-1","name":"Scoped Example"}]`
		}
		response := map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(endpoint.Port())
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, "Library", "Application Support", "DEVONthink", "MCP", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(map[string]any{"server": map[string]any{"port": port}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEVONTHINK_BASE_URL", "local")
}

func runMockScopedSearch(t *testing.T) (string, error) {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--json", "--no-cache", "records", "search", "invoice", "--smart-group", "sg-1"})
	err := cmd.Execute()
	return output.String(), err
}

func TestRecordsSearchSmartGroupLiveFailureNeverFallsBackToUnscopedRows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	configureMockSmartGroupMCP(t, true)
	dbPath := defaultDBPath("devonthink-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertBatch("records", []json.RawMessage{json.RawMessage(`{"uuid":"unscoped-1","name":"Unscoped Example"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := runMockScopedSearch(t)
	if err == nil {
		t.Fatalf("failed live Smart Group search returned success: %s", output)
	}
	if strings.Contains(output, "Unscoped Example") {
		t.Fatalf("failed scoped read returned unscoped local row: %s", output)
	}
}

func TestRecordsSearchSmartGroupSuccessFeedsOfflineMirror(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	configureMockSmartGroupMCP(t, false)
	output, err := runMockScopedSearch(t)
	if err != nil || !strings.Contains(output, "Scoped Example") {
		t.Fatalf("scoped live search failed: %v, output %s", err, output)
	}
	db, err := store.Open(defaultDBPath("devonthink-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	row, err := db.Get("records", "scoped-1")
	if err != nil || !strings.Contains(string(row), "Scoped Example") {
		t.Fatalf("scoped result missing from offline mirror: %v, row %s", err, row)
	}
}
