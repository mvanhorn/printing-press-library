// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/cliutil/testenv"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNapPlanningProtocol(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		empty     bool
		wantError int
	}{
		{"query-headers-and-bounds", []string{"campsite", "discover", "--region", "kanto", "--power", "--limit", "2", "--check-in", "2026-10-05", "--check-out", "2026-10-06", "--json"}, false, 0},
		{"negative-discovery-empty", []string{"campsite", "discover", "--keyword", "no-such-campsite", "--limit", "2", "--json"}, true, 0},
		{"typed-throttle", []string{"pitch", "inspect", "11007", "20005062", "--json"}, false, 7},
		{"normal-throttle-timeout", []string{"pitch", "inspect", "11007", "20005062", "--timeout", "100ms", "--json"}, false, 5},
		{"dry-run-skips-files", []string{"planner", "changes", "/missing/before.json", "/missing/after.json", "--dry-run", "--json"}, false, 0},
		{"incompatible-data-source", []string{"planner", "fit", "11007", "20005062", "--data-source", "local", "--json"}, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			if tc.name == "typed-throttle" {
				t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
			}
			if tc.name == "normal-throttle-timeout" {
				t.Setenv("PRINTING_PRESS_DOGFOOD", "")
				t.Setenv("PRINTING_PRESS_VERIFY", "")
			}
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("X-Requested-With") != "XMLHttpRequest" || r.Header.Get("X-VERSION-2") != "1" {
					t.Errorf("source headers lost: %v", r.Header)
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.wantError == 7 || tc.name == "normal-throttle-timeout" {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(429)
					fmt.Fprint(w, "{\"message\":\"source throttled\"}")
					return
				}
				if tc.empty {
					fmt.Fprint(w, "{\"info\":{\"total_count\":0},\"result\":[]}")
					return
				}
				q := r.URL.Query()
				if q.Get("other") != "26,43" || q.Get("region_id") != "2" || q.Get("count") != "2" || q.Get("page_id") != "1" || q.Get("check_in") != "2026-10-05" || q.Get("check_out") != "2026-10-06" {
					t.Errorf("dropped/misnamed source query: %v", q)
				}
				b, e := os.ReadFile(filepath.Join("..", "napcamp", "testdata", "search-kanto.json"))
				if e != nil {
					t.Fatal(e)
				}
				w.Write(b)
			}))
			defer srv.Close()
			t.Setenv("NAP_CAMP_BASE_URL", srv.URL)
			root := RootCmd()
			var out, errs bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errs)
			root.SetArgs(tc.args)
			e := root.Execute()
			code := 0
			if e != nil {
				code = ExitCode(e)
			}
			if code != tc.wantError {
				t.Fatalf("exit=%d want=%d err=%v stderr=%s", ExitCode(e), tc.wantError, e, errs.String())
			}
			if tc.wantError != 0 {
				if tc.name == "normal-throttle-timeout" && (e == nil || !strings.Contains(e.Error(), "Nap Camp source GET") || out.Len() != 0) {
					t.Fatal("throttle must fail with provider context and no availability data", e, out.String())
				}
				return
			}
			var j map[string]any
			if e := json.Unmarshal(out.Bytes(), &j); e != nil {
				t.Fatalf("not JSON: %s", out.String())
			}
			if tc.name == "dry-run-skips-files" {
				if requests != 0 || j["dry_run"] != true {
					t.Fatal("dry-run performed I/O", j, requests)
				}
				return
			}
			results, ok := j["results"].([]any)
			if !ok {
				t.Fatal(j)
			}
			if tc.empty {
				if len(results) != 0 || strings.Contains(out.String(), "\"results\": null") {
					t.Fatal("empty results wrong", out.String())
				}
				return
			}
			if len(results) != 2 {
				t.Fatal("source bound lost", len(results))
			}
			for _, v := range results {
				p := v.(map[string]any)["plan_previews"].([]any)
				if len(p) > 3 {
					t.Fatal("preview bound lost")
				}
			}
		})
	}
}

func TestNapZeroResultOutputModes(t *testing.T) {
	for _, mode := range []string{"--json", "--agent", "--csv"} {
		t.Run(mode, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, "{\"info\":{\"total_count\":0},\"result\":[]}")
			}))
			defer srv.Close()
			t.Setenv("NAP_CAMP_BASE_URL", srv.URL)
			root := RootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"campsite", "discover", "--keyword", "no-such-campsite", mode})
			if e := root.Execute(); e != nil {
				t.Fatal(e)
			}
			if mode == "--csv" {
				if strings.TrimSpace(out.String()) == "[]" {
					t.Fatal("JSON literal in CSV stream")
				}
				r := csv.NewReader(strings.NewReader(out.String()))
				for {
					_, e := r.Read()
					if e == io.EOF {
						break
					}
					if e != nil {
						t.Fatal("invalid CSV", e)
					}
				}
				return
			}
			var j ObjectForZero
			if e := json.Unmarshal(out.Bytes(), &j); e != nil {
				t.Fatal("non-JSON machine output", out.String(), e)
			}
			if mode == "--agent" {
				j = j["results"].(map[string]any)
			}
			rows, ok := j["results"].([]any)
			if !ok || len(rows) != 0 {
				t.Fatal("zero result did not retain empty array", j)
			}
		})
	}
}

type ObjectForZero = map[string]any
