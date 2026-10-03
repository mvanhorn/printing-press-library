package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/store"
	"path/filepath"
	"testing"
)

func TestPlanningRejectsCommittedOpenWAL(t *testing.T) {
	home := testenv.Isolate(t)
	path := filepath.Join(home, "wal-profile", "data.db")
	ctx := platform.ContextWithSession(context.Background(), &platform.Session{GateOutcome: platform.GateVerified, Paths: platform.Paths{DataFile: path}})
	db, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("planning_snapshot", "base", json.RawMessage(`{"name":"checkpointedbaseline"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.DB().Exec("PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Upsert("planning_snapshot", "new", json.RawMessage(`{"name":"committedwalneedle"}`)); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := newNovelHostelsSavedCmd(&rootFlags{asJSON: true})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("immutable reader returned success while committed WAL writer is open: %s", out.String())
	}
	if out.Len() != 0 {
		t.Fatalf("unsafe read emitted partial/success output: %s", out.String())
	}
}
