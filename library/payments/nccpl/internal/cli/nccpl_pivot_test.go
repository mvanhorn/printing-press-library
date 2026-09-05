package cli

// Hand-authored tests for `panel --pivot`.
//
// Named nccpl_pivot_test.go rather than panel_pivot_test.go for the same reason the
// other hand-written tests here carry the nccpl_ prefix: internal/cli/<command>_<sub>.go
// is the generator's file convention for a SUBCOMMAND of <command>, so panel_*.go reads
// as a `panel` subcommand that does not exist.
//
// Every test drives the real `panel` command through the real root against a real
// SQLite store written by the real store writer, so nothing here can pass against a
// mock that has drifted from what sync and ingest actually persist.
//
// What is being pinned: --pivot folds rows by SUMMING a metric into a
// (sector x investor class) cell. Three inputs make that sum report a figure NCCPL
// never published -- no --metrics (every numeric field added together), a resource
// whose row key is not (sector, investor class), and more than one settlement date
// (a month of daily flows collapsed into one cell). Each must be refused with exit
// code 2 and a message naming the problem, because a fabricated cell is
// indistinguishable from a real one once it leaves this program.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// pivotFixtureDB builds a store holding two settlement dates of fipi-sector rows plus
// one date of fipi rows.
//
// The shape is deliberate:
//
//   - fipi-sector carries TWO dates whose NET_VALUE differs, so a date collapse is
//     visible as a wrong number rather than a plausible one (-6.5 vs the -8.5 sum).
//   - every row also carries BUY_VALUE, so a missing --metrics is visible the same way
//     (-6.5 vs the 93.5 that NET_VALUE + BUY_VALUE would produce).
//   - fipi rows are keyed (client_type, segment) -- investor class x market SEGMENT --
//     which is the shape that pivots into a plausible-looking meaningless table.
func pivotFixtureDB(t *testing.T) string {
	t.Helper()
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	saveExportFixture(t, dbPath, "fipi-sector", "2026-09-03", []store.NCCPLRow{
		{Key: "0801|COMMERCIAL BANKS|FIPI", Payload: `{"SEC_CODE":"0801","SECTOR_NAME":"COMMERCIAL BANKS","CLIENT_TYPE":"FIPI","NET_VALUE":-6.5,"BUY_VALUE":100}`},
		{Key: "0801|COMMERCIAL BANKS|INDIVIDUALS", Payload: `{"SEC_CODE":"0801","SECTOR_NAME":"COMMERCIAL BANKS","CLIENT_TYPE":"INDIVIDUALS","NET_VALUE":6.5,"BUY_VALUE":40}`},
		{Key: "0802|CEMENT|FIPI", Payload: `{"SEC_CODE":"0802","SECTOR_NAME":"CEMENT","CLIENT_TYPE":"FIPI","NET_VALUE":-1.25,"BUY_VALUE":10}`},
		{Key: "0802|CEMENT|INDIVIDUALS", Payload: `{"SEC_CODE":"0802","SECTOR_NAME":"CEMENT","CLIENT_TYPE":"INDIVIDUALS","NET_VALUE":1.25,"BUY_VALUE":5}`},
	})
	saveExportFixture(t, dbPath, "fipi-sector", "2026-09-04", []store.NCCPLRow{
		{Key: "0801|COMMERCIAL BANKS|FIPI", Payload: `{"SEC_CODE":"0801","SECTOR_NAME":"COMMERCIAL BANKS","CLIENT_TYPE":"FIPI","NET_VALUE":-2,"BUY_VALUE":80}`},
		{Key: "0801|COMMERCIAL BANKS|INDIVIDUALS", Payload: `{"SEC_CODE":"0801","SECTOR_NAME":"COMMERCIAL BANKS","CLIENT_TYPE":"INDIVIDUALS","NET_VALUE":2,"BUY_VALUE":30}`},
	})
	saveExportFixture(t, dbPath, "fipi", "2026-09-03", []store.NCCPLRow{
		{Key: "FOREIGN CORPORATES|EQUITY", Payload: `{"client_type":"FOREIGN CORPORATES","segment":"EQUITY","net_value":-1234567.89}`},
		{Key: "FOREIGN CORPORATES|FUTURE", Payload: `{"client_type":"FOREIGN CORPORATES","segment":"FUTURE","net_value":98765.43}`},
	})
	return dbPath
}

// runPivot executes `panel` and returns stdout plus the error, without failing.
func runPivot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"panel", "--no-learn"}, args...)
	stdout, _, err := runRootArgs(t, full...)
	return stdout, err
}

