package cli

// Hand-authored tests for the `export` command.
//
// Every test drives the real command through the real root, against a real SQLite
// store written by the real store writer, so nothing here can pass on a mock that
// has drifted from what sync and ingest actually persist.
//
// Named nccpl_export_test.go, not export_test.go, for the same reason the other
// hand-written tests here carry the nccpl_ prefix: internal/cli/<command>_<sub>.go
// is the generator's file convention for a SUBCOMMAND of <command> (fipi.go plus
// fipi_data.go), so a file called export_test.go reads as an "export test"
// subcommand that does not exist.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// saveExportFixture writes one (resource, date) through the same store call sync
// and ingest use, so the tests read a store built by the production write path.
func saveExportFixture(t *testing.T, dbPath, resource, date string, rows []store.NCCPLRow) {
	t.Helper()
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := store.EnsureNCCPLSchema(ctx, s); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.SaveNCCPLDate(ctx, s, resource, date, rows, time.Now()); err != nil {
		t.Fatalf("save %s/%s: %v", resource, date, err)
	}
}

// exportFixtureDB builds the store every export test reads.
//
// The shape is deliberate: fipi holds two consecutive sessions, var-margins holds
// 2026-09-01 and 2026-09-03 and NEVER 2026-09-02. That hole is the fixture for the
// owner rule -- a session that was never fetched must stay absent from the output.
// The two var-margins vintages also disagree on HUBC's margin, so a forward-fill
// would be visible rather than silently plausible.
func exportFixtureDB(t *testing.T) string {
	t.Helper()
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	saveExportFixture(t, dbPath, "fipi", "2026-09-01", []store.NCCPLRow{
		{Key: "FOREIGN CORPORATES|EQUITY", Payload: `{"client_type":"FOREIGN CORPORATES","segment":"EQUITY","net_value":-1234567.89}`},
		{Key: "OVERSEAS PAKISTANIES|EQUITY", Payload: `{"client_type":"OVERSEAS PAKISTANIES","segment":"EQUITY","net_value":98765.43}`},
	})
	saveExportFixture(t, dbPath, "fipi", "2026-09-02", []store.NCCPLRow{
		{Key: "FOREIGN CORPORATES|EQUITY", Payload: `{"client_type":"FOREIGN CORPORATES","segment":"EQUITY","net_value":455000.5}`},
		{Key: "OVERSEAS PAKISTANIES|EQUITY", Payload: `{"client_type":"OVERSEAS PAKISTANIES","segment":"EQUITY","net_value":-12000}`},
	})
	saveExportFixture(t, dbPath, "var-margins", "2026-09-01", []store.NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","var_margin":15.5,"free_float":38.4}`},
		{Key: "OGDC", Payload: `{"symbol":"OGDC","var_margin":12.25,"free_float":14.9}`},
	})
	// 2026-09-02 is skipped on purpose -- it was never fetched.
	saveExportFixture(t, dbPath, "var-margins", "2026-09-03", []store.NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","var_margin":16.75,"free_float":38.4}`},
		{Key: "OGDC", Payload: `{"symbol":"OGDC","var_margin":12.25,"free_float":15.1}`},
	})
	return dbPath
}

// runExport executes `export` with the supplied args and fails on a non-zero exit.
// Returns (dump-or-stdout, summary-or-stderr).
func runExport(t *testing.T, args ...string) (string, string) {
	t.Helper()
	full := append([]string{"export", "--no-learn"}, args...)
	stdout, stderr, err := runRootArgs(t, full...)
	if err != nil {
		t.Fatalf("export %v: %v (stderr=%q)", args, err, stderr)
	}
	return stdout, stderr
}

// decodeExportJSONL parses a JSONL dump into records, failing on any line that is
// not a standalone JSON object.
func decodeExportJSONL(t *testing.T, dump string) []nccplExportRecord {
	t.Helper()
	out := make([]nccplExportRecord, 0)
	for i, line := range strings.Split(strings.TrimRight(dump, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec nccplExportRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d is not a JSON object: %v (line=%q)", i+1, err, line)
		}
		out = append(out, rec)
	}
	return out
}

