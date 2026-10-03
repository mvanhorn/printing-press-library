package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/store"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aliasContext(path string) context.Context {
	return platform.ContextWithSession(context.Background(), &platform.Session{GateOutcome: platform.GateVerified, Paths: platform.Paths{DataFile: path}})
}
func executeLocalCache(ctx context.Context, args ...string) (string, error) {
	flags := &rootFlags{asJSON: true}
	root := &cobra.Command{Use: "test", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newSearchCmd(flags), newWorkflowCmd(flags), newNovelHostelsSavedCmd(flags))
	configurePlanningCache(root, flags)
	root.SetContext(ctx)
	root.SetArgs(args)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	return out.String(), err
}
func TestActualCacheReadsAndWritesRejectAliasesWithCommittedOpenWAL(t *testing.T) {
	home := testenv.Isolate(t)
	real := filepath.Join(home, "real", "data.db")
	ctx := aliasContext(real)
	if err := savePlanning(ctx, map[string]any{"name": "baselineword"}); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, "symlink.db")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	writer, err := store.OpenWithContext(ctx, real)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Upsert("planning_snapshot", "new", json.RawMessage(`{"name":"walneedleword"}`)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{real, alias} {
		for _, args := range [][]string{{"saved"}, {"search", "walneedleword"}, {"workflow", "status"}} {
			out, err := executeLocalCache(aliasContext(path), args...)
			if err == nil || !strings.Contains(err.Error(), "cache_visibility_unavailable") || out != "" {
				t.Fatalf("unsafe read succeeded: %s %v %s", args, err, out)
			}
		}
	}
	// SQLite's own locks serialize writes to the same canonical filename.
	if err := savePlanning(aliasContext(real), map[string]any{"name": "canonicalconcurrentword"}); err != nil {
		t.Fatal(err)
	}
	if err := savePlanning(aliasContext(alias), map[string]any{"name": "symlinkconcurrentword"}); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(home, "hard.db")
	if err := os.Link(real, hard); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{real, alias, hard} {
		if err := savePlanning(aliasContext(path), map[string]any{"name": "mustneverbeclaimed"}); err == nil {
			t.Fatal("unsafe alias/open-WAL save reported success")
		}
	}
	if _, err := os.Stat(hard + "-wal"); !os.IsNotExist(err) {
		t.Fatal("denied alias save created its own WAL", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	// Hard-link aliases remain unsafe after writer close.
	if err := savePlanning(aliasContext(hard), map[string]any{"name": "mustneverbeclaimed"}); err == nil {
		t.Fatal("closed hard-link alias save accepted")
	}
	if _, err := executeLocalCache(aliasContext(hard), "saved"); err == nil {
		t.Fatal("closed hard-link alias read accepted")
	}
	if err := os.Remove(hard); err != nil {
		t.Fatal(err)
	}
	out, err := executeLocalCache(aliasContext(alias), "search", "walneedleword")
	if err != nil || !strings.Contains(out, "walneedleword") {
		t.Fatal("post-close read did not recover latest committed observation", err, out)
	}
	if err := savePlanning(aliasContext(alias), map[string]any{"name": "canonicalsaveword"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(alias + "-wal"); !os.IsNotExist(err) {
		t.Fatal("symlink save used an alias WAL", err)
	}
	out, err = executeLocalCache(ctx, "saved")
	if err != nil || !strings.Contains(out, "walneedleword") || !strings.Contains(out, "canonicalsaveword") || !strings.Contains(out, "canonicalconcurrentword") || !strings.Contains(out, "symlinkconcurrentword") || strings.Contains(out, "mustneverbeclaimed") {
		t.Fatal("canonical save lost or misclaimed observations", err, out)
	}
}

func TestPinnedCacheReaderDoesNotFollowRestoredPathReplacement(t *testing.T) {
	home := testenv.Isolate(t)
	selected := filepath.Join(home, "selected.db")
	other := filepath.Join(home, "other.db")
	if err := savePlanning(aliasContext(selected), map[string]any{"name": "selectedprofileword"}); err != nil {
		t.Fatal(err)
	}
	if err := savePlanning(aliasContext(other), map[string]any{"name": "wrongprofileword"}); err != nil {
		t.Fatal(err)
	}
	db, guard, err := OpenPlanningReadOnly(aliasContext(selected), "")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := db.Path()
	defer guard.Close()
	defer db.Close()
	held := filepath.Join(home, "held.db")
	if err := os.Rename(selected, held); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, selected); err != nil {
		t.Fatal(err)
	}
	var data string
	if err := db.DB().QueryRow("SELECT data FROM resources WHERE resource_type='planning_snapshot' LIMIT 1").Scan(&data); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(selected, other); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(held, selected); err != nil {
		t.Fatal(err)
	}
	if err := guard.Check(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "selectedprofileword") || strings.Contains(data, "wrongprofileword") {
		t.Fatal("lazy SQL opened substituted profile", data)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshot); !os.IsNotExist(err) {
		t.Fatal("private snapshot was retained", err)
	}
}

func TestCachePercentAliasesFailBeforeReadsOrSchemaWrites(t *testing.T) {
	home := testenv.Isolate(t)
	selected := filepath.Join(home, "target%41.db")
	wrong := filepath.Join(home, "targetA.db")
	for _, path := range []string{selected, wrong} {
		db, err := store.OpenWithContext(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Upsert("planning_snapshot", "row", json.RawMessage(`{"name":"keepunchangedword"}`)); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	beforeSelected, err := os.ReadFile(selected)
	if err != nil {
		t.Fatal(err)
	}
	beforeWrong, err := os.ReadFile(wrong)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"saved"}, {"search", "keepunchangedword"}, {"workflow", "status"}} {
		out, err := executeLocalCache(aliasContext(selected), args...)
		if err == nil || out != "" || !strings.Contains(err.Error(), "cache_visibility_unavailable") {
			t.Fatal("URI alias read was allowed", err, out)
		}
	}
	if err := savePlanning(aliasContext(selected), map[string]any{"name": "mustneverbeclaimed"}); err == nil {
		t.Fatal("URI alias writer/preflight was allowed")
	}
	afterSelected, _ := os.ReadFile(selected)
	afterWrong, _ := os.ReadFile(wrong)
	if !bytes.Equal(beforeSelected, afterSelected) || !bytes.Equal(beforeWrong, afterWrong) {
		t.Fatal("URI alias guard changed selected or wrong database")
	}
}

func TestPrivateSnapshotURIHandlesTemporaryDirectoryMetacharacters(t *testing.T) {
	home := testenv.Isolate(t)
	selected := filepath.Join(home, "selected.db")
	if err := savePlanning(aliasContext(selected), map[string]any{"name": "selectedsnapshotword"}); err != nil {
		t.Fatal(err)
	}
	tempParent := filepath.Join(home, "scratch%41?#")
	if err := os.MkdirAll(tempParent, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tempParent)
	db, guard, err := OpenPlanningReadOnly(aliasContext(selected), "")
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	defer db.Close()
	var data string
	if err := db.DB().QueryRow("SELECT data FROM resources WHERE resource_type='planning_snapshot' LIMIT 1").Scan(&data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "selectedsnapshotword") {
		t.Fatal("temporary snapshot URI selected another path", data)
	}
	if err := guard.Check(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(tempParent)
	if err != nil || len(entries) != 0 {
		t.Fatal("temporary snapshot escaped cleanup", err, entries)
	}
}