// requirePivotRefused asserts the invocation exited with the usage code, printed no
// matrix, and explained itself with every phrase the caller names.
func requirePivotRefused(t *testing.T, stdout string, err error, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("invocation was accepted; it must be refused (stdout=%q)", stdout)
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("exit code = %d, want 2 (the usage code every other command uses)", code)
	}
	if strings.Contains(stdout, "SECTOR") {
		t.Errorf("a matrix was printed before the refusal; stdout=%q", stdout)
	}
	msg := strings.ToLower(err.Error())
	for _, w := range want {
		if !strings.Contains(msg, strings.ToLower(w)) {
			t.Errorf("message does not name %q: %s", w, err.Error())
		}
	}
}

// TestPanelPivot_RefusesWithoutMetric pins bug (1): with no --metrics, nccplPivotPanel
// matches every metric, so NET_VALUE and BUY_VALUE are added together in one cell.
func TestPanelPivot_RefusesWithoutMetric(t *testing.T) {
	dbPath := pivotFixtureDB(t)

	stdout, err := runPivot(t, "--resource", "fipi-sector",
		"--from", "2026-09-03", "--to", "2026-09-03",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "--metrics", "summed")

	// The pre-existing multi-field guard must survive the rewrite.
	stdout, err = runPivot(t, "--resource", "fipi-sector",
		"--metrics", "NET_VALUE,BUY_VALUE",
		"--from", "2026-09-03", "--to", "2026-09-03",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "single field")
}

// TestPanelPivot_RefusesNonSectorResource pins bug (2): fipi rows are keyed
// (client_type, segment), so pivoting them prints market segments under
// investor-class headings -- a table that looks right and means nothing.
func TestPanelPivot_RefusesNonSectorResource(t *testing.T) {
	dbPath := pivotFixtureDB(t)

	stdout, err := runPivot(t, "--resource", "fipi", "--metrics", "net_value",
		"--from", "2026-09-03", "--to", "2026-09-03",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "sector", "investor class", "fipi-sector")

	// A per-symbol resource has no investor-class dimension at all and must be
	// refused too, rather than rendering an empty matrix with a footnote.
	stdout, err = runPivot(t, "--resource", "var-margins", "--metrics", "free_float",
		"--from", "2026-09-03", "--to", "2026-09-03",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "var-margins", "fipi-sector")
}

// TestPanelPivot_RefusesMultiDateRange pins bug (3) in its explicit form: an
// asked-for range has no date axis in the matrix, so every session would be summed
// into each cell.
func TestPanelPivot_RefusesMultiDateRange(t *testing.T) {
	dbPath := pivotFixtureDB(t)

	stdout, err := runPivot(t, "--resource", "fipi-sector", "--metrics", "NET_VALUE",
		"--from", "2026-09-03", "--to", "2026-09-04",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "one settlement date", "--from", "--to")

	// A half-open range is the same hazard: --to alone still spans the mirror.
	stdout, err = runPivot(t, "--resource", "fipi-sector", "--metrics", "NET_VALUE",
		"--to", "2026-09-04",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "one settlement date")
}

// TestPanelPivot_RefusesMultiDateStore pins bug (3) in the form --from/--to cannot
// see: NO range was given, so the panel spans everything stored. This is the silent
// case -- a month of flows folded into one number with nothing in the output saying so.
func TestPanelPivot_RefusesMultiDateStore(t *testing.T) {
	dbPath := pivotFixtureDB(t)

	stdout, err := runPivot(t, "--resource", "fipi-sector", "--metrics", "NET_VALUE",
		"--pivot", "--db", dbPath, "--json")
	requirePivotRefused(t, stdout, err, "one settlement date", "2 dates", "2026-09-03..2026-09-04")

	// The sum that must never be printed: -6.5 on 09-03 plus -2 on 09-04.
	if strings.Contains(stdout, "-8.5") {
		t.Errorf("the across-date sum reached stdout: %q", stdout)
	}
}

