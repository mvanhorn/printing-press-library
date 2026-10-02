// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/store"
)

// TestNovelPullHelpWires smoke-tests that the pull command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPullHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"pull", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pull --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "pull"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pull --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestFallbackSnipIDIsPositionalAndEditStable guards the UUID-less fallback id: it
// keys on the clip position (episode+start+end), so editing a snip's content does
// NOT change its id (a re-pull updates the row instead of duplicating it), while
// two snips at different clip spans still get different ids.
func TestFallbackSnipIDIsPositionalAndEditStable(t *testing.T) {
	base := snipd.Snip{EpisodeID: "ep1", Start: "15:11", End: "16:37", Note: "original", Title: "A"}

	// Editing content (or the title) must NOT change the id — position is identity.
	edited := base
	edited.Note = "edited note"
	edited.Quote = "new quote"
	edited.Title = "renamed"
	if fallbackSnipID(base, -1) != fallbackSnipID(edited, -1) {
		t.Error("editing a UUID-less snip's content changed its id; a re-pull would duplicate it")
	}

	// Two snips at different clip spans must get different ids.
	other := base
	other.End = "17:00"
	if fallbackSnipID(base, -1) == fallbackSnipID(other, -1) {
		t.Error("snips at different clip spans collided on the fallback id")
	}

	if !strings.HasPrefix(fallbackSnipID(base, -1), "ep1#") {
		t.Errorf("fallback id %q is not namespaced under the episode id", fallbackSnipID(base, -1))
	}
}

// TestFallbackSnipIDBothEmptyTimesDistinguishedByOrdinal guards the case Greptile
// flagged: when a UUID-less snip exports with no start AND no end, start+end alone
// collapses every such snip in an episode to one key, silently overwriting. The
// caller's per-episode ordinal must keep distinct snips distinct — while staying
// stable (a re-pull at the same position updates the row, it doesn't duplicate).
func TestFallbackSnipIDBothEmptyTimesDistinguishedByOrdinal(t *testing.T) {
	a := snipd.Snip{EpisodeID: "ep1", Start: "", End: "", Note: "first"}
	b := snipd.Snip{EpisodeID: "ep1", Start: "", End: "", Note: "second"}

	// Same episode, both timestamps empty, distinct snips → distinct ordinals → distinct ids.
	if fallbackSnipID(a, 0) == fallbackSnipID(b, 1) {
		t.Error("two both-empty snips in one episode collided; one would overwrite the other")
	}

	// Same position → same id (a re-pull updates, not duplicates).
	if fallbackSnipID(a, 0) != fallbackSnipID(a, 0) {
		t.Error("both-empty fallback id is not stable at a fixed ordinal")
	}

	// Editing content at a fixed position must not change the id.
	edited := a
	edited.Note = "edited"
	edited.Quote = "added"
	if fallbackSnipID(a, 0) != fallbackSnipID(edited, 0) {
		t.Error("editing a both-empty snip's content changed its id at the same ordinal")
	}
}

