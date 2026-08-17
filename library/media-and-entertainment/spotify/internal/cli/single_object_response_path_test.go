// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func runSingleObjectResponsePathCommand(t *testing.T, payload string, newCommand func(*rootFlags) *cobra.Command, args ...string) json.RawMessage {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("base_url = \""+server.URL+"\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	flags := &rootFlags{
		asJSON:     true,
		configPath: configPath,
		noCache:    true,
		timeout:    5 * time.Second,
	}
	cmd := newCommand(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var envelope struct {
		Results json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode command output %q: %v", output.String(), err)
	}
	return envelope.Results
}

func TestCurrentUserProfileReturnsWholeObject(t *testing.T) {
	payload := `{"display_name":"Ada","id":"user-1","country":"IT","images":[{"url":"https://example.com/profile.jpg"}]}`
	results := runSingleObjectResponsePathCommand(t, payload, newMeGetCurrentUsersProfileCmd)

	var profile map[string]json.RawMessage
	if err := json.Unmarshal(results, &profile); err != nil {
		t.Fatalf("expected profile object, got %s: %v", results, err)
	}
	if string(profile["display_name"]) != `"Ada"` || string(profile["id"]) != `"user-1"` {
		t.Fatalf("expected whole profile with display_name and id, got %s", results)
	}
}

func TestPlaylistReturnsWholeObject(t *testing.T) {
	payload := `{"id":"playlist-1","name":"Road Trip","images":[{"url":"https://example.com/cover.jpg"}],"tracks":{"items":[{"track":{"id":"track-1"}}]}}`
	results := runSingleObjectResponsePathCommand(t, payload, newPlaylistsGetCmd, "playlist-1")

	var playlist map[string]json.RawMessage
	if err := json.Unmarshal(results, &playlist); err != nil {
		t.Fatalf("expected playlist object, got %s: %v", results, err)
	}
	if string(playlist["id"]) != `"playlist-1"` || string(playlist["name"]) != `"Road Trip"` {
		t.Fatalf("expected whole playlist with id and name, got %s", results)
	}
	if _, ok := playlist["tracks"]; !ok {
		t.Fatalf("expected nested tracks object to be preserved, got %s", results)
	}
}

func TestCategoryWithOnlyIconsArrayReturnsWholeObject(t *testing.T) {
	payload := `{"id":"focus","name":"Focus","icons":[{"url":"https://example.com/focus.jpg"}]}`
	results := runSingleObjectResponsePathCommand(t, payload, newBrowseGetACategoryCmd, "focus")

	var category map[string]json.RawMessage
	if err := json.Unmarshal(results, &category); err != nil {
		t.Fatalf("expected category object, got %s: %v", results, err)
	}
	if string(category["id"]) != `"focus"` || string(category["name"]) != `"Focus"` {
		t.Fatalf("expected whole category rather than its only array, got %s", results)
	}
}

func TestAlbumsCollectionStillExtractsAlbumsArray(t *testing.T) {
	payload := `{"albums":[{"id":"album-1","name":"First"},{"id":"album-2","name":"Second"}]}`
	results := runSingleObjectResponsePathCommand(t, payload, newAlbumsGetMultipleCmd, "--ids", "album-1,album-2")

	var albums []map[string]json.RawMessage
	if err := json.Unmarshal(results, &albums); err != nil {
		t.Fatalf("expected albums array, got %s: %v", results, err)
	}
	if len(albums) != 2 || string(albums[0]["id"]) != `"album-1"` {
		t.Fatalf("expected extracted albums array, got %s", results)
	}
}
