// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelGeoResolveHelpWires smoke-tests that the geo resolve command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelGeoResolveHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"geo", "resolve", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("geo resolve --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "resolve"} {
		if !strings.Contains(help, want) {
			t.Fatalf("geo resolve --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestCampaignLocationIDs(t *testing.T) {
	cases := []struct {
		name string
		data string
		want []string
	}{
		{"single", `{"targeting":{"locations":{"include":[{"id":"US","type":"COUNTRY"}]}}}`, []string{"US"}},
		{"multiple", `{"targeting":{"locations":{"include":[{"id":"US"},{"id":"MX"}]}}}`, []string{"US", "MX"}},
		{"no targeting", `{"name":"x"}`, nil},
		{"empty include", `{"targeting":{"locations":{"include":[]}}}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := campaignLocationIDs(tc.data)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestNovelGeoResolve_EmptyStore(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "geo", "resolve", "--json")
	if err != nil {
		t.Fatalf("geo resolve: %v", err)
	}
	var rows []geoResolveRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("geo resolve json: %v\n%s", err, out)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty rows, got %+v", rows)
	}
}

func TestNovelGeoResolve_LocalOnly(t *testing.T) {
	novelSeedStore(t)
	// No live credentials in the test env and --data-source local forbids a
	// live resolve, so the id cannot be resolved. The command must still
	// succeed, but report the row as unresolved with an empty name — echoing
	// the id back as its own name would make total failure look like success.
	out, err := runNovelCmd(t, "geo", "resolve", "--json", "--data-source", "local")
	if err != nil {
		t.Fatalf("geo resolve: %v", err)
	}
	var rows []geoResolveRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("geo resolve json: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0].LocationID != "1000156" {
		t.Fatalf("expected location 1000156 listed, got %+v", rows)
	}
	if rows[0].Resolved {
		t.Fatalf("expected unresolved row without a live client, got %+v", rows[0])
	}
	if rows[0].Name != "" {
		t.Fatalf("unresolved row must not alias the id as its name, got %q", rows[0].Name)
	}
	if len(rows[0].Campaigns) != 1 || rows[0].Campaigns[0] != "cmpn_1" {
		t.Fatalf("unexpected campaigns for location: %+v", rows[0].Campaigns)
	}
}
