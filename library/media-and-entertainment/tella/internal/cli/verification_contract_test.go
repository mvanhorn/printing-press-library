// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func TestRequiredWorkflowInputsWaivedOnlyForOfflineHarness(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cases := []struct {
		name    string
		makeCmd func(*rootFlags) *cobra.Command
		args    []string
	}{
		{"apply-storyboard", newVideosApplyStoryboardCmd, []string{"mock-value"}},
		{"add-broll", newVideosClipsAddBrollCmd, []string{"mock-value", "mock-value", "/mock/path"}},
		{"cut-words", newVideosClipsCutWordsCmd, []string{"mock-value", "mock-value"}},
		{"storyboard-image", newNovelVideosStoryboardImageCmd, []string{"mock-value"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, harness := range []string{"", "PRINTING_PRESS_VERIFY", "PRINTING_PRESS_DOGFOOD"} {
				for _, dryRun := range []bool{false, true} {
					flags := lessonTestFlags(t, server)
					flags.dryRun, flags.asJSON = dryRun, true
					if harness != "" {
						t.Setenv(harness, "1")
					}
					out, err := runLessonCommand(tc.makeCmd(flags), tc.args...)
					if harness != "" && dryRun {
						if err != nil {
							t.Fatalf("%s dry run: %v", harness, err)
						}
						var intent struct {
							DryRun   bool     `json:"dry_run"`
							Workflow string   `json:"workflow"`
							Steps    []string `json:"steps"`
						}
						if err := json.Unmarshal([]byte(out), &intent); err != nil {
							t.Fatal(err, out)
						}
						if !intent.DryRun || intent.Workflow == "" || len(intent.Steps) == 0 {
							t.Fatalf("missing dry-run intent: %s", out)
						}
					} else if err == nil {
						t.Fatalf("harness=%q dryRun=%v accepted missing inputs", harness, dryRun)
					}
				}
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("made %d network requests", requests.Load())
	}
}