// TestTsLater guards the incremental cursor comparison: it must order by real
// instant, not lexically, so timestamps with different offsets or fractional
// precision can't park the cursor before episodes it should still cover.
func TestTsLater(t *testing.T) {
	cases := []struct {
		name, a, b string
		want       bool
	}{
		{"any ts beats the empty initial cursor", "2026-07-13T10:00:00Z", "", true},
		{"an earlier instant does not beat a later one", "2026-07-13T09:00:00Z", "2026-07-13T10:00:00Z", false},
		// Same instant, different offset + precision — lexical order would say true.
		{"same instant across offsets is not later", "2026-07-13T12:00:00.000+02:00", "2026-07-13T10:00:00Z", false},
		// Later instant whose string sorts lower than the Z form.
		{"a genuinely later instant across offsets", "2026-07-13T13:00:00+02:00", "2026-07-13T10:00:00Z", true},
	}
	for _, tc := range cases {
		if got := tsLater(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: tsLater(%q, %q) = %v, want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

func TestStoreAndReconcileSnipsRemovesDeletedEpisodeSnips(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "snipd.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = db.Close() }()

	episodes := []snipd.Episode{
		{EpisodeID: "episode-1", Title: "Episode One"},
		{EpisodeID: "episode-2", Title: "Episode Two"},
	}
	initial := []snipd.Snip{
		{SnipID: "keep", EpisodeID: "episode-1", Note: "retained insight"},
		{SnipID: "deleted", EpisodeID: "episode-1", Note: "obsolete searchable phrase"},
		{SnipID: "other-episode", EpisodeID: "episode-2", Note: "outside refreshed partition"},
	}
	if err := storeAndReconcileSnips(db, episodes, initial); err != nil {
		t.Fatalf("initial export: %v", err)
	}
	refreshed := []snipd.Snip{
		{SnipID: "keep", EpisodeID: "episode-1", Note: "retained insight updated"},
	}
	if err := storeAndReconcileSnips(db, episodes[:1], refreshed); err != nil {
		t.Fatalf("refreshed export: %v", err)
	}

	count, err := db.Count("snips")
	if err != nil {
		t.Fatalf("count snips: %v", err)
	}
	if count != 2 {
		t.Fatalf("snip count = %d, want retained snip plus untouched other episode", count)
	}
	matches, err := db.Search("obsolete", 10, "snips")
	if err != nil {
		t.Fatalf("search stale phrase: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("stale FTS matches = %d, want 0", len(matches))
	}
	kept, err := db.Get("snips", "keep")
	if err != nil {
		t.Fatalf("get retained snip: %v", err)
	}
	var decoded snipd.Snip
	if err := json.Unmarshal(kept, &decoded); err != nil {
		t.Fatalf("decode retained snip: %v", err)
	}
	if decoded.Note != "retained insight updated" {
		t.Fatalf("retained note = %q, want refreshed value", decoded.Note)
	}
	if _, err := db.Get("snips", "other-episode"); err != nil {
		t.Fatalf("unrefreshed episode snip was removed: %v", err)
	}
}

func TestStoreAndReconcileSnipsHandlesEpisodeWithNoRemainingSnips(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "snipd.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = db.Close() }()
	episodes := []snipd.Episode{{EpisodeID: "episode-1"}}
	if err := storeAndReconcileSnips(db, episodes, []snipd.Snip{{SnipID: "last", EpisodeID: "episode-1"}}); err != nil {
		t.Fatalf("initial export: %v", err)
	}
	if err := storeAndReconcileSnips(db, episodes, nil); err != nil {
		t.Fatalf("empty refreshed export: %v", err)
	}
	count, err := db.Count("snips")
	if err != nil {
		t.Fatalf("count snips: %v", err)
	}
	if count != 0 {
		t.Fatalf("snip count = %d, want 0", count)
	}
}

func TestValidateExportBatchRejectsIncompleteDeletionSet(t *testing.T) {
	expected := []snipd.MetaEpisode{{EpisodeID: "episode-1", TotalSnipCount: 2}}
	complete := []snipd.Episode{{EpisodeID: "episode-1", SnipCount: 2}}
	snips := []snipd.Snip{{SnipID: "one", EpisodeID: "episode-1"}, {SnipID: "two", EpisodeID: "episode-1"}}
	if err := validateExportBatch(expected, complete, snips); err != nil {
		t.Fatalf("complete batch rejected: %v", err)
	}
	for _, tc := range []struct {
		name     string
		episodes []snipd.Episode
		snips    []snipd.Snip
	}{
		{"missing episode", nil, nil},
		{"truncated snip list", []snipd.Episode{{EpisodeID: "episode-1", SnipCount: 1}}, snips[:1]},
		{"inconsistent parsed snips", complete, snips[:1]},
		{"duplicate episode", append(complete, complete[0]), snips},
		{"unexpected episode", []snipd.Episode{{EpisodeID: "episode-2", SnipCount: 2}}, snips},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateExportBatch(expected, tc.episodes, tc.snips); err == nil {
				t.Fatal("incomplete export accepted as an authoritative deletion set")
			}
		})
	}
	if err := validateExportBatch(
		[]snipd.MetaEpisode{{EpisodeID: "episode-1", TotalSnipCount: 0}},
		[]snipd.Episode{{EpisodeID: "episode-1", SnipCount: 0}}, nil,
	); err != nil {
		t.Fatalf("genuinely empty episode rejected: %v", err)
	}
}

func TestValidateExportBatchRejectsUnsafeSnipIdentities(t *testing.T) {
	expected := []snipd.MetaEpisode{{EpisodeID: "episode-1", TotalSnipCount: 2}}
	episodes := []snipd.Episode{{EpisodeID: "episode-1", SnipCount: 2}}
	good := []snipd.Snip{
		{SnipID: "one", EpisodeID: "episode-1", URL: "https://share.snipd.com/snip/one"},
		{SnipID: "two", EpisodeID: "episode-1", URL: "https://share.snipd.com/snip/two"},
	}
	for _, tc := range []struct {
		name  string
		snips []snipd.Snip
	}{
		{"missing identifying URL", []snipd.Snip{good[0], {EpisodeID: "episode-1"}}},
		{"duplicate snip ID", []snipd.Snip{good[0], good[0]}},
		{"duplicate fallback position", []snipd.Snip{{EpisodeID: "episode-1", URL: "opaque-1", Start: "1:00", End: "1:30"}, {EpisodeID: "episode-1", URL: "opaque-2", Start: "1:00", End: "1:30"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateExportBatch(expected, episodes, tc.snips); err == nil {
				t.Fatal("unsafe export identity accepted for deletion reconciliation")
			}
		})
	}
}

func TestFallbackSnipDoesNotPruneUUIDKeyedRows(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "snipd.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	episodes := []snipd.Episode{{EpisodeID: "episode-1"}}
	if err := storeAndReconcileSnips(db, episodes, []snipd.Snip{{SnipID: "old-uuid", EpisodeID: "episode-1"}}); err != nil {
		t.Fatal(err)
	}
	if err := storeAndReconcileSnips(db, episodes, []snipd.Snip{{EpisodeID: "episode-1", URL: "opaque-link", Start: "1:00", End: "1:30"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get("snips", "old-uuid"); err != nil {
		t.Fatalf("uncertain fallback identity pruned old UUID row: %v", err)
	}
}

func snipExportZIP(t *testing.T, ids ...string) []byte {
	return snipExportZIPForEpisode(t, "episode-1", ids...)
}

func snipExportZIPForEpisode(t *testing.T, episodeID string, ids ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("episodes/" + episodeID + "_full_content.md")
	if err != nil {
		t.Fatal(err)
	}
	markdown := "episode_title=<<episode_title>>One<</episode_title>>\nshow_title=<<show_title>>Show<</show_title>>\n"
	for _, id := range ids {
		markdown += fmt.Sprintf("@@SNIP@@\nsnip_url=<<snip_url>>https://share.snipd.com/snip/%s<</snip_url>>\n", id)
	}
	if _, err := f.Write([]byte(markdown)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPullCursorStaysAtInitialMetadataSnapshot(t *testing.T) {
	firstZIP := snipExportZIPForEpisode(t, "episode-1", "11111111-2222-3333-4444-555555555555")
	secondZIP := snipExportZIPForEpisode(t, "episode-2")
	var getCount atomic.Int32
	var postCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if getCount.Add(1) == 1 {
				fmt.Fprint(w, `{"episode_batches":[{"index":1,"episodes":[{"episode_id":"episode-1","total_snip_count":1,"latest_snip_update_ts":"2026-10-01T00:01:00Z"}]},{"index":2,"episodes":[{"episode_id":"episode-2","total_snip_count":1,"latest_snip_update_ts":"2026-10-01T00:02:00Z"}]}]}`)
			} else {
				// The first episode was edited after its export, while the
				// second batch also changed count and forced this refresh.
				fmt.Fprint(w, `{"episode_batches":[{"index":1,"episodes":[{"episode_id":"episode-1","total_snip_count":1,"latest_snip_update_ts":"2026-10-01T00:03:00Z"}]},{"index":2,"episodes":[{"episode_id":"episode-2","total_snip_count":0,"latest_snip_update_ts":"2026-10-01T00:04:00Z"}]}]}`)
			}
			return
		}
		if postCount.Add(1) == 1 {
			w.Write(firstZIP)
		} else {
			w.Write(secondZIP)
		}
	}))
	defer server.Close()
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	t.Setenv("SNIPD_BASE_URL", server.URL)
	t.Setenv("SNIPD_TOKEN", "synthetic-test-token")
	cmd := RootCmd()
	cmd.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "config.toml"), "--json", "pull", "--db", filepath.Join(t.TempDir(), "mirror.db")})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("local pull: %v (%s)", err, output.String())
	}
	var result pullResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode pull result: %v (%s)", err, output.String())
	}
	if result.Cursor != "2026-10-01T00:02:00Z" || getCount.Load() != 2 || postCount.Load() != 2 {
		t.Fatalf("cursor=%q GET=%d POST=%d, want initial snapshot cursor and 2/2 calls", result.Cursor, getCount.Load(), postCount.Load())
	}
}

