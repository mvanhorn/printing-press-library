// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/carstay"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil/testenv"
	"github.com/spf13/cobra"
)

// TestNovelSpotsCompareHelpWires smoke-tests that the spots compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSpotsCompareHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"spots", "compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spots compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("spots compare --help missing %q in output:\n%s", want, help)
		}
	}
}

// Exercise the actual shortlist handler against deterministic public-contract
// HTTP responses, including failures after a successful first detail.
func TestCarstayShortlistCompleteAndPartialEvidence(t *testing.T) {
	ids := []string{"632c59b82b614b99a252d1b2", "5cff4813839680041631c452"}
	for _, tc := range []struct {
		kind   string
		failed int
	}{{"compare", 0}, {"compare", 1}, {"compare", 2}, {"fit", 1}, {"audit", 1}} {
		t.Run(fmt.Sprintf("%s-failed-%d", tc.kind, tc.failed), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := path.Base(r.URL.Path)
				if tc.failed == 2 || (tc.failed == 1 && id == ids[1]) {
					http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"station":{"_id":%q,"name":"試験の場所","prefecture":"三重県","activityOnly":false,"price":2200,"length":8,"breadth":4}}`, id)
			}))
			defer srv.Close()
			client := carstay.New(srv.URL, 2)
			client.HTTP = srv.Client()
			activity := false
			selected := []carstay.Spot{{ID: ids[0], Prefecture: "三重県", ActivityOnly: &activity}, {ID: ids[1], Prefecture: "三重県", ActivityOnly: &activity}}
			var out, diagnostic bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			cmd.SetErr(&diagnostic)
			flags := &rootFlags{asJSON: true, agent: true, compact: true}
			options := defaultCarstayOptions()
			options.length = 6
			view := carstayView{Meta: carstayMeta(tc.kind, "ja"), FetchFailures: []map[string]string{}}
			err := carstayNovelRun(context.Background(), cmd, flags, client, selected, selected, options, "", view)
			if tc.failed == 0 && err != nil {
				t.Fatalf("complete shortlist failed: %v", err)
			}
			if tc.failed > 0 && (err == nil || ExitCode(err) != 5) {
				t.Fatalf("incomplete evidence reported success: %v", err)
			}
			var envelope struct {
				Meta struct {
					Complete   bool                `json:"detail_fetch_complete"`
					Requested  int                 `json:"requested_stations"`
					Successful int                 `json:"successful_stations"`
					Failures   []map[string]string `json:"fetch_failures"`
				} `json:"meta"`
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatalf("not one agent envelope: %s (%v)", out.String(), err)
			}
			if envelope.Results == nil || len(envelope.Results) != 2-tc.failed || envelope.Meta.Requested != 2 || envelope.Meta.Successful != 2-tc.failed || len(envelope.Meta.Failures) != tc.failed || envelope.Meta.Complete != (tc.failed == 0) {
				t.Fatalf("lost evidence scope: %s", out.String())
			}
			if tc.kind == "compare" && tc.failed == 0 {
				for i, row := range envelope.Results {
					if row["id"] != ids[i] || row["availability"] != "unknown" || row["vehicle_acceptance"] != "unknown" {
						t.Fatalf("comparison changed requested source/unknowns: %#v", row)
					}
				}
			}
		})
	}
}
