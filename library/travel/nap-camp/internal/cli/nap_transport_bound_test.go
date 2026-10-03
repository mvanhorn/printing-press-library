// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/cliutil/testenv"
)

func TestOversizedSourceAndPlannerFailWithoutSuccessEvidence(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		for _, command := range []string{"raw-source", "planner"} {
			t.Run(command+map[bool]string{false: "-plain", true: "-gzip"}[compressed], func(t *testing.T) {
				testenv.Isolate(t)
				t.Setenv("PRINTING_PRESS_DOGFOOD", "")
				t.Setenv("PRINTING_PRESS_VERIFY", "")
				payload := []byte("[]" + strings.Repeat(" ", (4<<20)-1))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if compressed {
						w.Header().Set("Content-Encoding", "gzip")
						writer := gzip.NewWriter(w)
						_, _ = writer.Write(payload)
						_ = writer.Close()
						return
					}
					_, _ = w.Write(payload)
				}))
				defer server.Close()
				t.Setenv("NAP_CAMP_BASE_URL", server.URL)
				root := RootCmd()
				args := []string{"source", "filters", "--json"}
				if command == "planner" {
					args = []string{"planner", "fit", "11007", "20005062", "--json"}
				}
				var out, errs bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&errs)
				root.SetArgs(args)
				err := root.Execute()
				if err == nil || ExitCode(err) == 0 || !strings.Contains(err.Error(), "4 MiB") || !strings.Contains(err.Error(), "Nap Camp") {
					t.Fatalf("overflow did not fail with provider context: err=%v stdout=%s", err, out.String())
				}
				if command == "planner" && out.Len() != 0 {
					t.Fatalf("planner returned availability/data on failure: %s", out.String())
				}
				if out.Len() != 0 {
					var envelope map[string]any
					if e := json.Unmarshal(out.Bytes(), &envelope); e != nil {
						t.Fatalf("error envelope is not JSON: %s", out.String())
					}
					for _, key := range []string{"results", "data", "availability", "inventory", "complete"} {
						if _, present := envelope[key]; present {
							t.Fatalf("raw source returned successful field %s: %s", key, out.String())
						}
					}
				}
			})
		}
	}
}
