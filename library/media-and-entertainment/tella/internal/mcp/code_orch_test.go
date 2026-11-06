// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestCodeOrchTranscriptReadsUseLiveRoute(t *testing.T) {
	for _, tc := range []struct {
		name       string
		endpointID string
		override   bool
		wantQuery  url.Values
	}{
		{"cut", "videos.clips.get-cut-transcript", false, url.Values{}},
		{"uncut", "videos.clips.get-uncut-transcript", false, url.Values{"includeCut": {"true"}}},
		{"uncut forces includeCut", "videos.clips.get-uncut-transcript", true, url.Values{"includeCut": {"true"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const response = `{"words":[{"index":0,"text":"hello","startTimeMs":0,"endTimeMs":300}]}`
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls++
				if request.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", request.Method)
				}
				if request.URL.Path != "/v1/videos/vid_one/clips/cl_one/transcript" {
					t.Errorf("unexpected path: %s", request.URL.Path)
					http.NotFound(w, request)
					return
				}
				if !reflect.DeepEqual(request.URL.Query(), tc.wantQuery) {
					t.Errorf("query = %v, want %v", request.URL.Query(), tc.wantQuery)
				}
				body, err := io.ReadAll(request.Body)
				if err != nil || len(body) != 0 {
					t.Errorf("GET body = %q, error = %v; want empty", body, err)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, response)
			}))
			defer server.Close()
			t.Setenv("TELLA_BASE_URL", server.URL)
			t.Setenv("TELLA_API_KEY", "fixture")
			t.Setenv("TELLA_SESSION_COOKIE", "")

			params := map[string]any{"id": "vid_one", "clipId": "cl_one"}
			if tc.override {
				params["includeCut"] = false
			}
			var req mcplib.CallToolRequest
			req.Params.Name = "tella_read"
			req.Params.Arguments = map[string]any{"endpoint_id": tc.endpointID, "params": params}
			result, err := handleCodeOrchRead(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			if result == nil || result.IsError || len(result.Content) != 1 {
				t.Fatalf("result = %#v, want successful text response", result)
			}
			content, ok := result.Content[0].(mcplib.TextContent)
			if !ok || content.Text != response || calls != 1 {
				t.Fatalf("content = %#v, calls = %d", result.Content, calls)
			}
		})
	}
}
