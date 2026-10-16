// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
)

// TestNovelCoverageHelpWires smoke-tests that the coverage command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCoverageHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"coverage", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("coverage --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "coverage"} {
		if !strings.Contains(help, want) {
			t.Fatalf("coverage --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSamePagePath(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"https://www.Example.test/Story/", "http://example.test/Story", true},
		{"https://example.test/Story", "/Story/", true},
		{"https://one.test/story", "https://two.test/story", false},
		{"/Story", "/story", false},
	} {
		t.Run(tc.a+"|"+tc.b, func(t *testing.T) {
			if got := samePagePath(tc.a, tc.b); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizePagePathDecodesEscapes(t *testing.T) {
	paths := []string{"/caf%c3%a9/", "/café", "/caf%C3%A9"}
	for _, path := range paths[1:] {
		if !samePagePath(paths[0], path) {
			t.Errorf("%q and %q should normalize equally", paths[0], path)
		}
	}
}

func TestIsNoDataResponse(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		want bool
	}{
		{"empty", nil, true},
		{"null", []byte(`null`), true},
		{"no data object", []byte(`{"response":"No Data"}`), true},
		{"array", []byte(`[]`), false},
		{"other object", []byte(`{"response":"ok"}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNoDataResponse(tc.raw); got != tc.want {
				t.Fatalf("isNoDataResponse(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestInspectionDecode(t *testing.T) {
	for _, tc := range []struct {
		raw string
		n   int
		bad bool
	}{{`[]`, 0, false}, {`{"response":"No Data"}`, 0, false}, {`null`, 0, false}, {`[{"page":"/story","index_verdict":"PASS"}]`, 1, false}, {`{"unexpected":true}`, 0, true}, {`broken`, 0, true}} {
		t.Run(tc.raw, func(t *testing.T) {
			rows, err := decodeInspectionRows([]byte(tc.raw))
			if (err != nil) != tc.bad || (!tc.bad && (rows == nil || len(rows) != tc.n)) {
				t.Fatalf("rows=%+v err=%v", rows, err)
			}
			if tc.n == 1 && rows[0].Verdict != "PASS" {
				t.Fatal(rows)
			}
		})
	}
}
func TestCoverageClassification(t *testing.T) {
	for _, tc := range []struct {
		name, verdict, state, want string
		noindex, missing           bool
	}{{"pass", "PASS", "", "indexed", false, false}, {"negative", "NEUTRAL", "Crawled - currently not indexed", "not_indexed", false, false}, {"fail", "FAIL", "Submitted and indexed", "not_indexed", false, false}, {"positive", "VERDICT_UNSPECIFIED", "Submitted and indexed", "indexed", false, false}, {"unknown", "VERDICT_UNSPECIFIED", "Unknown", "not_indexed", false, false}, {"missing", "", "", "never_inspected", false, true}, {"intentional", "PASS", "", "noindex_intentional", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []inspectionRow{{Page: "/STORY/", Verdict: tc.verdict, CoverageState: tc.state, Robots: "ALLOWED"}}
			if tc.missing {
				rows = nil
			}
			r := classifyCoverage([]publishedInventoryItem{{URL: "http://example.com/STORY", PostID: 42}}, rows, map[string]bool{"https://example.com/STORY/": tc.noindex}, true)
			if r.Published != 1 || r.Summary[tc.want] != 1 {
				t.Fatalf("%+v", r)
			}
			if tc.want != "indexed" && (len(r.Gaps) != 1 || r.Gaps[0].PostID != 42 || r.Gaps[0].Status != tc.want) {
				t.Fatal(r)
			}
		})
	}
	r := classifyCoverage(nil, nil, nil, true)
	if r.Published != 0 || r.Gaps == nil || len(r.Gaps) != 0 {
		t.Fatal(r)
	}
}

func TestCoverageIncompleteScanMarksMissingRowsUnknown(t *testing.T) {
	inventory := []publishedInventoryItem{{URL: "https://example.com/known"}, {URL: "https://example.com/unseen"}}
	rows := []inspectionRow{{Page: "https://example.com/known", Verdict: "PASS"}}
	for _, tc := range []struct {
		complete bool
		want     string
		gaps     int
	}{
		{complete: false, want: "unknown", gaps: 0},
		{complete: true, want: "never_inspected", gaps: 1},
	} {
		got := classifyCoverage(inventory, rows, nil, tc.complete)
		if got.Summary[tc.want] != 1 || len(got.Gaps) != tc.gaps {
			t.Fatalf("complete=%v: result=%+v", tc.complete, got)
		}
	}
}

func TestCoverageResultJSONIncludesScanStatusAndNote(t *testing.T) {
	result := emptyCoverageResult()
	result.ScanComplete = false
	result.Note = "raise --max-scan-pages"
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["scan_complete"] != false || decoded["note"] != result.Note {
		t.Fatalf("JSON=%s", raw)
	}
}
