// Copyright 2026 Chris Drit and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// printRows runs printJSONOrTableWithMeta into a buffer, which isTerminal
// treats as piped output — the case where --csv and --plain were ignored
// (issue #2453).
func printRows(t *testing.T, flags *rootFlags, rows []map[string]any, meta map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := printJSONOrTableWithMeta(cmd, flags, rows, meta); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

var formatTestRows = []map[string]any{
	{"form": "6-K", "accession": "0001009001-26-000012", "value": 132175000000.0},
	{"form": "10-K", "accession": "0001009001-26-000001", "value": 4404000000.0},
}

var formatTestMeta = map[string]any{"total_hits": 2, "returned": 2}

func TestPrintJSONOrTableCSVWhenPiped(t *testing.T) {
	got := printRows(t, &rootFlags{csv: true}, formatTestRows, formatTestMeta)
	want := "accession,form,value\n" +
		"0001009001-26-000012,6-K,132175000000\n" +
		"0001009001-26-000001,10-K,4404000000\n"
	if got != want {
		t.Fatalf("--csv output:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintJSONOrTablePlainWhenPiped(t *testing.T) {
	rows := []map[string]any{{"form": "8-K", "display": "Acme\tCorp\nInc"}}
	got := printRows(t, &rootFlags{plain: true}, rows, formatTestMeta)
	want := "display\tform\nAcme Corp Inc\t8-K\n"
	if got != want {
		t.Fatalf("--plain output:\n%q\nwant:\n%q", got, want)
	}
}

func TestPrintJSONOrTableCSVWinsOverJSON(t *testing.T) {
	got := printRows(t, &rootFlags{csv: true, asJSON: true}, formatTestRows, formatTestMeta)
	if !strings.HasPrefix(got, "accession,form,value\n") {
		t.Fatalf("--csv --json should print CSV, got:\n%s", got)
	}
}

func TestPrintJSONOrTableCSVHonorsSelect(t *testing.T) {
	got := printRows(t, &rootFlags{csv: true, selectFields: "form"}, formatTestRows, nil)
	want := "form\n6-K\n10-K\n"
	if got != want {
		t.Fatalf("--csv --select form output:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintJSONOrTableCSVEmptyRows(t *testing.T) {
	for _, rows := range [][]map[string]any{nil, {}} {
		if got := printRows(t, &rootFlags{csv: true}, rows, formatTestMeta); got != "" {
			t.Fatalf("empty --csv output = %q, want empty", got)
		}
		if got := printRows(t, &rootFlags{plain: true}, rows, formatTestMeta); got != "" {
			t.Fatalf("empty --plain output = %q, want empty", got)
		}
	}
}

func TestPrintJSONOrTableQuietSuppressesCSV(t *testing.T) {
	if got := printRows(t, &rootFlags{csv: true, quiet: true}, formatTestRows, nil); got != "" {
		t.Fatalf("--csv --quiet output = %q, want empty", got)
	}
}

// Without a format flag, piped output keeps the JSON envelope with meta.
func TestPrintJSONOrTableDefaultPipedEnvelope(t *testing.T) {
	got := printRows(t, &rootFlags{}, formatTestRows, formatTestMeta)
	var env struct {
		Results []map[string]any `json:"results"`
		Meta    map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal([]byte(got), &env); err != nil {
		t.Fatalf("piped output is not the JSON envelope: %v\n%s", err, got)
	}
	if len(env.Results) != 2 || env.Meta["returned"] != float64(2) {
		t.Fatalf("envelope = %+v", env)
	}
}

// --agent sets --compact, whose generic allow-list matches none of these
// commands' columns; with --csv or --plain it must not blank every row.
func TestPrintJSONOrTableAgentKeepsRowColumns(t *testing.T) {
	rows := []map[string]any{{"form": "8-K", "file_date": "2026-11-02"}}
	agent := rootFlags{agent: true, asJSON: true, compact: true}
	csvFlags := agent
	csvFlags.csv = true
	if got, want := printRows(t, &csvFlags, rows, nil), "file_date,form\n2026-11-02,8-K\n"; got != want {
		t.Fatalf("--agent --csv output:\n%q\nwant:\n%q", got, want)
	}
	plainFlags := agent
	plainFlags.plain = true
	if got, want := printRows(t, &plainFlags, rows, nil), "file_date\tform\n2026-11-02\t8-K\n"; got != want {
		t.Fatalf("--agent --plain output:\n%q\nwant:\n%q", got, want)
	}
}
