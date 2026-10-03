// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/cliutil/testenv"
)

func TestAgentSelectionKeepsEnvelope(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"planner", "changes", filepath.Join("..", "napcamp", "testdata", "before.json"), filepath.Join("..", "napcamp", "testdata", "after.json"), "--agent", "--select", "campsite_id"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data, dataOK := envelope["results"].(map[string]any)
	meta, metaOK := envelope["meta"].(map[string]any)
	if !dataOK || !metaOK || data["campsite_id"] != "11007" || meta["source"] == nil {
		t.Fatalf("projection dropped envelope: %s", out.String())
	}
}

func TestPartialPitchComparisonExitsFailure(t *testing.T) {
	for _, mode := range []string{"--json", "--agent"} {
		t.Run(mode, func(t *testing.T) {
			testenv.Isolate(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/campsite/11008" {
					http.Error(w, "missing fixture", http.StatusNotFound)
					return
				}
				name := "campsite-11007.json"
				if r.URL.Path == "/api/campsite/11007/plans/20005062" {
					name = "plan-20005062.json"
				}
				data, err := os.ReadFile(filepath.Join("..", "napcamp", "testdata", name))
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				_, _ = w.Write(data)
			}))
			defer server.Close()
			t.Setenv("NAP_CAMP_BASE_URL", server.URL)
			root := RootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"planner", "compare", "11007:20005062", "11008:20005100", mode})
			err := root.Execute()
			if err == nil || ExitCode(err) == 0 {
				t.Fatalf("partial comparison exited success: %s", out.String())
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if mode == "--agent" {
				result = result["results"].(map[string]any)
			}
			results, resultsOK := result["results"].([]any)
			failures, errorsOK := result["errors"].([]any)
			if !resultsOK || !errorsOK || len(results) != 1 || len(failures) != 1 || result["complete"] != false {
				t.Fatalf("partial evidence contract lost: %s", out.String())
			}
		})
	}
}