// decodeExportSummary parses the run summary envelope.
//
// When the dump owns stdout the summary shares stderr with the "no local mirror"
// hint, so leading plain-text diagnostics are skipped before decoding.
func decodeExportSummary(t *testing.T, s string) nccplExportView {
	t.Helper()
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	var view nccplExportView
	if err := json.Unmarshal([]byte(s), &view); err != nil {
		t.Fatalf("summary is not JSON: %v (got=%q)", err, s)
	}
	return view
}

// TestExport_JSONLShape pins the default dump: one JSON object per line carrying the
// store's own coordinates plus the payload nested (not flattened, not stringified),
// with the summary kept off stdout so the dump can be piped straight into a reader.
func TestExport_JSONLShape(t *testing.T) {
	dbPath := exportFixtureDB(t)

	dump, _ := runExport(t, "--db", dbPath)
	recs := decodeExportJSONL(t, dump)
	if len(recs) != 8 {
		t.Fatalf("got %d records, want 8", len(recs))
	}
	for _, rec := range recs {
		if rec.Resource == "" || rec.Date == "" || rec.Key == "" {
			t.Errorf("record missing coordinates: %+v", rec)
		}
		if rec.ObservedAt == "" {
			t.Errorf("record missing observed_at vintage: %+v", rec)
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			t.Errorf("payload must be a nested JSON object, got %s: %v", rec.Payload, err)
		}
	}
	// The payload must be the stored object byte-for-byte, not a reshaped copy.
	var first map[string]any
	if err := json.Unmarshal(recs[0].Payload, &first); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if first["client_type"] != "FOREIGN CORPORATES" {
		t.Errorf("payload lost its upstream field names: %s", recs[0].Payload)
	}

	// With --json the caller gets one document on stdout instead: the summary
	// with the records inlined. Streaming JSONL and a JSON summary cannot share
	// stdout, so --json picks the document form -- see TestExport_JSONDocument.
	jsonOut, _ := runExport(t, "--db", dbPath, "--json")
	view := decodeExportSummary(t, jsonOut)
	if view.TotalRows != 8 {
		t.Errorf("summary total_rows = %d, want 8", view.TotalRows)
	}
	if view.Format != "jsonl" || view.Output != "stdout" {
		t.Errorf("summary format/output = %q/%q, want jsonl/stdout", view.Format, view.Output)
	}
	if len(view.Resources) != 2 {
		t.Errorf("summary listed %d resource(s), want 2", len(view.Resources))
	}
}

// TestExport_CSVShape pins the CSV dump: a header row in a fixed column order, one
// row per observation, and the payload preserved as a parseable JSON string rather
// than exploded into per-resource columns that would differ per resource.
func TestExport_CSVShape(t *testing.T) {
	dbPath := exportFixtureDB(t)

	dump, _ := runExport(t, "--db", dbPath, "--format", "csv")
	rows, err := csv.NewReader(strings.NewReader(dump)).ReadAll()
	if err != nil {
		t.Fatalf("dump is not valid CSV: %v", err)
	}
	if len(rows) != 9 {
		t.Fatalf("got %d CSV rows, want 9 (header + 8 observations)", len(rows))
	}
	if strings.Join(rows[0], ",") != strings.Join(nccplExportCSVHeader, ",") {
		t.Errorf("header = %v, want %v", rows[0], nccplExportCSVHeader)
	}
	for i, row := range rows[1:] {
		if len(row) != len(nccplExportCSVHeader) {
			t.Fatalf("row %d has %d columns, want %d", i+1, len(row), len(nccplExportCSVHeader))
		}
		if !json.Valid([]byte(row[4])) {
			t.Errorf("row %d payload column is not JSON: %q", i+1, row[4])
		}
	}
}

// TestExport_ResourcesFilter pins --resources: a named subset is the only thing
// dumped, and an unknown name fails loudly instead of quietly exporting nothing.
func TestExport_ResourcesFilter(t *testing.T) {
	dbPath := exportFixtureDB(t)

	dump, _ := runExport(t, "--db", dbPath, "--resources", "var-margins")
	recs := decodeExportJSONL(t, dump)
	if len(recs) != 4 {
		t.Fatalf("got %d records for var-margins, want 4", len(recs))
	}
	for _, rec := range recs {
		if rec.Resource != "var-margins" {
			t.Errorf("filter leaked resource %q", rec.Resource)
		}
	}
	jsonOut, _ := runExport(t, "--db", dbPath, "--resources", "var-margins", "--json")
	view := decodeExportSummary(t, jsonOut)
	if len(view.Resources) != 1 || view.Resources[0].Resource != "var-margins" {
		t.Errorf("summary resources = %+v, want only var-margins", view.Resources)
	}

	// A typo must be an error, not an empty export that reads like "no data".
	if _, _, err := runRootArgs(t, "export", "--no-learn", "--db", dbPath, "--resources", "var-margns"); err == nil {
		t.Error("an unknown --resources name must fail rather than export nothing")
	}
}

