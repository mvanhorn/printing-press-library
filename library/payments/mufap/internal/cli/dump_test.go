// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// TestNovelExportHelpWires smoke-tests that the export command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelExportHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"dump", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dump --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dump"} {
		if !strings.Contains(help, want) {
			t.Fatalf("dump --help missing %q in output:\n%s", want, help)
		}
	}
}

// exportTestMirror builds a MUFAP mirror holding n daily-returns rows on one
// date and returns its path.
func exportTestMirror(t *testing.T, n int) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.EnsureMUFAPSchema(context.Background(), s); err != nil {
		t.Fatalf("ensure mufap schema: %v", err)
	}
	rows := make([]store.MUFAPRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, store.MUFAPRow{
			Key:     fmt.Sprintf("Income|Money Market|Fund %02d", i),
			Payload: fmt.Sprintf(`{"Fund Name":"Fund %02d","30 Days":"1.%02d"}`, i, i),
		})
	}
	if err := store.SaveMUFAPDate(context.Background(), s, "daily-returns", "2026-09-04", rows, time.Now()); err != nil {
		t.Fatalf("save mufap date: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	return dbPath
}

func exportTestRun(t *testing.T, args ...string) (string, string) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v error = %v\nstdout:\n%s\nstderr:\n%s", args, err, out.String(), errBuf.String())
	}
	return out.String(), errBuf.String()
}

// TestNovelExportEmptyCasesShareOneShape pins that all three empty states --
// no mirror file, a mirror carrying no MUFAP schema, and a range with no rows
// -- emit the same document in the requested --format. The missing-mirror guard
// used to emit a JSON array regardless of --format, so `export --format csv`
// handed a loader "[]" where every other empty path handed it a header row.
func TestNovelExportEmptyCasesShareOneShape(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "missing.db")

	noSchema := filepath.Join(t.TempDir(), "noschema.db")
	s, err := store.OpenWithContext(context.Background(), noSchema)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	populated := exportTestMirror(t, 3)

	for _, tc := range []struct {
		name string
		db   string
		args []string
	}{
		{"no mirror file", absent, nil},
		{"no mufap schema", noSchema, nil},
		{"no rows in range", populated, []string{"--from", "2020-01-01", "--to", "2020-01-02"}},
	} {
		csvArgs := append([]string{"dump", "daily-returns", "--db", tc.db, "--format", "csv"}, tc.args...)
		gotCSV, _ := exportTestRun(t, csvArgs...)
		if gotCSV != "date,row_key,observed_at\n" {
			t.Errorf("%s: csv empty dump = %q, want the bare identity header", tc.name, gotCSV)
		}
		jsonlArgs := append([]string{"dump", "daily-returns", "--db", tc.db, "--format", "jsonl"}, tc.args...)
		gotJSONL, _ := exportTestRun(t, jsonlArgs...)
		if gotJSONL != "" {
			t.Errorf("%s: jsonl empty dump = %q, want zero lines", tc.name, gotJSONL)
		}
	}
}

// TestNovelExportLimitCapsRows pins that --limit caps the dump. The cap is
// pushed into SQL as LIMIT rather than slicing a fully loaded table, which is
// what keeps `--limit 5` from reading a multi-year panel into memory first.
func TestNovelExportLimitCapsRows(t *testing.T) {
	dbPath := exportTestMirror(t, 6)
	out, _ := exportTestRun(t, "dump", "daily-returns", "--db", dbPath, "--limit", "2")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("--limit 2 emitted %d line(s), want 2:\n%s", len(lines), out)
	}
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %q is not JSON: %v", line, err)
		}
		if rec["row_key"] == nil {
			t.Errorf("line %q lost its row_key", line)
		}
	}
}

