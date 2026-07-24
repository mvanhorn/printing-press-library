// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
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

// TestSearchPluralTypeMapsToSingular covers every plural->singular mapping
// resolveSpotifyLiveSearchType performs, not just albums: the singular value
// it resolves must be what actually reaches the outgoing /search request's
// `type` query parameter.
func TestSearchPluralTypeMapsToSingular(t *testing.T) {
	cases := []struct {
		plural   string
		singular string
	}{
		{"albums", "album"},
		{"artists", "artist"},
		{"playlists", "playlist"},
		{"tracks", "track"},
		{"shows", "show"},
		{"episodes", "episode"},
		{"audiobooks", "audiobook"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.plural, func(t *testing.T) {
			var gotType string
			_, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
				gotType = r.URL.Query().Get("type")
				fmt.Fprintf(w, `{"%ss":{"items":[]}}`, tc.singular)
			}, "needle", "--type", tc.plural)
			if err != nil {
				t.Fatal(err)
			}
			if gotType != tc.singular {
				t.Fatalf("--type %s: outgoing type=%q, want %q", tc.plural, gotType, tc.singular)
			}
		})
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

// TestSearchAllCandidateTypesRejectedIsAnError covers the branch of the
// per-type fan-out where every candidate type is rejected: the combined
// request 400s, and so does every subsequent per-type retry. That must
// surface as a real error, not a silent exit 0 with an empty result set,
// and (since --data-source defaults to live in runSearchTest) it must not
// fall through to local FTS either.
func TestSearchAllCandidateTypesRejectedIsAnError(t *testing.T) {
	stdout, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"type unavailable in market"}`, http.StatusBadRequest)
	}, "needle", "--limit", "50")
	if err == nil {
		t.Fatalf("expected an error when every candidate type is rejected, got stdout=%s", stdout)
	}
	if ExitCode(err) == 0 {
		t.Fatalf("expected a non-zero exit code, got 0: %v", err)
	}
}

// typeRecorder collects the distinct `type` values the CLI asked for. Raw
// request counts are the wrong assertion here: the client transparently retries
// 429s and 5xx, so one logical per-type call can be several HTTP requests.
type typeRecorder struct {
	mu    sync.Mutex
	types []string
}

func (r *typeRecorder) record(searchType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, seen := range r.types {
		if seen == searchType {
			return
		}
	}
	r.types = append(r.types, searchType)
}

func (r *typeRecorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.types...)
}

// disableClientRetries turns off the HTTP client's retry/backoff so error-path
// tests assert on the CLI's own behavior instead of waiting out the backoff.
func disableClientRetries(t *testing.T) {
	t.Helper()
	t.Setenv(cliutil.DogfoodEnvVar, "1")
}

// A non-rejection failure part-way through the degraded fan-out must not throw
// away the hits the earlier candidate types already returned.
func TestSearchKeepsResultsWhenLaterTypeFailsHard(t *testing.T) {
	disableClientRetries(t)
	var recorder typeRecorder
	stdout, stderr, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		searchType := r.URL.Query().Get("type")
		recorder.record(searchType)
		switch {
		case strings.Contains(searchType, ","):
			http.Error(w, `{"error":"type unavailable in market"}`, http.StatusBadRequest)
		case searchType == "album":
			fmt.Fprint(w, `{"albums":{"items":[{"id":"al1","name":"Album"}]}}`)
		case searchType == "artist":
			fmt.Fprint(w, `{"artists":{"items":[{"id":"a1","name":"Artist"}]}}`)
		case searchType == "playlist":
			http.Error(w, `{"error":"server exploded"}`, http.StatusInternalServerError)
		default:
			http.Error(w, `{"error":"should not be reached"}`, http.StatusTeapot)
		}
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatalf("partial results were discarded: %v", err)
	}
	for _, id := range []string{"al1", "a1"} {
		if !strings.Contains(stdout, id) {
			t.Fatalf("lost result %q collected before the failure: %s", id, stdout)
		}
	}
	// The fan-out stops at the hard failure instead of walking the rest.
	want := []string{strings.Join(spotifyLiveSearchTypes, ","), "album", "artist", "playlist"}
	if got := recorder.seen(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requested types = %v, want %v", got, want)
	}
	if !strings.Contains(stderr, "playlist") {
		t.Fatalf("stderr does not report the aborted type: %s", stderr)
	}
}

// --type is validated against the live catalog whitelist only when the request
// actually reaches the API; local FTS indexes every synced resource type.
func TestSearchLocalDataSourceAcceptsNonCatalogType(t *testing.T) {
	for _, resourceType := range []string{"browse", "categories", "recommendations"} {
		t.Run(resourceType, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dataSource: "local"}
			cmd := newSearchCmd(flags)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"needle", "--type", resourceType, "--db", filepath.Join(t.TempDir(), "data.db")})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("local search rejected --type %q: %v (exit %d)", resourceType, err, ExitCode(err))
			}
		})
	}
}

// 429 is throttling, not "this catalog type is unavailable": it must propagate
// on the first request instead of being replayed once per candidate type.
func TestSearchRateLimitIsNotFannedOut(t *testing.T) {
	disableClientRetries(t)
	var recorder typeRecorder
	_, stderr, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
		recorder.record(r.URL.Query().Get("type"))
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	}, "needle", "--limit", "50")
	if err == nil {
		t.Fatal("429 did not propagate as an error")
	}
	want := []string{strings.Join(spotifyLiveSearchTypes, ",")}
	if got := recorder.seen(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requested types = %v, want %v (429 must not fan out per type)", got, want)
	}
	if strings.Contains(stderr, "excluded type") {
		t.Fatalf("429 was reported as a permanent type rejection: %s", stderr)
	}
}

// 401/403/404 are auth, permission and missing-resource failures — same rule.
func TestSearchNonRejectionStatusesAreNotFannedOut(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			disableClientRetries(t)
			var recorder typeRecorder
			_, _, err := runSearchTest(t, func(w http.ResponseWriter, r *http.Request) {
				recorder.record(r.URL.Query().Get("type"))
				http.Error(w, `{"error":"nope"}`, status)
			}, "needle", "--limit", "50")
			if err == nil {
				t.Fatalf("HTTP %d did not propagate as an error", status)
			}
			want := []string{strings.Join(spotifyLiveSearchTypes, ",")}
			if got := recorder.seen(); !reflect.DeepEqual(got, want) {
				t.Fatalf("requested types = %v, want %v", got, want)
			}
		})
	}
}

// A degraded result set must be distinguishable from a complete one in the JSON
// envelope itself, not only on stderr.
func TestSearchDegradedEnvelopeCarriesReason(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
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
	}

	assertReason := func(t *testing.T, stdout string) {
		t.Helper()
		var envelope struct {
			Meta map[string]any `json:"meta"`
		}
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("invalid envelope: %v\n%s", err, stdout)
		}
		reason, _ := envelope.Meta["reason"].(string)
		if reason == "" {
			t.Fatalf("degraded envelope has no meta.reason: %s", stdout)
		}
		if !strings.Contains(reason, "audiobook") {
			t.Fatalf("meta.reason does not name the excluded type: %q", reason)
		}
	}

	t.Run("json", func(t *testing.T) {
		stdout, _, err := runSearchTest(t, handler, "needle", "--limit", "50")
		if err != nil {
			t.Fatal(err)
		}
		assertReason(t, stdout)
	})
	t.Run("agent", func(t *testing.T) {
		stdout, err := runSearchTestAgent(t, "live", handler, "needle", "--limit", "50")
		if err != nil {
			t.Fatal(err)
		}
		assertReason(t, stdout)
	})
}

// A complete live search must NOT carry a degradation reason.
func TestSearchCompleteEnvelopeHasNoReason(t *testing.T) {
	stdout, _, err := runSearchTest(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"artists":{"items":[{"id":"a1","name":"Artist"}]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope.Meta["reason"]; ok {
		t.Fatalf("complete result set claims degradation: %s", stdout)
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

// runSearchTestAgent mirrors runSearchTest but enables agent mode, which is the
// only path where the printer adds its own provenance envelope.
func runSearchTestAgent(t *testing.T, dataSource string, handler http.HandlerFunc, args ...string) (string, error) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("base_url = %q\naccess_token = \"test-token\"\n", server.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{asJSON: true, agent: true, dataSource: dataSource, configPath: configPath}
	cmd := newSearchCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestSearchAgentEmitsSingleEnvelopeWithTrueProvenance(t *testing.T) {
	stdout, err := runSearchTestAgent(t, "live", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"artists":{"items":[{"id":"a1","name":"Artist"}]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta    map[string]any    `json:"meta"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("results is not a flat array — output is double-wrapped: %v\n%s", err, stdout)
	}
	if got := envelope.Meta["source"]; got != "live" {
		t.Fatalf("meta.source = %v, want live\n%s", got, stdout)
	}
	if len(envelope.Results) != 1 {
		t.Fatalf("results = %d, want 1\n%s", len(envelope.Results), stdout)
	}
	if strings.Count(stdout, `"meta"`) != 1 {
		t.Fatalf("expected exactly one meta block, got %d\n%s", strings.Count(stdout, `"meta"`), stdout)
	}
}

