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
	"strings"
	"sync/atomic"
	"testing"
)

func runSearchTest(t *testing.T, handler http.HandlerFunc, args ...string) (string, string, error) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("base_url = %q\naccess_token = \"test-token\"\n", server.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{asJSON: true, dataSource: "live", configPath: configPath}
	cmd := newSearchCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestSearchDefaultCarriesEveryLiveType(t *testing.T) {
	var gotQ, gotType string
	_, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotQ, gotType = r.URL.Query().Get("q"), r.URL.Query().Get("type")
		fmt.Fprint(w, `{"artists":{"items":[]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	if gotQ != "needle" || gotType != strings.Join(spotifyLiveSearchTypes, ",") {
		t.Fatalf("q=%q type=%q", gotQ, gotType)
	}
}

func TestSearchArtistCarriesSingularType(t *testing.T) {
	var gotType string
	_, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotType = r.URL.Query().Get("type")
		fmt.Fprint(w, `{"artists":{"items":[]}}`)
	}, "needle", "--type", "artist")
	if err != nil {
		t.Fatal(err)
	}
	if gotType != "artist" {
		t.Fatalf("type=%q", gotType)
	}
}

func TestSearchPluralAlbumsMapsToAlbum(t *testing.T) {
	var gotType string
	_, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotType = r.URL.Query().Get("type")
		fmt.Fprint(w, `{"albums":{"items":[]}}`)
	}, "needle", "--type", "albums")
	if err != nil {
		t.Fatal(err)
	}
	if gotType != "album" {
		t.Fatalf("type=%q", gotType)
	}
}

func TestSearchLocalOnlyTypesBypassLive(t *testing.T) {
	for _, resourceType := range []string{"me", "chapters"} {
		t.Run(resourceType, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requests.Add(1)
			}))
			defer server.Close()
			flags := &rootFlags{asJSON: true, dataSource: "auto"}
			cmd := newSearchCmd(flags)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"needle", "--type", resourceType, "--db", filepath.Join(t.TempDir(), "data.db")})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 0 {
				t.Fatalf("made %d live requests", requests.Load())
			}
		})
	}
}

func TestSearchBogusTypeIsUsageError(t *testing.T) {
	flags := &rootFlags{dataSource: "live"}
	cmd := newSearchCmd(flags)
	cmd.SetArgs([]string{"needle", "--type", "bogus"})
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("err=%v exit=%d", err, ExitCode(err))
	}
	for _, valid := range []string{"album", "artist", "playlist", "track", "show", "episode", "audiobook", "me", "chapters"} {
		if !strings.Contains(err.Error(), valid) {
			t.Fatalf("error does not name valid type %q: %v", valid, err)
		}
	}
}

func TestSearchAggregatesMultipleTypePaths(t *testing.T) {
	stdout, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"artists":{"items":[{"id":"a1","name":"Artist"}]},"tracks":{"items":[{"id":"t1","name":"Track"}]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a1", "t1"} {
		if !strings.Contains(stdout, id) {
			t.Fatalf("output does not contain id %q: %s", id, stdout)
		}
	}
}

func TestSearchSkipsEmptyEarlierTypePath(t *testing.T) {
	stdout, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"artists":{"items":[]},"tracks":{"items":[{"id":"t1","name":"Track"}]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "t1") {
		t.Fatalf("output does not contain later result: %s", stdout)
	}
}

func TestSearchAllEmptyPathsReturnsEmptyResults(t *testing.T) {
	stdout, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"artists":{"items":[]},"tracks":{"items":[]},"albums":{"items":[]},"playlists":{"items":[]},"shows":{"items":[]},"episodes":{"items":[]},"audiobooks":{"items":[]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid output: %v: %s", err, stdout)
	}
	if len(envelope.Results) != 0 {
		t.Fatalf("results=%s", stdout)
	}
}

func TestSearchDegradesWhenOneTypeIsRejected(t *testing.T) {
	stdout, stderr, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		searchType := r.URL.Query().Get("type")
		if strings.Contains(searchType, ",") || searchType == "audiobook" {
			http.Error(w, `{"error":"type unavailable in market"}`, http.StatusBadRequest)
			return
		}
		if searchType == "track" {
			fmt.Fprint(w, `{"tracks":{"items":[{"id":"t1","name":"Track"}]}}`)
			return
		}
		fmt.Fprintf(w, `{"%ss":{"items":[]}}`, searchType)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "t1") || !strings.Contains(stderr, "audiobook") {
		t.Fatalf("stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestSearchLocalDataSourceBypassesLive(t *testing.T) {
	var requests atomic.Int32
	flags := &rootFlags{asJSON: true, dataSource: "local"}
	cmd := newSearchCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"needle", "--db", filepath.Join(t.TempDir(), "data.db")})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatalf("made %d live requests", requests.Load())
	}
}
