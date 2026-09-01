// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
)

type fixtureRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

func fixtureClient(server *httptest.Server) *client.Client {
	api := client.New(&config.Config{BaseURL: server.URL, TellaApiKey: "fixture"}, 2*time.Second, 0)
	api.NoCache = true
	return api
}

func decodeFixtureBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		return nil
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode request body %q: %v", data, err)
	}
	return body
}

func TestCleanPlanAndApplyUsesOfficialBatchPayloads(t *testing.T) {
	var requests []fixtureRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			fmt.Fprint(w, `{"clip":{"id":"cl_one","cuts":[{"startTimeMs":50,"durationMs":25}]}}`)
			return
		}
		requests = append(requests, fixtureRequest{Method: request.Method, Path: request.URL.Path, Body: decodeFixtureBody(t, request)})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"clip":{"id":"cl_one","cuts":[{"startTimeMs":50,"durationMs":25}]}}`)
	}))
	defer server.Close()

	api := fixtureClient(server)
	plan, snapshot, err := planCleanClip(api, "vid_one", "cl_one", cleanOptions{
		RemoveFillers: true, RemoveSilences: "natural",
		TimeRanges: []cleanRange{{FromMs: 100, ToMs: 200}, {FromMs: 180, ToMs: 300}},
		WordRanges: []cleanWordRange{{FromWordIndex: 8, ToWordIndex: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	existing := snapshot.Cuts.([]any)
	if len(existing) != 1 || plan.ExistingCuts == nil {
		t.Fatalf("existing cuts not preserved in snapshot/plan: %#v %#v", snapshot.Cuts, plan.ExistingCuts)
	}
	snapshots := []cutSnapshot{snapshot}
	result, err := applyCleanPlans(api, []cleanClipPlan{plan}, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedOps != 4 || result.FailedOps != 0 {
		t.Fatalf("apply result = %#v", result)
	}
	if !reflect.DeepEqual(snapshots[0].ExpectedCuts, snapshot.Cuts) {
		t.Fatalf("expected post-clean cuts = %#v, want %#v", snapshots[0].ExpectedCuts, snapshot.Cuts)
	}
	wantPaths := []string{
		"/v1/videos/vid_one/clips/cl_one/cut",
		"/v1/videos/vid_one/clips/cl_one/cut-by-transcript",
		"/v1/videos/vid_one/clips/cl_one/remove-fillers",
		"/v1/videos/vid_one/clips/cl_one/remove-silences",
	}
	if len(requests) != len(wantPaths) {
		t.Fatalf("requests = %#v", requests)
	}
	for i, want := range wantPaths {
		if requests[i].Path != want {
			t.Fatalf("request %d path = %q, want %q", i, requests[i].Path, want)
		}
	}
	cuts := requests[0].Body["cuts"].([]any)
	if len(cuts) != 1 || cuts[0].(map[string]any)["fromMs"] != float64(100) || cuts[0].(map[string]any)["toMs"] != float64(300) {
		t.Fatalf("batched cut payload = %#v", requests[0].Body)
	}
	if requests[3].Body["mode"] != "natural" {
		t.Fatalf("remove-silences payload = %#v", requests[3].Body)
	}
}

func TestVideosClipsCleanDryRunDoesNotMutate(t *testing.T) {
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			mutations++
		}
		fmt.Fprint(w, `{"clip":{"id":"cl_one","cuts":[]}}`)
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	flags := &rootFlags{dryRun: true, noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosClipsCleanCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"vid_one", "cl_one", "--remove-fillers", "--remove-silences", "natural", "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if mutations != 0 {
		t.Fatalf("dry-run sent %d mutations", mutations)
	}
	if !strings.Contains(output.String(), `"dry_run": true`) || !strings.Contains(output.String(), `"applied": false`) {
		t.Fatalf("dry-run output = %s", output.String())
	}
}

func TestVideosCleanDiscoversAndAppliesEveryClip(t *testing.T) {
	var mu sync.Mutex
	fillers := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/timeline"):
			fmt.Fprint(w, `{"video":{"id":"vid_one"},"clips":[{"id":"cl_one"},{"id":"cl_two"}],"included":[]}`)
		case request.Method == http.MethodGet:
			clipID := request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:]
			fmt.Fprintf(w, `{"clip":{"id":%q,"cuts":[]}}`, clipID)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/remove-fillers"):
			parts := strings.Split(request.URL.Path, "/")
			mu.Lock()
			fillers = append(fillers, parts[len(parts)-2])
			mu.Unlock()
			fmt.Fprint(w, `{"clip":{}}`)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	t.Setenv("HOME", t.TempDir())
	flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newVideosCleanCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"vid_one", "--remove-fillers", "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("video clean: %v\n%s", err, output.String())
	}
	if !reflect.DeepEqual(fillers, []string{"cl_one", "cl_two"}) {
		t.Fatalf("remove-fillers clips = %v", fillers)
	}
	if !strings.Contains(output.String(), `"clip_count": 2`) || !strings.Contains(output.String(), `"applied": true`) {
		t.Fatalf("video clean output = %s", output.String())
	}
}

func TestCleanPartialFailureRequiresManualRecoveryForTouchedClips(t *testing.T) {
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && strings.Contains(request.URL.Path, "cl_two"):
			http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
			return
		case request.Method == http.MethodPost:
			fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":11,"durationMs":20}]}}`)
		case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "cl_one"):
			fmt.Fprint(w, `{"clip":{"cuts":[{"startTimeMs":11,"durationMs":20}]}}`)
		case request.Method == http.MethodPatch:
			patches++
			fmt.Fprint(w, `{"clip":{}}`)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	plans := []cleanClipPlan{
		{VideoID: "vid_one", ClipID: "cl_one", Operations: []cleanOperation{{Op: "remove-fillers"}}},
		{VideoID: "vid_one", ClipID: "cl_two", Operations: []cleanOperation{{Op: "remove-fillers"}}},
	}
	snapshots := []cutSnapshot{
		{VideoID: "vid_one", ClipID: "cl_one", Cuts: []any{map[string]any{"startTimeMs": float64(10), "durationMs": float64(20)}}},
		{VideoID: "vid_one", ClipID: "cl_two", Cuts: []any{map[string]any{"startTimeMs": float64(30), "durationMs": float64(40)}}},
	}
	result, err := applyCleanPlans(fixtureClient(server), plans, snapshots)
	if err == nil {
		t.Fatal("expected partial failure")
	}
	if result.AppliedOps != 1 || result.FailedOps != 1 || result.RecoveryComplete || len(result.Recovery) != 1 {
		t.Fatalf("partial result = %#v", result)
	}
	if patches != 0 || !result.Recovery[0].CurrentMatchesExpected || !result.Recovery[0].ManualRestoreRequired {
		t.Fatalf("automatic restore must not run: patches=%d result=%#v", patches, result)
	}
}

func TestCleanFirstFailureDoesNotRollbackUnmodifiedClip(t *testing.T) {
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		case http.MethodPatch:
			patches++
			fmt.Fprint(w, `{"clip":{}}`)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	plan := cleanClipPlan{VideoID: "vid_one", ClipID: "cl_one", Operations: []cleanOperation{{Op: "remove-fillers"}}}
	snapshot := cutSnapshot{VideoID: "vid_one", ClipID: "cl_one", Cuts: []any{map[string]any{"startTimeMs": float64(10), "durationMs": float64(20)}}}
	result, err := applyCleanPlans(fixtureClient(server), []cleanClipPlan{plan}, []cutSnapshot{snapshot})
	if err == nil {
		t.Fatal("expected first operation to fail")
	}
	if patches != 0 || len(result.Recovery) != 0 || result.RecoveryComplete {
		t.Fatalf("first failure patches=%d result=%#v", patches, result)
	}
}
