package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govcache"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestGovLocalLifecycle(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "evidence.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s := govdata.Snapshot{ID: "week1", Query: "water", Source: "both", CapturedAt: "2026-11-01T12:00:00Z", Scope: map[string]any{"limit": 2}, Datasets: []govdata.Dataset{{Source: "california", ID: "ca", Title: "Water", Resources: []govdata.Resource{{ID: "r", URL: "https://agency.gov/water.csv", Format: "CSV"}}}, {Source: "federal", ID: "us", Title: "Water", Resources: []govdata.Resource{{ID: "0", URL: "https://agency.gov/water.csv", Format: "CSV"}}}}}
	if err = govcache.Save(context.Background(), db.DB(), s); err != nil {
		t.Fatal(err)
	}
	s.ID = "week2"
	s.Datasets = s.Datasets[:1]
	s.Datasets[0].Title = "Water updated"
	if err = govcache.Save(context.Background(), db.DB(), s); err != nil {
		t.Fatal(err)
	}
	db.Close()
	run := func(kind string, args ...string) map[string]any {
		t.Helper()
		flags := &rootFlags{asJSON: true}
		cmd := newGovLocalCmd(flags, kind)
		out := new(bytes.Buffer)
		cmd.SetOut(out)
		stderr := new(bytes.Buffer)
		cmd.SetErr(stderr)
		cmd.SetArgs(append(args, "--db", dbPath))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(stderr.Bytes(), []byte("not been synced")) {
			t.Fatalf("saved snapshot incorrectly called unsynced: %s", stderr.String())
		}
		var v map[string]any
		if err := json.Unmarshal(out.Bytes(), &v); err != nil {
			t.Fatalf("invalid output %q: %v", out.String(), err)
		}
		return v
	}
	if r := run("overlap", "--snapshot", "week1"); r["total"] != float64(1) {
		t.Fatalf("overlap: %+v", r)
	}
	if r := run("changes", "--from", "week1", "--to", "week2"); r["total_changes"] != float64(2) {
		t.Fatalf("diff: %+v", r)
	}
	run("coverage", "--snapshot", "week1")
	run("audit", "--snapshot", "week1")
	dest := filepath.Join(dir, "intake.json")
	run("lock", "--snapshot", "week1", "--resource", "california:ca:r", "--output", dest)
	raw, err := os.ReadFile(dest)
	if err != nil || !bytes.Contains(raw, []byte("metadata_sha256")) {
		t.Fatalf("manifest %q %v", raw, err)
	}
	flags := &rootFlags{asJSON: true}
	cmd := newGovLocalCmd(flags, "lock")
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--snapshot", "week1", "--resource", "california:ca:r", "--output", dest, "--db", dbPath})
	if err := cmd.Execute(); err == nil {
		t.Fatal("lock overwrote existing artifact")
	}
}
func TestGovLocalDryRunDoesNoIO(t *testing.T) {
	for _, kind := range []string{"overlap", "changes", "coverage", "audit", "lock"} {
		flags := &rootFlags{asJSON: true, dryRun: true}
		cmd := newGovLocalCmd(flags, kind)
		out := new(bytes.Buffer)
		cmd.SetOut(out)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--db", "/nonexistent/must-not-be-opened/evidence.db"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s dry run failed: %v", kind, err)
		}
		if !json.Valid(out.Bytes()) {
			t.Fatalf("%s dry run invalid JSON %q", kind, out.String())
		}
	}
}