// TestExport_DateBounds pins --from/--to, including the inverted-window rejection
// that stops a typo from looking like an empty archive.
func TestExport_DateBounds(t *testing.T) {
	dbPath := exportFixtureDB(t)

	dump, _ := runExport(t, "--db", dbPath, "--resources", "fipi",
		"--from", "2026-09-02", "--to", "2026-09-02")
	recs := decodeExportJSONL(t, dump)
	if len(recs) != 2 {
		t.Fatalf("got %d records inside the one-day window, want 2", len(recs))
	}
	for _, rec := range recs {
		if rec.Date != "2026-09-02" {
			t.Errorf("date bound leaked %s", rec.Date)
		}
	}

	// A window that lands entirely before the archive returns nothing at all.
	dump, _ = runExport(t, "--db", dbPath, "--from", "2020-01-01", "--to", "2020-01-31")
	if got := decodeExportJSONL(t, dump); len(got) != 0 {
		t.Errorf("window before the archive returned %d record(s), want 0", len(got))
	}

	if _, _, err := runRootArgs(t, "export", "--no-learn", "--db", dbPath,
		"--from", "2026-09-05", "--to", "2026-09-01"); err == nil {
		t.Error("an inverted --from/--to window must be rejected")
	}
	if _, _, err := runRootArgs(t, "export", "--no-learn", "--db", dbPath, "--from", "yesterday"); err == nil {
		t.Error("a malformed --from must be rejected")
	}
}

// TestExport_GapsStayGaps is the owner rule, pinned.
//
// var-margins holds 2026-09-01 and 2026-09-03 and was never fetched for 2026-09-02.
// Export must emit exactly what is stored: no interpolated date, no carried-forward
// value, no synthesised row. Asking for the hole alone must return nothing, and the
// two real vintages must keep their own differing values.
func TestExport_GapsStayGaps(t *testing.T) {
	dbPath := exportFixtureDB(t)

	dump, _ := runExport(t, "--db", dbPath, "--resources", "var-margins",
		"--from", "2026-09-01", "--to", "2026-09-03")
	recs := decodeExportJSONL(t, dump)
	if len(recs) != 4 {
		t.Fatalf("got %d records across the span, want exactly the 4 stored rows", len(recs))
	}

	byDate := map[string]int{}
	margins := map[string]float64{}
	for _, rec := range recs {
		byDate[rec.Date]++
		var payload struct {
			Symbol string  `json:"symbol"`
			Margin float64 `json:"var_margin"`
		}
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			t.Fatalf("payload: %v", err)
		}
		if payload.Symbol == "HUBC" {
			margins[rec.Date] = payload.Margin
		}
	}
	if _, present := byDate["2026-09-02"]; present {
		t.Fatalf("2026-09-02 was never fetched and must be absent, got %d row(s) for it", byDate["2026-09-02"])
	}
	if byDate["2026-09-01"] != 2 || byDate["2026-09-03"] != 2 {
		t.Errorf("stored dates = %v, want 2 rows on each of 2026-09-01 and 2026-09-03", byDate)
	}
	if margins["2026-09-01"] != 15.5 || margins["2026-09-03"] != 16.75 {
		t.Errorf("HUBC var_margin = %v; each date must keep its own stored value, never a carried-forward one", margins)
	}

	jsonOut, _ := runExport(t, "--db", dbPath, "--resources", "var-margins",
		"--from", "2026-09-01", "--to", "2026-09-03", "--json")
	view := decodeExportSummary(t, jsonOut)
	if view.TotalRows != 4 {
		t.Errorf("summary total_rows = %d, want 4", view.TotalRows)
	}
	if len(view.Resources) != 1 || view.Resources[0].Dates != 2 {
		t.Errorf("summary must report the 2 stored dates, not the 3-day span: %+v", view.Resources)
	}

	// Asking for the hole alone is an empty export, not a fabricated one.
	dump, _ = runExport(t, "--db", dbPath, "--resources", "var-margins",
		"--from", "2026-09-02", "--to", "2026-09-02")
	if got := decodeExportJSONL(t, dump); len(got) != 0 {
		t.Fatalf("the missing session produced %d record(s); a gap must stay a gap: %+v", len(got), got)
	}
	jsonOut, _ = runExport(t, "--db", dbPath, "--resources", "var-margins",
		"--from", "2026-09-02", "--to", "2026-09-02", "--json")
	if view := decodeExportSummary(t, jsonOut); view.TotalRows != 0 {
		t.Errorf("summary total_rows = %d for a never-fetched session, want 0", view.TotalRows)
	}
}

