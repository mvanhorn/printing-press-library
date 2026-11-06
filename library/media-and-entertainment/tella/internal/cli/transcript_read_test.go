// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/store"
	"github.com/spf13/cobra"
)

const transcriptReadPath = "/v1/videos/vid_one/clips/cl_one/transcript"
const transcriptReadCut = `{"words":[{"index":0,"text":"hello","startTimeMs":0,"endTimeMs":300,"hidden":false},{"index":2,"text":"world","startTimeMs":300,"endTimeMs":700,"hidden":false}]}`
const transcriptReadUncut = `{"words":[{"index":0,"text":"hello","startTimeMs":0,"endTimeMs":300,"sourceStartTimeMs":0,"sourceEndTimeMs":300},{"index":1,"text":"oops","cut":true,"sourceStartTimeMs":300,"sourceEndTimeMs":500},{"index":2,"text":"world","startTimeMs":300,"endTimeMs":700,"sourceStartTimeMs":500,"sourceEndTimeMs":900}]}`

func transcriptReadFlags(t *testing.T, server *httptest.Server) *rootFlags {
	t.Helper()
	t.Setenv("TELLA_BASE_URL", server.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	t.Setenv("TELLA_SESSION_COOKIE", "")
	return &rootFlags{asJSON: true, noCache: true, dataSource: "live", timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
}

func executeTranscriptRead(cmd *cobra.Command, args ...string) (string, error) {
	var output, errOutput bytes.Buffer
	cmd.SilenceUsage = true
	cmd.SetOut(&output)
	cmd.SetErr(&errOutput)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func checkTranscriptReadRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", request.Method)
	}
	if body := decodeFixtureBody(t, request); body != nil {
		t.Errorf("GET body = %#v, want empty", body)
	}
}

func TestTranscriptReadAliasesUseLiveRoute(t *testing.T) {
	for _, tc := range []struct {
		name       string
		newCmd     func(*rootFlags) *cobra.Command
		includeCut string
		response   string
	}{
		{"cut", newVideosClipsGetCutTranscriptCmd, "", transcriptReadCut},
		{"uncut", newVideosClipsGetUncutTranscriptCmd, "true", transcriptReadUncut},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				checkTranscriptReadRequest(t, request)
				if request.URL.Path != transcriptReadPath {
					http.NotFound(w, request)
					return
				}
				calls++
				want := url.Values{}
				if tc.includeCut != "" {
					want.Set("includeCut", tc.includeCut)
				}
				if !reflect.DeepEqual(request.URL.Query(), want) {
					t.Errorf("query = %v, want %v", request.URL.Query(), want)
				}
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			cmd := tc.newCmd(transcriptReadFlags(t, server))
			if cmd.Annotations["pp:path"] != "/v1/videos/{id}/clips/{clipId}/transcript" {
				t.Fatal(cmd.Annotations)
			}
			output, err := executeTranscriptRead(cmd, "vid_one", "cl_one")
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Results json.RawMessage `json:"results"`
			}
			if err := json.Unmarshal([]byte(output), &envelope); err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(envelope.Results, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.response), &want); err != nil {
				t.Fatal(err)
			}
			if envelope, ok := got.(map[string]any); ok {
				if value, exists := envelope["results"]; exists {
					got = value
				}
			}
			if calls != 1 || !reflect.DeepEqual(got, want) {
				t.Fatalf("calls=%d output=%s", calls, output)
			}
		})
	}
}

func TestGetTranscriptQueryAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		query url.Values
		bad   bool
	}{
		{name: "defaults", query: url.Values{"view": {"words"}}},
		{name: "all flags", args: []string{"--view", "words", "--include-cut", "--start-ms", "0", "--end-ms", "900"}, query: url.Values{"view": {"words"}, "includeCut": {"true"}, "startTimeMs": {"0"}, "endTimeMs": {"900"}}},
		{name: "sentences", args: []string{"--view", "sentences", "--start-ms", "10"}, query: url.Values{"view": {"sentences"}, "startTimeMs": {"10"}}},
		{name: "text", args: []string{"--view", "text", "--end-ms", "900"}, query: url.Values{"view": {"text"}, "endTimeMs": {"900"}}},
		{name: "bad view", args: []string{"--view", "bad"}, bad: true},
		{name: "empty view", args: []string{"--view="}, bad: true},
		{name: "include cut text", args: []string{"--view", "text", "--include-cut"}, bad: true},
		{name: "include cut sentences", args: []string{"--view", "sentences", "--include-cut"}, bad: true},
		{name: "equal window", args: []string{"--start-ms", "10", "--end-ms", "10"}, bad: true},
		{name: "reversed window", args: []string{"--start-ms", "20", "--end-ms", "10"}, bad: true},
		{name: "negative start", args: []string{"--start-ms", "-1"}, bad: true},
		{name: "zero end", args: []string{"--end-ms", "0"}, bad: true},
		{name: "oversize start", args: []string{"--start-ms", "9007199254740992"}, bad: true},
		{name: "oversize end", args: []string{"--end-ms", "9007199254740992"}, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls++
				checkTranscriptReadRequest(t, request)
				if request.URL.Path != transcriptReadPath {
					http.NotFound(w, request)
					return
				}
				if !reflect.DeepEqual(request.URL.Query(), tc.query) {
					t.Errorf("query=%v want=%v", request.URL.Query(), tc.query)
				}
				fmt.Fprint(w, transcriptReadCut)
			}))
			defer server.Close()
			cmd := newVideosClipsGetTranscriptCmd(transcriptReadFlags(t, server))
			output, err := executeTranscriptRead(cmd, append([]string{"vid_one", "cl_one"}, tc.args...)...)
			if tc.bad {
				if err == nil || ExitCode(err) != 2 || calls != 0 {
					t.Fatalf("error=%v calls=%d output=%s", err, calls, output)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(transcriptReadCut), &want); err != nil {
				t.Fatal(err)
			}
			if envelope, ok := got.(map[string]any); ok {
				if value, exists := envelope["results"]; exists {
					got = value
				}
			}
			if calls != 1 || !reflect.DeepEqual(got, want) {
				t.Fatalf("calls=%d output=%s", calls, output)
			}
		})
	}
}

func TestTranscriptReadCaptionsAndDiff(t *testing.T) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		checkTranscriptReadRequest(t, request)
		if request.URL.Path != transcriptReadPath {
			http.NotFound(w, request)
			return
		}
		queries = append(queries, request.URL.Query())
		if request.URL.Query().Get("includeCut") == "true" {
			fmt.Fprint(w, transcriptReadUncut)
		} else {
			fmt.Fprint(w, transcriptReadCut)
		}
	}))
	defer server.Close()
	flags := transcriptReadFlags(t, server)
	output, err := executeTranscriptRead(newClipsCaptionsCmd(flags), "cl_one", "--video", "vid_one")
	if err != nil {
		t.Fatal(err)
	}
	if output != "1\n00:00:00,000 --> 00:00:00,700\nhello world\n\n" {
		t.Fatalf("SRT=%q", output)
	}
	output, err = executeTranscriptRead(newClipsTranscriptDiffCmd(flags), "cl_one", "--video", "vid_one")
	if err != nil {
		t.Fatal(err)
	}
	var diff struct {
		RemovedWords []removedWord `json:"removed_words"`
		RemovedCount int           `json:"removed_count"`
		UncutLength  int           `json:"uncut_length"`
	}
	if err := json.Unmarshal([]byte(output), &diff); err != nil {
		t.Fatal(err)
	}
	if diff.RemovedCount != 1 || diff.UncutLength != 3 || len(diff.RemovedWords) != 1 || diff.RemovedWords[0].Word != "oops" || diff.RemovedWords[0].Position != 1 {
		t.Fatalf("diff=%s", output)
	}
	if !reflect.DeepEqual(queries, []url.Values{{}, {}, {"includeCut": {"true"}}}) {
		t.Fatalf("queries=%v", queries)
	}
}

