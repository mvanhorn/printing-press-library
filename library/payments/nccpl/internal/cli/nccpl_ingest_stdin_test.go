package cli

// Hand-authored test for `ingest --stdin`.
//
// Named nccpl_ingest_stdin_test.go rather than ingest_stdin_test.go for the same
// reason as the other hand-written tests here: internal/cli/<command>_<sub>.go is
// the generator's convention for a SUBCOMMAND file.

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

const stdinVarMarginsBody = `{"success":true,"margins":[` +
	`{"symbol":"HUBC","var_value":"15.5","hair_cut":"20.0","26week_avg":"11.3616","acc_qty%":"0.0"},` +
	`{"symbol":"OGDC","var_value":"12.25","hair_cut":"17.5","26week_avg":"9.8","acc_qty%":"0.0"}]}`

// runRootArgsStdin is runRootArgs with a stdin payload attached. Kept here rather
// than in the generated test harness so a regeneration cannot drop it.
func runRootArgsStdin(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	rootCmd := RootCmd()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return stdout.String(), stderr.String(), err
}

// TestIngestStdin_FilesABodyFromStdin pins the flag's whole point: a captured
// response body can be piped straight in, with no temp file, and lands in the
// store on the same path a file ingest uses.
func TestIngestStdin_FilesABodyFromStdin(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	stdout, _, err := runRootArgsStdin(t, stdinVarMarginsBody,
		"ingest", "--no-learn", "--stdin",
		"--resource", "var-margins", "--date", "2026-09-04",
		"--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("ingest --stdin: %v", err)
	}
	var view nccplIngestView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("--json output must be one JSON document: %v (got=%q)", err, stdout)
	}
	if view.TotalRows != 2 {
		t.Fatalf("total_rows = %d, want 2", view.TotalRows)
	}
	if len(view.Ingested) != 1 {
		t.Fatalf("ingested = %+v, want one batch", view.Ingested)
	}
	got := view.Ingested[0]
	if got.Resource != "var-margins" || got.Date != "2026-09-04" {
		t.Errorf("batch = %+v, want var-margins/2026-09-04", got)
	}
	if got.Source != "stdin" {
		t.Errorf("source = %q, want \"stdin\" so the provenance is not mistaken for a file", got.Source)
	}

	// The rows must be readable back through the ordinary store path.
	ctx := context.Background()
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	obs, err := store.NCCPLObservations(ctx, db, "var-margins", "", "")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("store holds %d row(s), want 2", len(obs))
	}
	for _, o := range obs {
		if o.Date != "2026-09-04" {
			t.Errorf("row dated %s; --date must be honoured exactly, never inferred", o.Date)
		}
	}
}

// TestIngestStdin_Refusals pins the two ways --stdin can be misused. Both must
// fail loudly rather than silently store nothing, because a silent no-op here
// would read as "the capture held no data".
func TestIngestStdin_Refusals(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	// Empty stdin is a usage error, not an empty success.
	if _, _, err := runRootArgsStdin(t, "   \n",
		"ingest", "--no-learn", "--stdin",
		"--resource", "var-margins", "--date", "2026-09-04", "--db", dbPath); err == nil {
		t.Error("empty stdin must be rejected rather than reported as nothing ingested")
	}

	// A raw body with no --resource/--date cannot be filed; it must be reported
	// as skipped with a reason, never guessed at.
	stdout, _, err := runRootArgsStdin(t, stdinVarMarginsBody,
		"ingest", "--no-learn", "--stdin", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("a raw body without --resource/--date should be a clean skip, got: %v", err)
	}
	var view nccplIngestView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("output must be JSON: %v (got=%q)", err, stdout)
	}
	if view.TotalRows != 0 {
		t.Errorf("total_rows = %d; a body that cannot be filed must store nothing", view.TotalRows)
	}
	if len(view.Skipped) != 1 || !strings.Contains(strings.ToLower(view.Skipped[0]), "resource") {
		t.Errorf("skipped = %+v, want one entry naming the missing --resource/--date", view.Skipped)
	}
}