// TestExport_EmptyStore pins both empty cases: no store file at all, and a real
// store that holds nothing matching the filter. Neither is an error, and neither
// writes a byte of dump.
func TestExport_EmptyStore(t *testing.T) {
	home := withTempLearnHome(t)
	missing := filepath.Join(home, "not-created-yet.db")

	dump, _ := runExport(t, "--db", missing)
	if strings.TrimSpace(dump) != "" {
		t.Errorf("a missing store must dump nothing, got %q", dump)
	}
	jsonOut, _ := runExport(t, "--db", missing, "--json")
	view := decodeExportSummary(t, jsonOut)
	if view.TotalRows != 0 || len(view.Resources) != 0 {
		t.Errorf("summary = %+v, want an empty export", view)
	}
	if view.Note == "" {
		t.Error("an empty export must say why it is empty")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("export must never create the store it reads")
	}

	// A real store whose only resource is filtered out is equally empty.
	dbPath := exportFixtureDB(t)
	dump, _ = runExport(t, "--db", dbPath, "--resources", "slb")
	if strings.TrimSpace(dump) != "" {
		t.Errorf("a resource with nothing stored must dump nothing, got %q", dump)
	}
	jsonOut, _ = runExport(t, "--db", dbPath, "--resources", "slb", "--json")
	if view := decodeExportSummary(t, jsonOut); view.TotalRows != 0 {
		t.Errorf("summary total_rows = %d, want 0", view.TotalRows)
	}
}

// TestExport_JSONDocument pins the --json contract: exactly ONE JSON document on
// stdout, with the records inlined under "records".
//
// This is the shape the Printing Press's own live dogfood json_fidelity check
// asserts, and getting it wrong is how `export` shipped broken: the summary went
// to stderr while stdout carried either bare JSONL or, on an empty store, nothing
// at all -- so `export --json` was unparseable for the agent surface it exists to
// serve. Streaming JSONL and a JSON summary cannot share one stream, so --json
// picks the document form and --output keeps the streaming form.
func TestExport_JSONDocument(t *testing.T) {
	dbPath := exportFixtureDB(t)

	stdout, _ := runExport(t, "--db", dbPath, "--json")
	var view nccplExportView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("--json stdout must be exactly one JSON document: %v (got=%q)", err, stdout)
	}
	if len(view.Records) != 8 || view.TotalRows != 8 {
		t.Fatalf("inlined records = %d, total_rows = %d, want 8 and 8", len(view.Records), view.TotalRows)
	}
	for _, rec := range view.Records {
		if rec.Resource == "" || rec.Date == "" || rec.Key == "" || rec.ObservedAt == "" {
			t.Errorf("inlined record missing coordinates: %+v", rec)
		}
		if !json.Valid(rec.Payload) {
			t.Errorf("inlined payload is not JSON: %s", rec.Payload)
		}
	}
	if view.Note == "" || !strings.Contains(view.Note, "records") {
		t.Errorf("note must say the records were inlined, got %q", view.Note)
	}

	// The empty case is the one that actually failed the gate: stdout must still
	// be one valid document, not an empty stream.
	home := withTempLearnHome(t)
	emptyOut, _ := runExport(t, "--db", filepath.Join(home, "absent.db"), "--json")
	var emptyView nccplExportView
	if err := json.Unmarshal([]byte(emptyOut), &emptyView); err != nil {
		t.Fatalf("--json on an empty store must still emit one JSON document: %v (got=%q)", err, emptyOut)
	}
	if emptyView.TotalRows != 0 || len(emptyView.Records) != 0 {
		t.Errorf("empty store view = %+v, want zero rows and no records", emptyView)
	}

	// --output keeps the streaming form: the file gets JSONL, stdout gets the
	// summary, and the summary does NOT carry an inlined copy of the records.
	outPath := filepath.Join(t.TempDir(), "dump.jsonl")
	fileOut, _ := runExport(t, "--db", dbPath, "--output", outPath, "--json")
	var fileView nccplExportView
	if err := json.Unmarshal([]byte(fileOut), &fileView); err != nil {
		t.Fatalf("--output summary must be JSON: %v (got=%q)", err, fileOut)
	}
	if len(fileView.Records) != 0 {
		t.Errorf("--output must stream to the file, not inline %d record(s) in the summary", len(fileView.Records))
	}
	if fileView.TotalRows != 8 {
		t.Errorf("--output summary total_rows = %d, want 8", fileView.TotalRows)
	}
}

