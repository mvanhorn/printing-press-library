// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// TestNovelPanelHelpWires smoke-tests that the panel command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPanelHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"panel", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("panel --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "panel"} {
		if !strings.Contains(help, want) {
			t.Fatalf("panel --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelPanelHelpDocumentsTabQuirks pins the two per-tab quirks measured
// against the live site on 2026-09-04, both of which the help text used to
// contradict: tab=payout has no "Validity Date" column (its date is "Payout
// Date"), and tab=pricing/ter return 551 rows for an explicit date against
// returns' 388 because they are current reference data, not a dated panel --
// so their row count must not be read as a universe width.
func TestNovelPanelHelpDocumentsTabQuirks(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"panel", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("panel --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Payout Date", "universe_width_reliable", "reference data"} {
		if !strings.Contains(help, want) {
			t.Errorf("panel --help missing %q; the measured per-tab quirk is undocumented again:\n%s", want, help)
		}
	}
}

// panelTestAccountingMirror stores one daily-returns row exactly as MUFAP
// publishes it, accounting negative included.
func panelTestAccountingMirror(t *testing.T) string {
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

// panelTestRows runs panel and returns the decoded rows. UseNumber keeps a
// decoded numeric cell distinguishable from one emitted as text.
func panelTestRows(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v error = %v\nstdout:\n%s\nstderr:\n%s", args, err, out.String(), errBuf.String())
	}
	dec := json.NewDecoder(strings.NewReader(out.String()))
	dec.UseNumber()
	var view struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := dec.Decode(&view); err != nil {
		t.Fatalf("panel output is not JSON: %v\n%s", err, out.String())
	}
	if len(view.Rows) != 1 {
		t.Fatalf("panel returned %d row(s), want 1:\n%s", len(view.Rows), out.String())
	}
	return view.Rows
}

// TestNovelPanelDecodesAccountingNegatives guards the same measured quirk the
// dump guards (endpoint contract, section I): MUFAP writes "(4.97)" for -4.97
// and never uses a minus sign, so a panel row that carries the cell as text
// reads as a POSITIVE 4.97 to anything that strips the punctuation -- which is
// how the equity market's reported sign was once inverted. Cells decode by
// default; --raw-values returns MUFAP's own text.
func TestNovelPanelDecodesAccountingNegatives(t *testing.T) {
	dbPath := panelTestAccountingMirror(t)
	args := []string{"panel", "--db", dbPath, "--from", "2026-09-04", "--to", "2026-09-04", "--json"}

	row := panelTestRows(t, args...)[0]
	ytd, ok := row["YTD"].(json.Number)
	if !ok {
		t.Fatalf("YTD = %#v (%T), want a JSON number; MUFAP's \"(4.97)\" must not survive as text", row["YTD"], row["YTD"])
	}
	if got, err := ytd.Float64(); err != nil || got != -4.97 {
		t.Fatalf("YTD = %v (err %v), want -4.97: the accounting negative lost its sign", ytd, err)
	}
	if nav, isNum := row["NAV"].(json.Number); !isNum || nav.String() != "203.9071" {
		t.Errorf("NAV = %#v, want the JSON number 203.9071", row["NAV"])
	}
	// Non-numeric cells and the promoted identity/label columns stay text.
	for key, want := range map[string]string{
		"Benchmark":     "N/A",
		"Validity Date": "Sep 04, 2026",
		"date":          "2026-09-04",
		"fund":          "Alfalah GHP Dedicated Equity Fund",
		"sector":        "Dedicated Equity Funds",
	} {
		if got, isStr := row[key].(string); !isStr || got != want {
			t.Errorf("%s = %#v, want the string %q", key, row[key], want)
		}
	}

	rawRow := panelTestRows(t, append(append([]string{}, args...), "--raw-values")...)[0]
	if got, isStr := rawRow["YTD"].(string); !isStr || got != "(4.97)" {
		t.Errorf("--raw-values YTD = %#v, want the string %q", rawRow["YTD"], "(4.97)")
	}
	if got, isStr := rawRow["NAV"].(string); !isStr || got != "203.9071" {
		t.Errorf("--raw-values NAV = %#v, want the string %q", rawRow["NAV"], "203.9071")
	}
}