func TestFetchValidatedBatchRefreshesDriftedMetadata(t *testing.T) {
	zipBody := snipExportZIP(t, "11111111-2222-3333-4444-555555555555")
	var postCount atomic.Int32
	var getCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCount.Add(1)
			w.Write(zipBody)
		case http.MethodGet:
			getCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"episode_batches":[{"episodes":[{"episode_id":"episode-1","total_snip_count":1,"latest_snip_update_ts":"2026-10-01T00:01:00Z"}]}]}`)
		}
	}))
	defer server.Close()
	client := snipd.NewClient(server.URL, "synthetic-test-token")
	old := []snipd.MetaEpisode{{EpisodeID: "episode-1", TotalSnipCount: 2}}
	eps, snips, current, err := fetchValidatedBatch(context.Background(), client, "", old)
	if err != nil {
		t.Fatalf("complete export rejected after metadata drift: %v", err)
	}
	if len(eps) != 1 || len(snips) != 1 || current[0].TotalSnipCount != 1 || postCount.Load() != 1 || getCount.Load() != 1 {
		t.Fatalf("result episodes=%d snips=%d current=%+v POST=%d GET=%d", len(eps), len(snips), current, postCount.Load(), getCount.Load())
	}
}

func TestFetchValidatedBatchRetriesAfterCountMismatch(t *testing.T) {
	first := snipExportZIP(t, "11111111-2222-3333-4444-555555555555")
	second := snipExportZIP(t, "11111111-2222-3333-4444-555555555555", "66666666-7777-8888-9999-aaaaaaaaaaaa")
	var postCount atomic.Int32
	var getCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if postCount.Add(1) == 1 {
				w.Write(first)
			} else {
				w.Write(second)
			}
		case http.MethodGet:
			getCount.Add(1)
			fmt.Fprint(w, `{"episode_batches":[{"episodes":[{"episode_id":"episode-1","total_snip_count":2}]}]}`)
		}
	}))
	defer server.Close()
	client := snipd.NewClient(server.URL, "synthetic-test-token")
	expected := []snipd.MetaEpisode{{EpisodeID: "episode-1", TotalSnipCount: 2}}
	_, snips, _, err := fetchValidatedBatch(context.Background(), client, "", expected)
	if err != nil {
		t.Fatal(err)
	}
	if len(snips) != 2 || postCount.Load() != 2 || getCount.Load() != 1 {
		t.Fatalf("snips=%d POST=%d GET=%d, want 2/2/1", len(snips), postCount.Load(), getCount.Load())
	}
}
