// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestThumbnailInpointSendsStartTimeMs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		newCmd func(*rootFlags) *cobra.Command
		args   []string
		path   string
	}{
		{"video", newVideosThumbnailGetVideoCmd, []string{"vid_one"}, "/v1/videos/vid_one/thumbnail"},
		{"clip", newVideosClipsGetThumbnailCmd, []string{"vid_one", "cl_one"}, "/v1/videos/vid_one/clips/cl_one/thumbnail"},
		{"source", newVideosClipsGetSourceThumbnailCmd, []string{"vid_one", "cl_one", "src_one"}, "/v1/videos/vid_one/clips/cl_one/sources/src_one/thumbnail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				requests++
				if request.Method != http.MethodGet || request.URL.Path != tc.path {
					t.Errorf("request = %s %s, want GET %s", request.Method, request.URL.Path, tc.path)
				}
				query := request.URL.Query()
				if query.Get("startTimeMs") != "1000" || query.Has("inpointMs") || query.Get("response") != "json" {
					t.Errorf("query = %v, want startTimeMs=1000, response=json and no inpointMs", query)
				}
				if body := decodeFixtureBody(t, request); body != nil {
					t.Errorf("GET body = %v, want empty", body)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"url":"https://cdn.example/thumbnail.jpg"}`)
			}))
			defer server.Close()
			t.Setenv("TELLA_BASE_URL", server.URL)
			t.Setenv("TELLA_API_KEY", "fixture")
			t.Setenv("HOME", t.TempDir())
			flags := &rootFlags{
				asJSON: true, noCache: true, timeout: 2 * time.Second,
				configPath: filepath.Join(t.TempDir(), "missing.toml"),
			}
			cmd := tc.newCmd(flags)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(append(tc.args, "--inpoint-ms", "1000", "--response", "json"))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("thumbnail: %v\n%s", err, output.String())
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
			var result struct {
				Results struct {
					URL string `json:"url"`
				} `json:"results"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("decode output: %v\n%s", err, output.String())
			}
			if result.Results.URL != "https://cdn.example/thumbnail.jpg" {
				t.Fatalf("thumbnail output = %s", output.String())
			}
		})
	}
}