// TestNovelExportSelectNarrowsRecords pins that --select is honoured rather
// than accepted and ignored. A streamed dump cannot go through
// printJSONFiltered, so the projection has to happen per record -- including on
// the identity columns, or the CSV header keeps a column of blanks.
func TestNovelExportSelectNarrowsRecords(t *testing.T) {
	dbPath := exportTestMirror(t, 2)

	gotCSV, _ := exportTestRun(t, "dump", "daily-returns", "--db", dbPath, "--format", "csv", "--select", "date,row_key")
	header := strings.SplitN(gotCSV, "\n", 2)[0]
	if header != "date,row_key" {
		t.Errorf("csv header with --select date,row_key = %q, want %q", header, "date,row_key")
	}

	gotJSONL, _ := exportTestRun(t, "dump", "daily-returns", "--db", dbPath, "--select", "Fund Name")
	first := strings.SplitN(strings.TrimRight(gotJSONL, "\n"), "\n", 2)[0]
	var rec map[string]any
	if err := json.Unmarshal([]byte(first), &rec); err != nil {
		t.Fatalf("line %q is not JSON: %v", first, err)
	}
	if len(rec) != 1 || rec["Fund Name"] == nil {
		t.Errorf("--select \"Fund Name\" kept %v, want only the Fund Name column", rec)
	}
}

