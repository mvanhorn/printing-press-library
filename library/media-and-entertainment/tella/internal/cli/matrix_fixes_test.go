package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/store"
	"github.com/spf13/cobra"
)

func TestWorkflowArchiveEnvelopePagination(t *testing.T) {
	for _, mode := range []string{"fresh", "resume", "full", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "archive.db")
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if mode == "resume" || mode == "full" {
				if err := db.SaveSyncState("videos", "resume-token", 0); err != nil {
					t.Fatal(err)
				}
			}
			requests := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests[r.URL.Path]++
				query := r.URL.Query()
				if r.Method != http.MethodGet || query.Has("after") || (r.URL.Path == "/v1/videos" && query.Get("limit") != "100") {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				switch r.URL.Path {
				case "/v1/backgrounds", "/v1/motion/documents", "/v1/playlist-groups", "/v1/social-accounts", "/v1/subtitle-presets", "/v1/tags", "/v1/analytics", "/v1/text-presets":
					fmt.Fprint(w, `[]`)
				case "/v1/webhooks/endpoints":
					fmt.Fprint(w, `{"endpoints":[]}`)
				case "/v1/playlists":
					fmt.Fprint(w, `{"playlists":[],"pagination":{"nextCursor":null,"hasMore":false}}`)
				case "/v1/webhooks/messages":
					fmt.Fprint(w, `{"messages":[],"pagination":{"nextCursor":null,"hasMore":false}}`)
				case "/v1/videos":
					if requests[r.URL.Path] == 1 {
						wantCursor := ""
						if mode == "resume" {
							wantCursor = "resume-token"
						}
						if query.Get("cursor") != wantCursor {
							t.Errorf("first cursor = %q, want %q", query.Get("cursor"), wantCursor)
						}
						fmt.Fprint(w, `{"videos":[{"id":"video-one"},{"id":"video-two"}],"pagination":{"nextCursor":"next-token","hasMore":true}}`)
					} else {
						if query.Get("cursor") != "next-token" {
							t.Errorf("second cursor = %q, want next-token", query.Get("cursor"))
						}
						if mode == "interrupted" {
							http.Error(w, "unavailable", http.StatusBadRequest)
							return
						}
						fmt.Fprint(w, `{"videos":[{"id":"video-three"}],"pagination":{"nextCursor":null,"hasMore":false}}`)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			flags := transcriptReadFlags(t, server)
			if mode == "resume" || mode == "full" {
				client, err := flags.newClient()
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Upsert("videos", "video-one", json.RawMessage(`{"id":"video-one"}`)); err != nil {
					t.Fatal(err)
				}
				scope := syncCheckpointScope("videos", false, nil, syncClientTemplateVars(client), syncActivePathContext())
				if _, _, _, err := db.BindSyncScope("videos", scope); err != nil {
					t.Fatal(err)
				}
				if err := db.SaveSyncProgressInScope("videos", scope, "resume-token", 0); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newWorkflowArchiveCmd(flags)
			cmd.SilenceUsage = true
			var output, errOutput bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&errOutput)
			args := []string{"--db", dbPath}
			if mode == "full" {
				args = append(args, "--full")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("archive: %v\n%s", err, errOutput.String())
			}
			wantCount, wantCursor := 3, ""
			ids := []string{"video-one", "video-two", "video-three"}
			if mode == "interrupted" {
				wantCount, wantCursor = 2, "next-token"
				ids = ids[:2]
			}
			for _, id := range ids {
				data, err := db.Get("videos", id)
				if err != nil || data == nil {
					t.Fatalf("missing %s: data=%s err=%v", id, data, err)
				}
			}
			status, err := db.Status()
			if err != nil || status["videos"] != wantCount || status["playlists"] != 0 || status["webhooks"] != 0 {
				t.Fatalf("store counts = %v, err=%v", status, err)
			}
			if data, err := db.Get("videos", "videos-singleton"); (err != nil && !errors.Is(err, sql.ErrNoRows)) || data != nil {
				t.Fatalf("singleton row = %s, err=%v", data, err)
			}
			cursor, _, count, err := db.GetSyncState("videos")
			if err != nil || cursor != wantCursor || count != wantCount {
				t.Fatalf("sync state = %q, %d, %v", cursor, count, err)
			}
			var result struct {
				ResourcesSynced int    `json:"resources_synced"`
				TotalItems      int    `json:"total_items"`
				StorePath       string `json:"store_path"`
				Timestamp       string `json:"timestamp"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("archive JSON: %v\n%s", err, output.String())
			}
			wantResources := 12
			if mode == "interrupted" {
				wantResources--
			}
			if result.ResourcesSynced != wantResources || result.TotalItems != wantCount || result.StorePath != dbPath {
				t.Fatalf("archive output = %+v\n%s", result, errOutput.String())
			}
			if _, err := time.Parse(time.RFC3339, result.Timestamp); err != nil {
				t.Fatal(err)
			}
			if requests["/v1/videos"] != 2 || requests["/v1/playlists"] != 1 || requests["/v1/webhooks/messages"] != 1 {
				t.Fatalf("requests = %v", requests)
			}
			if !strings.Contains(errOutput.String(), fmt.Sprintf("videos: %d synced", wantCount)) {
				t.Fatalf("missing stderr progress: %s", errOutput.String())
			}
		})
	}
}

func TestSyncEmptyEnvelope(t *testing.T) {
	for _, body := range []string{
		`{"playlists":[],"pagination":{"nextCursor":null,"hasMore":false}}`,
		`{"data":[],"pagination":{"nextCursor":null,"hasMore":false}}`,
		`[]`,
		`{"id":"playlist-one","name":"Single playlist"}`,
	} {
		t.Run(body, func(t *testing.T) {
			wantCount := 0
			if strings.Contains(body, "playlist-one") {
				wantCount = 1
			}
			items, cursor, more := extractPageItems(json.RawMessage(body), "cursor")
			if (items == nil) != (wantCount == 1) || len(items) != 0 || cursor != "" || more {
				t.Fatalf("page = %s, %q, %v", items, cursor, more)
			}
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet || r.URL.Path != "/v1/playlists" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			client, err := transcriptReadFlags(t, server).newClient()
			if err != nil {
				t.Fatal(err)
			}
			db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			result := syncResource(t.Context(), client, db, "playlists", "", false, 0, false, false, nil, &bytes.Buffer{})
			rows, err := db.List("playlists", 10)
			if result.Err != nil || result.Warn != nil || result.Count != wantCount || requests != 1 || err != nil || len(rows) != wantCount {
				t.Fatalf("sync=%+v requests=%d rows=%s err=%v", result, requests, rows, err)
			}
			if wantCount == 1 {
				if data, err := db.Get("playlists", "playlist-one"); err != nil || data == nil {
					t.Fatalf("single object missing: %s, %v", data, err)
				}
			}
		})
	}
}

func TestWebhooksCreateEndpointFilterTypes(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{"video.created, export.ready", []string{"video.created", "export.ready"}},
		{`["video.created"]`, []string{"video.created"}},
		{" , video.created, , export.ready, ", []string{"video.created", "export.ready"}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodPost || r.URL.Path != "/v1/webhooks/endpoints" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				var body struct {
					FilterTypes []string `json:"filterTypes"`
					URL         string   `json:"url"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(body.FilterTypes, tc.want) || body.URL != "https://example.com/webhook" {
					t.Errorf("POST body = %+v, want filterTypes=%v", body, tc.want)
				}
				fmt.Fprint(w, `{"id":"endpoint-one"}`)
			}))
			defer server.Close()
			cmd := newWebhooksCreateEndpointCmd(transcriptReadFlags(t, server))
			if _, err := executeTranscriptRead(cmd, "--filter-types", tc.input, "--url", "https://example.com/webhook"); err != nil {
				t.Fatal(err)
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
		})
	}
}

