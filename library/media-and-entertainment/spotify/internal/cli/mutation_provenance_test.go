// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runMutationTest drives one mutate command against an httptest server with
// the flag set `--agent` produces (see the agent block in root.go: --agent
// turns on --json and --compact). The store write that follows a successful
// mutation is redirected at SPOTIFY_DATA_DIR so a test can never upsert into
// the developer's real library database.
func runMutationTest(t *testing.T, newCmd func(*rootFlags) *cobra.Command, handler http.HandlerFunc, args ...string) (string, string, error) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("SPOTIFY_DATA_DIR", t.TempDir())
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("base_url = %q\naccess_token = \"test-token\"\n", server.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{agent: true, asJSON: true, compact: true, dataSource: "live", configPath: configPath}
	cmd := newCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

// TestLiveMutationsReportLiveProvenance pins the contract for a representative
// sample of the mutate commands: when the command has just sent a real
// POST/PUT/DELETE to the API, the `--agent` envelope must attribute the body
// it prints to that call (meta.source == "live"). "local" is the label for
// output the printer cannot attribute to a live request; on a mutation
// response it is a lie an agent has no way to detect, and it would make a
// playlist that really exists on Spotify look like a local echo.
func TestLiveMutationsReportLiveProvenance(t *testing.T) {
	cases := []struct {
		name     string
		newCmd   func(*rootFlags) *cobra.Command
		method   string
		path     string
		response string
		args     []string
	}{
		{
			name:     "me create-playlist",
			newCmd:   newMeCreatePlaylistCmd,
			method:   http.MethodPost,
			path:     "/me/playlists",
			response: `{"id":"pl1","name":"Road Trip","snapshot_id":"snap1"}`,
			args:     []string{"--name", "Road Trip"},
		},
		{
			name:     "me add-to-queue",
			newCmd:   newMeAddToQueueCmd,
			method:   http.MethodPost,
			path:     "/me/player/queue",
			response: `{}`,
			args:     []string{"--uri", "spotify:track:4iV5W9uYEdYUVa79Axb7Rh"},
		},
		{
			name:     "me follow-artists-users",
			newCmd:   newMeFollowArtistsUsersCmd,
			method:   http.MethodPut,
			path:     "/me/following",
			response: `{}`,
			args:     []string{"--type", "artist", "--ids", "0TnOYISbd1XYRBk9myaseg", "--ids-2", "0TnOYISbd1XYRBk9myaseg"},
		},
		{
			name:     "playlists items add-to-playlist",
			newCmd:   newPlaylistsItemsAddToPlaylistCmd,
			method:   http.MethodPost,
			path:     "/playlists/pl1/items",
			response: `{"snapshot_id":"snap2"}`,
			args:     []string{"pl1", "--uris", "spotify:track:4iV5W9uYEdYUVa79Axb7Rh"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			stdout, stderr, err := runMutationTest(t, tc.newCmd, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.response)
			}, tc.args...)
			if err != nil {
				t.Fatalf("command failed: %v (stderr=%s)", err, stderr)
			}
			// Guard the premise: if the command never reached the API, a
			// "live" label in the output would prove nothing.
			if gotMethod != tc.method || gotPath != tc.path {
				t.Fatalf("request = %s %s, want %s %s", gotMethod, gotPath, tc.method, tc.path)
			}
			var envelope struct {
				Meta map[string]any `json:"meta"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("output is not a JSON envelope: %v: %s", err, stdout)
			}
			source, ok := envelope.Meta["source"].(string)
			if !ok {
				t.Fatalf("envelope has no meta.source: %s", stdout)
			}
			if source != "live" {
				t.Fatalf("meta.source = %q, want \"live\": %s", source, stdout)
			}
		})
	}
}

// mutationClientCall matches a mutating request through the API client.
var mutationClientCall = regexp.MustCompile(`c\.(Post|Put|Patch|Delete)\w*\(`)

// TestMutationCommandsDoNotPrintThroughProvenanceLessPrinter is the
// whole-package version of the contract above: it fails on any command file
// that both issues a mutating request and prints through
// printOutputWithFlags, whose hardcoded "local" meta cannot be true of a
// response that just came off the wire. Such a file must print through
// printOutputWithFlagsMeta with the source it actually has. This is a
// file-level check on purpose: the same generator emits all ~36 mutate
// commands, so a regression arrives in every one of them at once, and only a
// sample of them is exercised end-to-end above.
func TestMutationCommandsDoNotPrintThroughProvenanceLessPrinter(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !mutationClientCall.Match(src) {
			continue
		}
		if bytes.Contains(src, []byte("printOutputWithFlags(")) {
			offenders = append(offenders, path)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("mutate commands print live API responses through the provenance-less printer (use printOutputWithFlagsMeta with source=live): %s", strings.Join(offenders, ", "))
	}
}