func TestTranscriptsSyncFailsOnTranscriptFetch(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusForbidden} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var fetched []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				checkTranscriptReadRequest(t, request)
				switch request.URL.Path {
				case "/v1/videos":
					fmt.Fprint(w, `{"videos":[{"id":"vid_one"}],"hasMore":false}`)
				case "/v1/videos/vid_one/clips":
					fmt.Fprint(w, `{"clips":[{"id":"cl_one"},{"id":"cl_empty"},{"id":"cl_missing"},{"id":"cl_never"}],"hasMore":false}`)
				case transcriptReadPath, "/v1/videos/vid_one/clips/cl_empty/transcript", "/v1/videos/vid_one/clips/cl_missing/transcript":
					fetched = append(fetched, request.URL.Path)
					if len(request.URL.Query()) != 0 {
						t.Errorf("query=%v", request.URL.Query())
					}
					switch request.URL.Path {
					case transcriptReadPath:
						fmt.Fprint(w, transcriptReadCut)
					case "/v1/videos/vid_one/clips/cl_empty/transcript":
						fmt.Fprint(w, `{"words":[]}`)
					default:
						http.Error(w, "transcript unavailable", code)
					}
				default:
					http.NotFound(w, request)
				}
			}))
			defer server.Close()
			dbPath := filepath.Join(t.TempDir(), "transcripts.db")
			output, err := executeTranscriptRead(newTranscriptsSyncCmd(transcriptReadFlags(t, server)), "--db", dbPath)
			wantExit := 3
			if code == http.StatusForbidden {
				wantExit = 4
			}
			if err == nil || ExitCode(err) != wantExit {
				t.Fatalf("error=%v exit=%d output=%s", err, ExitCode(err), output)
			}
			for _, want := range []string{"vid_one", "cl_missing", "after storing 1 transcripts"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error=%v missing %q", err, want)
				}
			}
			var failure struct {
				Code  int    `json:"code"`
				Error string `json:"error"`
			}
			if decodeErr := json.Unmarshal([]byte(output), &failure); decodeErr != nil || failure.Code != wantExit || !strings.Contains(failure.Error, "cl_missing") {
				t.Fatalf("failure envelope = %s, decode error = %v", output, decodeErr)
			}
			if len(fetched) != 3 {
				t.Fatalf("fetched=%v output=%s", fetched, output)
			}
			db, openErr := store.OpenWithContext(t.Context(), dbPath)
			if openErr != nil {
				t.Fatal(openErr)
			}
			defer db.Close()
			hits, searchErr := db.SearchTranscripts("hello", 10)
			if searchErr != nil || len(hits) != 1 {
				t.Fatalf("stored transcript hits=%v error=%v", hits, searchErr)
			}
		})
	}
}

func TestTranscriptReadTermRangesUseLiveRoute(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		checkTranscriptReadRequest(t, request)
		if request.URL.Path != transcriptReadPath {
			http.NotFound(w, request)
			return
		}
		calls++
		if !reflect.DeepEqual(request.URL.Query(), url.Values{"includeCut": {"true"}}) {
			t.Errorf("query=%v", request.URL.Query())
		}
		fmt.Fprint(w, transcriptReadUncut)
	}))
	defer server.Close()
	ranges, err := transcriptTermRanges(t.Context(), fixtureClient(server), "vid_one", "cl_one", "oops")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !reflect.DeepEqual(ranges, []map[string]int{{"fromWordIndex": 1, "toWordIndex": 1}}) {
		t.Fatalf("calls=%d ranges=%v", calls, ranges)
	}
}

func TestTranscriptReadUncutParsers(t *testing.T) {
	var value any
	if err := json.Unmarshal([]byte(transcriptReadUncut), &value); err != nil {
		t.Fatal(err)
	}
	ranges := transcriptTermRangesFromValue(value, "oops")
	if !reflect.DeepEqual(ranges, []map[string]int{{"fromWordIndex": 1, "toWordIndex": 1}}) {
		t.Fatalf("ranges=%v", ranges)
	}
	if transcriptLooksEmpty(json.RawMessage(transcriptReadUncut)) {
		t.Fatal("includeCut words were treated as empty")
	}
	text, timings := extractTranscriptText(json.RawMessage(transcriptReadUncut))
	if text != "hello oops world" || !strings.Contains(timings, `"cut":true`) {
		t.Fatalf("text=%q timings=%s", text, timings)
	}
}