// TestSearchAgentEmitsSingleEnvelopeWithLocalProvenance mirrors
// TestSearchAgentEmitsSingleEnvelopeWithTrueProvenance from the other
// direction: the live path is not the only one that can double-wrap the
// agent envelope. A --data-source local search under --agent must also
// emit exactly one envelope, with meta.source "local".
func TestSearchAgentEmitsSingleEnvelopeWithLocalProvenance(t *testing.T) {
	stdout, err := runSearchTestAgent(t, "local", func(http.ResponseWriter, *http.Request) {
		t.Fatal("local search must not hit the live API")
	}, "needle", "--db", filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta    map[string]any    `json:"meta"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("results is not a flat array — output is double-wrapped: %v\n%s", err, stdout)
	}
	if got := envelope.Meta["source"]; got != "local" {
		t.Fatalf("meta.source = %v, want local\n%s", got, stdout)
	}
	if strings.Count(stdout, `"meta"`) != 1 {
		t.Fatalf("expected exactly one meta block, got %d\n%s", strings.Count(stdout, `"meta"`), stdout)
	}
}

func TestSearchNonAgentJSONKeepsSingleProvenanceEnvelope(t *testing.T) {
	stdout, _, err := runSearchTest(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"artists":{"items":[{"id":"a1","name":"Artist"}]}}`)
	}, "needle", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta    map[string]any    `json:"meta"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("non-agent JSON lost its provenance envelope: %v\n%s", err, stdout)
	}
	if got := envelope.Meta["source"]; got != "live" {
		t.Fatalf("meta.source = %v, want live\n%s", got, stdout)
	}
}

// runSearchTestWithDataSource mirrors runSearchTest with an explicit data source
// and a temporary database path, so local-only paths do not touch the real store.
func runSearchTestWithDataSource(t *testing.T, dataSource string, handler http.HandlerFunc, args ...string) (string, string, error) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("base_url = %q\naccess_token = \"test-token\"\n", server.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{asJSON: true, dataSource: dataSource, configPath: configPath}
	cmd := newSearchCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append(append([]string{}, args...), "--db", filepath.Join(dir, "data.db")))
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestSearchLiveDataSourceRejectsLocalOnlyTypes(t *testing.T) {
	for _, localOnly := range []string{"me", "chapters"} {
		t.Run(localOnly, func(t *testing.T) {
			_, _, err := runSearchTestWithDataSource(t, "live", func(w http.ResponseWriter, _ *http.Request) {
				t.Fatal("live search must not be attempted for a local-only type")
			}, "needle", "--type", localOnly)
			if err == nil {
				t.Fatal("expected a usage error, got nil")
			}
			var cerr *cliError
			if !errors.As(err, &cerr) || cerr.code != 2 {
				t.Fatalf("expected exit code 2, got %v", err)
			}
			if !strings.Contains(err.Error(), "no live Spotify search") {
				t.Fatalf("error should name the constraint, got %q", err.Error())
			}
		})
	}
}

func TestSearchAutoStillFallsBackForLocalOnlyTypes(t *testing.T) {
	_, _, err := runSearchTestWithDataSource(t, "auto", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("auto must not hit the live API for a local-only type")
	}, "needle", "--type", "me")
	if err != nil {
		t.Fatalf("auto mode should still serve local-only types locally: %v", err)
	}
}