// TestNovelExportJSONFormatIsValidDocument pins the --format json contract:
// one array document, and an EMPTY array rather than empty stdout when the
// mirror holds nothing. jsonl correctly emits zero lines in that case, which
// leaves an agent with nothing to parse; this format is the shaped answer, so
// a regression back to empty output would silently break agent consumers.
func TestNovelExportJSONFormatIsValidDocument(t *testing.T) {
	dir := t.TempDir()
	cmd := RootCmd()
	cmd.SetArgs([]string{"dump", "daily-returns", "--format", "json", "--db", dir + "/absent.db"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export --format json against an absent mirror: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got == "" {
		t.Fatal("empty stdout: an absent mirror must still yield a parseable empty document")
	}
	var env struct {
		Resource string           `json:"resource"`
		Rows     []map[string]any `json:"rows"`
		Count    int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(got), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v (got %q)", err, got)
	}
	if len(env.Rows) != 0 || env.Count != 0 {
		t.Fatalf("expected 0 records from an absent mirror, got %d/%d", len(env.Rows), env.Count)
	}
	// An empty result must still say WHAT is empty.
	if env.Resource != "daily-returns" {
		t.Fatalf("envelope lost its provenance: resource = %q, want %q", env.Resource, "daily-returns")
	}
}

// exportTestAccountingMirror stores one daily-returns row written the way
// MUFAP writes them: an accounting negative ("(4.97)" is -4.97), a plain
// positive, a fund that did not report ("N/A"), and label columns.
func exportTestAccountingMirror(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.EnsureMUFAPSchema(context.Background(), s); err != nil {
		t.Fatalf("ensure mufap schema: %v", err)
	}
	rows := []store.MUFAPRow{{
		Key: "Dedicated Equity Funds|Dedicated Equity (Absolute Return )|Alfalah GHP Dedicated Equity Fund",
		Payload: `{"Fund Name":"Alfalah GHP Dedicated Equity Fund",` +
			`"Sector":"Dedicated Equity Funds","Category":"Dedicated Equity (Absolute Return )",` +
			`"YTD":"(4.97)","30 Days":"0.22","NAV":"203.9071","Benchmark":"N/A",` +
			`"Validity Date":"Sep 04, 2026"}`,
	}}
	if err := store.SaveMUFAPDate(context.Background(), s, "daily-returns", "2026-09-04", rows, time.Now()); err != nil {
		t.Fatalf("save mufap date: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	return dbPath
}

// exportTestDecodeLine decodes one JSONL record with UseNumber, so a cell that
// came out as a JSON number is distinguishable from one that came out as text.
func exportTestDecodeLine(t *testing.T, line string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var rec map[string]any
	if err := dec.Decode(&rec); err != nil {
		t.Fatalf("record %q is not JSON: %v", line, err)
	}
	return rec
}

// TestNovelExportDecodesAccountingNegatives is the regression guard for the
// measured MUFAP quirk in section I of the endpoint contract: MUFAP writes
// every negative in accounting notation and never with a minus sign, so a dump
// that passes cells through as text hands a research loader "(4.97)" in a
// numeric column. 96 of 388 YTD cells on 2026-09-04 were parenthesised; a
// consumer parsing floats failed on 97 of them, kept 291 and averaged +9.29
// where the full 387 reporting rows average +2.45. The dump therefore decodes by default, and
// --raw-values is the way back to MUFAP's own text.
func TestNovelExportDecodesAccountingNegatives(t *testing.T) {
	dbPath := exportTestAccountingMirror(t)

	gotJSONL, _ := exportTestRun(t, "dump", "daily-returns", "--db", dbPath)
	rec := exportTestDecodeLine(t, strings.TrimRight(gotJSONL, "\n"))

	ytd, ok := rec["YTD"].(json.Number)
	if !ok {
		t.Fatalf("YTD = %#v (%T), want a JSON number; MUFAP's \"(4.97)\" must not survive as text", rec["YTD"], rec["YTD"])
	}
	if got, err := ytd.Float64(); err != nil || got != -4.97 {
		t.Fatalf("YTD = %v (err %v), want -4.97: the accounting negative lost its sign", ytd, err)
	}
	// A positive cell decodes too, keeping MUFAP's own digits.
	if nav, isNum := rec["NAV"].(json.Number); !isNum || nav.String() != "203.9071" {
		t.Errorf("NAV = %#v, want the JSON number 203.9071", rec["NAV"])
	}
	// Cells MUFAP did not publish as numbers stay text: a fund that did not
	// report is never coerced to a zero, and labels are not touched.
	for key, want := range map[string]string{
		"Benchmark":     "N/A",
		"Validity Date": "Sep 04, 2026",
		"Fund Name":     "Alfalah GHP Dedicated Equity Fund",
		"Sector":        "Dedicated Equity Funds",
	} {
		if got, isStr := rec[key].(string); !isStr || got != want {
			t.Errorf("%s = %#v, want the string %q", key, rec[key], want)
		}
	}
	// Identity columns are untouched.
	if rec["date"] != "2026-09-04" || rec["row_key"] == nil || rec["observed_at"] == nil {
		t.Errorf("identity columns changed shape: %#v", rec)
	}

	// CSV carries a plain signed decimal in the numeric column.
	gotCSV, _ := exportTestRun(t, "dump", "daily-returns", "--db", dbPath, "--format", "csv")
	records, err := csv.NewReader(strings.NewReader(gotCSV)).ReadAll()
	if err != nil {
		t.Fatalf("csv dump is not parseable: %v\n%s", err, gotCSV)
	}
	if len(records) != 2 {
		t.Fatalf("csv dump has %d record(s), want a header and one row:\n%s", len(records), gotCSV)
	}
	col := -1
	for i, h := range records[0] {
		if h == "YTD" {
			col = i
		}
	}
	if col < 0 {
		t.Fatalf("csv header has no YTD column: %v", records[0])
	}
	if records[1][col] != "-4.97" {
		t.Errorf("csv YTD cell = %q, want %q", records[1][col], "-4.97")
	}

	// The escape hatch returns MUFAP's original text verbatim.
	gotRaw, _ := exportTestRun(t, "dump", "daily-returns", "--db", dbPath, "--raw-values")
	rawRec := exportTestDecodeLine(t, strings.TrimRight(gotRaw, "\n"))
	if got, isStr := rawRec["YTD"].(string); !isStr || got != "(4.97)" {
		t.Errorf("--raw-values YTD = %#v, want the string %q", rawRec["YTD"], "(4.97)")
	}
	if got, isStr := rawRec["NAV"].(string); !isStr || got != "203.9071" {
		t.Errorf("--raw-values NAV = %#v, want the string %q", rawRec["NAV"], "203.9071")
	}
}