// TestPanelPivot_RendersOneSectorDate is the happy path: a sector-wise resource, one
// metric, one date. The cells must carry that date's published figures exactly -- not
// the two-date sum, and not NET_VALUE blended with BUY_VALUE.
func TestPanelPivot_RendersOneSectorDate(t *testing.T) {
	dbPath := pivotFixtureDB(t)

	stdout, err := runPivot(t, "--resource", "fipi-sector", "--metrics", "NET_VALUE",
		"--from", "2026-09-03", "--to", "2026-09-03",
		"--pivot", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("a single-date sector pivot must be accepted: %v", err)
	}
	var pv nccplPivotView
	if err := json.Unmarshal([]byte(stdout), &pv); err != nil {
		t.Fatalf("--json output must be one JSON document: %v (got=%q)", err, stdout)
	}
	if pv.Metric != "NET_VALUE" {
		t.Errorf("metric = %q, want NET_VALUE", pv.Metric)
	}
	if pv.RowsUsed != 4 {
		t.Errorf("rows_used = %d, want 4 (the BUY_VALUE rows and 2026-09-04 must be excluded)", pv.RowsUsed)
	}
	if len(pv.Investors) != 2 || pv.Investors[0] != "FIPI" || pv.Investors[1] != "INDIVIDUALS" {
		t.Fatalf("investor_classes = %v, want [FIPI INDIVIDUALS]", pv.Investors)
	}
	if len(pv.Sectors) != 2 {
		t.Fatalf("sectors = %d, want 2", len(pv.Sectors))
	}
	banks := pv.Sectors[0]
	if banks.Sector != "0801|COMMERCIAL BANKS" {
		t.Errorf("first sector = %q, want 0801|COMMERCIAL BANKS", banks.Sector)
	}
	if banks.By["FIPI"] != -6.5 {
		t.Errorf("banks FIPI = %v, want -6.5 (the 2026-09-03 figure; -8.5 would mean the two dates were summed)", banks.By["FIPI"])
	}
	if banks.By["INDIVIDUALS"] != 6.5 {
		t.Errorf("banks INDIVIDUALS = %v, want 6.5", banks.By["INDIVIDUALS"])
	}
	if banks.Total != 0 {
		t.Errorf("banks total = %v, want 0 (a sector nets to zero across investor classes)", banks.Total)
	}
	cement := pv.Sectors[1]
	if cement.By["FIPI"] != -1.25 || cement.By["INDIVIDUALS"] != 1.25 {
		t.Errorf("cement cells = %v, want FIPI -1.25 / INDIVIDUALS 1.25", cement.By)
	}

	// The rendered table a human reads must carry the same figures.
	table, err := runPivot(t, "--resource", "fipi-sector", "--metrics", "NET_VALUE",
		"--from", "2026-09-03", "--to", "2026-09-03",
		"--pivot", "--db", dbPath, "--human-friendly")
	if err != nil {
		t.Fatalf("table render: %v", err)
	}
	for _, want := range []string{"SECTOR", "FIPI", "INDIVIDUALS", "-6.50", "6.50", "-1.25"} {
		if !strings.Contains(table, want) {
			t.Errorf("table is missing %q:\n%s", want, table)
		}
	}
	if strings.Contains(table, "-8.50") || strings.Contains(table, "93.50") {
		t.Errorf("table carries a summed figure the source never published:\n%s", table)
	}
}

// TestNCCPLPivotableResource pins the admissible set to the registry's own key shapes
// rather than to a hand-kept list of names, so a resource whose KeyParts change stops
// (or starts) qualifying automatically.
func TestNCCPLPivotableResource(t *testing.T) {
	want := map[string]bool{
		"fipi-sector": true,                 // SEC_CODE|SECTOR_NAME|CLIENT_TYPE
		"lipi-sector": true,                 // same
		"flows":       true,                 // FLSectorName|FLTypeNew -- scstrade's republication of the same shape
		"fipi":        false, "lipi": false, // client_type|segment -- investor class x market segment
		"fipi-normal": false, "lipi-normal": false, // CLIENT_TYPE|MARKET_TYPE -- ditto
		"var-margins": false, "mts": false, "slb": false, // one key part, no investor dimension
	}
	for name, ok := range want {
		res, found := nccplResourceByName(name)
		if !found {
			t.Fatalf("registry has no resource %q", name)
		}
		if got := nccplPivotableResource(res); got != ok {
			t.Errorf("pivotable(%s) = %v, want %v (KeyParts=%v)", name, got, ok, res.KeyParts)
		}
	}
	names := nccplPivotableResourceNames()
	if len(names) == 0 {
		t.Fatal("no resource qualifies for --pivot; the rejection message would name nothing")
	}
	if strings.Join(names, ",") != "fipi-sector,lipi-sector,flows" {
		t.Errorf("pivotable names = %v, want [fipi-sector lipi-sector flows]", names)
	}
}