// TestExport_OutputFileAndDryRun pins --output and --dry-run: the file receives the
// dump and stdout receives the summary, while a dry-run reports the same counts and
// creates no file at all.
func TestExport_OutputFileAndDryRun(t *testing.T) {
	dbPath := exportFixtureDB(t)
	outPath := filepath.Join(t.TempDir(), "store.jsonl")

	summaryOut, _ := runExport(t, "--db", dbPath, "--output", outPath, "--json")
	view := decodeExportSummary(t, summaryOut)
	if view.Output != outPath || view.TotalRows != 8 {
		t.Errorf("summary = %+v, want 8 rows written to %s", view, outPath)
	}
	written, err := os.ReadFile(outPath) // #nosec G304 -- test-owned temp path.
	if err != nil {
		t.Fatalf("reading --output file: %v", err)
	}
	if got := decodeExportJSONL(t, string(written)); len(got) != 8 {
		t.Errorf("--output file holds %d record(s), want 8", len(got))
	}

	dryPath := filepath.Join(t.TempDir(), "never-written.jsonl")
	summaryOut, _ = runExport(t, "--db", dbPath, "--output", dryPath, "--dry-run", "--json")
	view = decodeExportSummary(t, summaryOut)
	if !view.DryRun || view.Action != "export" {
		t.Errorf("dry-run summary = %+v, want dry_run=true action=export", view)
	}
	if view.TotalRows != 8 {
		t.Errorf("dry-run total_rows = %d, want the 8 rows it would have written", view.TotalRows)
	}
	if view.Would == "" {
		t.Error("dry-run must report what it would have written")
	}
	if _, err := os.Stat(dryPath); !os.IsNotExist(err) {
		t.Errorf("dry-run created %s; it must write nothing", dryPath)
	}
}

// TestExport_RejectsBadFormatAndStrayArgs pins the two flag-level refusals that
// would otherwise produce a silently wrong dump.
func TestExport_RejectsBadFormatAndStrayArgs(t *testing.T) {
	dbPath := exportFixtureDB(t)

	if _, _, err := runRootArgs(t, "export", "--no-learn", "--db", dbPath, "--format", "parquet"); err == nil {
		t.Error("an unsupported --format must be rejected, not silently downgraded")
	}
	if _, _, err := runRootArgs(t, "export", "--no-learn", "--db", dbPath, "fipi"); err == nil {
		t.Error("a stray positional argument must be rejected rather than ignored")
	}
}

// TestExportPayloadFallback pins the one defensive path: a stored payload that is
// not valid JSON degrades to a quoted string on its own line instead of making the
// entire export unparseable.
func TestExportPayloadFallback(t *testing.T) {
	if got := string(nccplExportPayload(`{"a":1}`)); got != `{"a":1}` {
		t.Errorf("valid JSON must pass through byte-for-byte, got %s", got)
	}
	got := nccplExportPayload("not json at all")
	if !json.Valid(got) {
		t.Fatalf("fallback produced invalid JSON: %s", got)
	}
	var s string
	if err := json.Unmarshal(got, &s); err != nil || s != "not json at all" {
		t.Errorf("fallback = %s, want the raw text as a JSON string", got)
	}
}
