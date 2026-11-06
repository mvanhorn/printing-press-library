// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExportsWaitHTTP(t *testing.T) {
	const exportID = "Export-Story-vid_abc/2026-01-15T10:00:00Z/Story/1920x1080/30FPS"
	const pollURI = "/v1/videos/vid_abc/exports/Export-Story-vid_abc%2F2026-01-15T10:00:00Z%2FStory%2F1920x1080%2F30FPS"
	for _, tc := range []struct {
		name       string
		initial    string
		statuses   []string
		timeout    string
		postCode   int
		getCode    int
		topLevel   bool
		wantStatus string
		wantError  bool
		wantNote   string
		wantPolls  int
	}{
		{name: "Completed", initial: "queued", statuses: []string{"running", "completed"}, wantStatus: "completed", wantPolls: 2},
		{name: "Failed", initial: "queued", statuses: []string{"failed"}, wantStatus: "failed", wantError: true, wantPolls: 1},
		{name: "Timeout", initial: "queued", statuses: []string{"running"}, timeout: "100ms", wantStatus: "running", wantError: true, wantNote: "timed out", wantPolls: -1},
		{name: "PostFailed", initial: "failed", wantStatus: "failed", wantError: true},
		{name: "TopLevelCompleted", initial: "completed", topLevel: true, wantStatus: "completed"},
		{name: "PostError", postCode: http.StatusBadRequest, wantStatus: "error", wantError: true, wantNote: "HTTP 400"},
		{name: "GetError", initial: "queued", getCode: http.StatusBadRequest, wantStatus: "error", wantError: true, wantNote: "HTTP 400", wantPolls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			posts, polls := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				status := tc.initial
				switch request.Method {
				case http.MethodPost:
					posts++
					if request.RequestURI != "/v1/videos/vid_abc/exports" {
						t.Errorf("POST URI = %q", request.RequestURI)
					}
					if body := decodeFixtureBody(t, request); body == nil || len(body) != 0 {
						t.Errorf("POST body = %#v, want {}", body)
					}
					if tc.postCode != 0 {
						http.Error(w, `{"error":"bad_request"}`, tc.postCode)
						return
					}
				case http.MethodGet:
					polls++
					if request.RequestURI != pollURI || request.URL.RawQuery != "" {
						t.Errorf("GET URI = %q, want %q", request.RequestURI, pollURI)
					}
					if body := decodeFixtureBody(t, request); body != nil {
						t.Errorf("GET body = %#v, want no body", body)
					}
					if tc.getCode != 0 {
						http.Error(w, `{"error":"bad_request"}`, tc.getCode)
						return
					}
					if len(tc.statuses) == 0 {
						t.Error("unexpected poll")
						http.Error(w, "unexpected poll", http.StatusBadRequest)
						return
					}
					status = tc.statuses[min(polls-1, len(tc.statuses)-1)]
				default:
					t.Errorf("unexpected method %s", request.Method)
					http.Error(w, "unexpected method", http.StatusBadRequest)
					return
				}
				progress, downloadURL := "null", "null"
				if status == "running" {
					progress = "42.5"
				} else if status == "completed" {
					progress, downloadURL = "100", `"https://cdn.tella.tv/export.mp4"`
				}
				payload := fmt.Sprintf(`{"exportId":%q,"status":%q,"progress":%s,"downloadUrl":%s,"updatedAt":"2026-11-05T10:00:00Z"}`, exportID, status, progress, downloadURL)
				if !tc.topLevel {
					payload = `{"export":` + payload + `}`
				}
				fmt.Fprint(w, payload)
			}))
			defer server.Close()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TELLA_BASE_URL", server.URL)
			t.Setenv("TELLA_API_KEY", "fixture")
			flags := &rootFlags{timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
			cmd := newExportsWaitCmd(flags)
			var output, errOutput bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&errOutput)
			cmd.Flags().SetOutput(&errOutput)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			timeout := tc.timeout
			if timeout == "" {
				timeout = "1s"
			}
			cmd.SetArgs([]string{"--video", "vid_abc", "--poll-interval", "1ms", "--timeout", timeout, "--inbox-limit", "1"})
			err := cmd.Execute()
			if (err != nil) != tc.wantError || (err != nil && ExitCode(err) != 5) {
				t.Fatalf("error = %v, wantError = %v; output = %s", err, tc.wantError, output.String())
			}
			if !strings.Contains(errOutput.String(), "Flag --inbox-limit has been deprecated") {
				t.Fatalf("missing deprecation notice on stderr: %s", errOutput.String())
			}
			var result struct {
				Exports []struct {
					VideoID  string   `json:"video_id"`
					ExportID string   `json:"export_id"`
					Status   string   `json:"status"`
					Progress *float64 `json:"progress"`
					URL      string   `json:"url"`
					Note     string   `json:"note"`
				} `json:"exports"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("decode result: %v; output = %s", err, output.String())
			}
			if len(result.Exports) != 1 {
				t.Fatalf("exports = %#v", result.Exports)
			}
			r := result.Exports[0]
			if r.VideoID != "vid_abc" || r.Status != tc.wantStatus || !strings.Contains(r.Note, tc.wantNote) {
				t.Fatalf("result = %#v", r)
			}
			if tc.postCode == 0 && r.ExportID != exportID {
				t.Fatalf("export ID = %q, want %q", r.ExportID, exportID)
			}
			if r.Status == "completed" && (r.URL != "https://cdn.tella.tv/export.mp4" || r.Progress == nil || *r.Progress != 100) {
				t.Fatalf("completed result = %#v", r)
			}
			if r.Status == "running" && (r.Progress == nil || *r.Progress != 42.5) {
				t.Fatalf("running progress = %v", r.Progress)
			}
			mu.Lock()
			defer mu.Unlock()
			if posts != 1 || (tc.wantPolls >= 0 && polls != tc.wantPolls) || (tc.wantPolls == -1 && polls == 0) {
				t.Fatalf("requests: posts = %d, polls = %d; want polls = %d", posts, polls, tc.wantPolls)
			}
			if flag := cmd.Flags().Lookup("inbox-limit"); !flag.Hidden || flag.Deprecated == "" {
				t.Fatal("inbox-limit must remain hidden and deprecated")
			}
		})
	}
}

func TestExportsWaitDryRun(t *testing.T) {
	cmd := newExportsWaitCmd(&rootFlags{dryRun: true, configPath: filepath.Join(t.TempDir(), "missing.toml")})
	var output, errOutput bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&errOutput)
	cmd.Flags().SetOutput(&errOutput)
	cmd.SetArgs([]string{"--video", "vid_abc", "--inbox-limit", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run must return before creating a client: %v", err)
	}
	if output.Len() != 0 || !strings.Contains(errOutput.String(), "Flag --inbox-limit has been deprecated") {
		t.Fatalf("stdout = %q, stderr = %q; want deprecation notice only on stderr", output.String(), errOutput.String())
	}
}

func TestParseExportResponse(t *testing.T) {
	for _, payload := range []string{
		`{"export":{"exportId":"job/one","status":"running","downloadUrl":null,"progress":0}}`,
		`{"exportId":"job/one","status":"running","downloadUrl":null,"progress":0}`,
	} {
		id, status, url, progress := parseExportResponse(json.RawMessage(payload))
		if id != "job/one" || status != "running" || url != "" || progress == nil || *progress != 0 {
			t.Fatalf("parsed %s as (%q, %q, %q, %v)", payload, id, status, url, progress)
		}
	}
	_, _, _, progress := parseExportResponse(json.RawMessage(`{"export":{"progress":null}}`))
	if progress != nil {
		t.Fatalf("null progress = %v", progress)
	}
}

func TestExportsWaitRejectsInvalidDurations(t *testing.T) {
	for _, args := range [][]string{
		{"--timeout", "0s"},
		{"--poll-interval", "-1s"},
	} {
		cmd := newExportsWaitCmd(&rootFlags{})
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(append([]string{"--video", "vid_abc"}, args...))
		if err := cmd.Execute(); err == nil || ExitCode(err) != 2 {
			t.Fatalf("args = %v, error = %v; want usage error", args, err)
		}
	}
}