func TestCutWordsDryRunUsesLiveTranscriptQuery(t *testing.T) {
	output, err := executeTranscriptRead(newVideosClipsCutWordsCmd(&rootFlags{dryRun: true, asJSON: true}), "vid_one", "cl_one", "--term", "oops")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Steps []struct {
			Method string            `json:"method"`
			Path   string            `json:"path"`
			Query  map[string]string `json:"query"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(output), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Method != "GET" || plan.Steps[0].Path != transcriptReadPath || !reflect.DeepEqual(plan.Steps[0].Query, map[string]string{"includeCut": "true"}) {
		t.Fatalf("plan=%s", output)
	}
}

func TestTranscriptReadNovelWorkflows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		newCmd func(*rootFlags) *cobra.Command
		args   []string
	}{
		{"silence-map", newVideosClipsSilenceMapCmd, []string{"vid_one", "cl_one"}},
		{"storyboard", newVideosStoryboardCmd, []string{"vid_one", "--include-transcript"}},
		{"audit", newVideosAuditCmd, []string{"vid_one"}},
		{"cut-words", newVideosClipsCutWordsCmd, []string{"vid_one", "cl_one", "--term", "oops"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				checkTranscriptReadRequest(t, request)
				switch request.URL.Path {
				case transcriptReadPath:
					calls++
					if !reflect.DeepEqual(request.URL.Query(), url.Values{"includeCut": {"true"}}) {
						t.Errorf("query=%v", request.URL.Query())
					}
					fmt.Fprint(w, transcriptReadUncut)
				case "/v1/videos/vid_one/clips":
					fmt.Fprint(w, `{"clips":[{"id":"cl_one"}]}`)
				case "/v1/videos/vid_one":
					fmt.Fprint(w, `{"video":{"id":"vid_one"}}`)
				case "/v1/videos/vid_one/clips/cl_one/silences", "/v1/videos/vid_one/clips/cl_one/sources", "/v1/videos/vid_one/clips/cl_one/layouts", "/v1/videos/vid_one/clips/cl_one/zooms", "/v1/videos/vid_one/clips/cl_one/blurs", "/v1/videos/vid_one/clips/cl_one/highlights":
					fmt.Fprint(w, `[]`)
				default:
					http.NotFound(w, request)
				}
			}))
			defer server.Close()
			output, err := executeTranscriptRead(tc.newCmd(transcriptReadFlags(t, server)), tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !json.Valid([]byte(output)) {
				t.Fatalf("calls=%d output=%s", calls, output)
			}
			if tc.name == "cut-words" && !strings.Contains(output, `"fromWordIndex": 1`) {
				t.Fatal(output)
			}
			if tc.name == "audit" && strings.Contains(output, "transcript_unavailable") {
				t.Fatal(output)
			}
		})
	}
}

func TestStoryboardReportsTranscriptReadError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		checkTranscriptReadRequest(t, request)
		switch request.URL.Path {
		case "/v1/videos/vid_one/clips":
			fmt.Fprint(w, `{"clips":[{"id":"cl_one"}]}`)
		case transcriptReadPath:
			calls++
			http.NotFound(w, request)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer server.Close()
	output, err := executeTranscriptRead(newVideosStoryboardCmd(transcriptReadFlags(t, server)), "vid_one", "--include-transcript")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Clips []map[string]any `json:"clips"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result.Clips) != 1 {
		t.Fatalf("calls=%d output=%s", calls, output)
	}
	item := result.Clips[0]
	message, _ := item["uncutTranscriptError"].(string)
	if !strings.Contains(message, "HTTP 404") {
		t.Fatalf("transcript error=%q", message)
	}
	if _, ok := item["uncutTranscript"]; ok {
		t.Fatal("failed transcript emitted as data")
	}
}