func TestListReadsPurgeStaleEnvelopes(t *testing.T) {
	for _, mode := range []string{"archive", "sync"} {
		t.Run(mode, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "archive.db")
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, resource := range []string{"videos", "playlists"} {
				for _, id := range []string{resource, resource + "-singleton"} {
					body := json.RawMessage(fmt.Sprintf(`{"%s":[],"name":"phantomenvelope"}`, resource))
					if err := db.Upsert(resource, id, body); err != nil {
						t.Fatal(err)
					}
				}
			}
			// A delete for a different resource type must leave the row and FTS intact.
			if err := db.Delete("unrelated", "videos-singleton"); err != nil {
				t.Fatal(err)
			}
			if rows, err := db.Search("phantomenvelope", 10); err != nil || len(rows) != 4 {
				t.Fatalf("seed search: %s, %v", rows, err)
			}
			if err := db.Upsert("videos", "video-preserved", json.RawMessage(`{"id":"video-preserved","name":"preserved"}`)); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/videos":
					fmt.Fprint(w, `{"videos":[{"id":"video-one"},{"id":"video-two"}],"pagination":{"nextCursor":null,"hasMore":false}}`)
				case "/v1/playlists":
					fmt.Fprint(w, `{"playlists":[],"pagination":{"nextCursor":null,"hasMore":false}}`)
				case "/v1/webhooks/messages":
					fmt.Fprint(w, `{"messages":[],"pagination":{"nextCursor":null,"hasMore":false}}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			flags := transcriptReadFlags(t, server)
			cmd := newWorkflowArchiveCmd(flags)
			args := []string{"--db", dbPath, "--full"}
			if mode == "sync" {
				cmd = newSyncCmd(flags)
				args = []string{"--db", dbPath, "--resources", "videos,playlists"}
			}
			output, errOutput, err := executeMatrixCommand(t, cmd, args...)
			if err != nil {
				t.Fatalf("%s: %v\nstdout: %s\nstderr: %s", mode, err, output, errOutput)
			}
			for _, resource := range []string{"videos", "playlists"} {
				for _, id := range []string{resource, resource + "-singleton"} {
					if data, err := db.Get(resource, id); (err != nil && !errors.Is(err, sql.ErrNoRows)) || data != nil {
						t.Fatalf("stale %s/%s: %s, %v", resource, id, data, err)
					}
				}
			}
			for _, id := range []string{"video-one", "video-two", "video-preserved"} {
				if data, err := db.Get("videos", id); err != nil || data == nil {
					t.Fatalf("missing %s: %s, %v", id, data, err)
				}
			}
			if counts, err := db.Status(); err != nil || counts["videos"] != 3 || counts["playlists"] != 0 {
				t.Fatalf("counts = %v, err=%v", counts, err)
			}
			if rows, err := db.Search("phantomenvelope", 10); err != nil || len(rows) != 0 {
				t.Fatalf("phantom search results: %s, %v", rows, err)
			}
			// Check FTS directly: the search join alone can hide orphaned entries.
			rawDB, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer rawDB.Close()
			var ftsCount int
			if err := rawDB.QueryRow(`SELECT count(*) FROM resources_fts WHERE resources_fts MATCH 'phantomenvelope'`).Scan(&ftsCount); err != nil || ftsCount != 0 {
				t.Fatalf("stale FTS entries = %d, err=%v", ftsCount, err)
			}
		})
	}
}

func TestSyncEmptyEnvelopeMaxPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/playlists" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"playlists":[],"pagination":{"nextCursor":null,"hasMore":false}}`)
	}))
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "sync.db")
	cmd := newSyncCmd(transcriptReadFlags(t, server))
	output, errOutput, err := executeMatrixCommand(t, cmd, "--db", dbPath, "--resources", "playlists", "--max-pages", "1")
	if err != nil {
		t.Fatalf("sync: %v\nstdout: %s\nstderr: %s", err, output, errOutput)
	}
	for _, text := range []string{output, errOutput} {
		for _, warning := range []string{"sync_warning", "max_pages_cap_hit", "max-pages", "truncat"} {
			if strings.Contains(text, warning) {
				t.Fatalf("unexpected warning: %s", text)
			}
		}
	}
	var summary struct {
		Event        string `json:"event"`
		TotalRecords int    `json:"total_records"`
		Success      int    `json:"success"`
	}
	for _, line := range strings.Split(strings.TrimSpace(errOutput), "\n") {
		if err := json.Unmarshal([]byte(line), &summary); err != nil {
			t.Fatal(err)
		}
	}
	if summary.Event != "sync_summary" || summary.TotalRecords != 0 || summary.Success != 1 {
		t.Fatalf("summary = %+v\n%s", summary, output)
	}
	var machineSummary struct {
		TotalRecords int `json:"total_records"`
		Success      int `json:"success"`
	}
	if err := json.Unmarshal([]byte(output), &machineSummary); err != nil || machineSummary.TotalRecords != 0 || machineSummary.Success != 1 {
		t.Fatalf("machine summary = %+v, err=%v\n%s", machineSummary, err, output)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if counts, err := db.Status(); err != nil || counts["playlists"] != 0 {
		t.Fatalf("counts = %v, err=%v", counts, err)
	}
}

func executeMatrixCommand(t *testing.T, cmd *cobra.Command, args ...string) (string, string, error) {
	t.Helper()
	// Sync writes events to process streams, while archive uses Cobra's writers.
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	oldOut, oldErr, oldHumanFriendly := os.Stdout, os.Stderr, humanFriendly
	os.Stdout, os.Stderr, humanFriendly = stdout, stderr, false
	defer func() { os.Stdout, os.Stderr, humanFriendly = oldOut, oldErr, oldHumanFriendly }()
	var output, errOutput bytes.Buffer
	cmd.SilenceUsage = true
	cmd.SetOut(&output)
	cmd.SetErr(&errOutput)
	cmd.SetArgs(args)
	execErr := cmd.Execute()
	for _, stream := range []struct {
		file *os.File
		buf  *bytes.Buffer
	}{{stdout, &output}, {stderr, &errOutput}} {
		if _, err := stream.file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(stream.buf, stream.file); err != nil {
			t.Fatal(err)
		}
	}
	return output.String(), errOutput.String(), execErr
}

func TestSyncEmptyPlaylistsLocalList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := defaultDBPath("tella-pp-cli")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/playlists" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"playlists":[],"pagination":{"nextCursor":null,"hasMore":false}}`)
	}))
	defer server.Close()
	flags := transcriptReadFlags(t, server)
	flags.dataSource = "local"
	output, errOutput, err := executeMatrixCommand(t, newPlaylistsListCmd(flags))
	if err == nil || !strings.Contains(err.Error(), "no local data for") || !strings.Contains(err.Error(), "sync' first") {
		t.Fatalf("never-synced list: %v\nstdout: %s\nstderr: %s", err, output, errOutput)
	}
	if requests != 0 {
		t.Fatalf("local list made %d API requests", requests)
	}
	flags.dataSource = "live"
	output, errOutput, err = executeMatrixCommand(t, newSyncCmd(flags), "--db", dbPath, "--resources", "playlists")
	if err != nil {
		t.Fatalf("sync: %v\nstdout: %s\nstderr: %s", err, output, errOutput)
	}
	flags.dataSource = "local"
	output, errOutput, err = executeMatrixCommand(t, newPlaylistsListCmd(flags))
	if err != nil {
		t.Fatalf("synced local list: %v\nstdout: %s\nstderr: %s", err, output, errOutput)
	}
	var result struct {
		Results json.RawMessage `json:"results"`
		Meta    DataProvenance  `json:"meta"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("local list JSON: %v\n%s", err, output)
	}
	if string(result.Results) != "[]" || result.Meta.Source != "local" || result.Meta.Reason != "user_requested" || result.Meta.ResourceType != "playlists" || result.Meta.SyncedAt == nil || result.Meta.SyncedAt.IsZero() {
		t.Fatalf("unexpected local list: %s", output)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want only one for sync", requests)
	}
}

func TestSyncShortPagesFollowCursor(t *testing.T) {
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		cursors = append(cursors, cursor)
		if r.Method != http.MethodGet || r.URL.Path != "/v1/playlists" || r.URL.Query().Get("limit") != "100" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		switch len(cursors) {
		case 1:
			fmt.Fprint(w, `{"playlists":[{"id":"playlist-one"},{"id":"playlist-two"}],"pagination":{"hasMore":true,"nextCursor":"c2"}}`)
		case 2:
			fmt.Fprint(w, `{"playlists":[{"id":"playlist-three"}],"pagination":{"hasMore":false,"nextCursor":null}}`)
		default:
			http.Error(w, "unexpected extra page", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "sync.db")
	output, errOutput, err := executeMatrixCommand(t, newSyncCmd(transcriptReadFlags(t, server)), "--db", dbPath, "--resources", "playlists")
	if err != nil {
		t.Fatalf("sync: %v\nstdout: %s\nstderr: %s", err, output, errOutput)
	}
	if !reflect.DeepEqual(cursors, []string{"", "c2"}) {
		t.Fatalf("request cursors = %v, want [\"\" \"c2\"]", cursors)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if rows, err := db.List("playlists", 0); err != nil || len(rows) != 3 {
		t.Fatalf("stored playlists = %s, err=%v; want 3", rows, err)
	}
	for _, id := range []string{"playlist-one", "playlist-two", "playlist-three"} {
		if data, err := db.Get("playlists", id); err != nil || data == nil {
			t.Fatalf("missing %s: %s, %v", id, data, err)
		}
	}
}
