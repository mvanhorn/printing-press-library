package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/store"
)

func planningProfileContext(t *testing.T, home, name string) context.Context {
	t.Helper()
	return platform.ContextWithSession(context.Background(), &platform.Session{
		ProfileName: name, GateOutcome: platform.GateVerified,
		Paths: platform.Paths{DataFile: filepath.Join(home, name, "data.db")},
	})
}

func TestPlanningSnapshotsUseVerifiedProfileForSaveReadAndSearch(t *testing.T) {
	home := testenv.Isolate(t)
	names := []string{"alphaevidenceword", "betaevidenceword"}
	contexts := []context.Context{planningProfileContext(t, home, "first"), planningProfileContext(t, home, "second")}
	for i, ctx := range contexts {
		if err := savePlanning(ctx, map[string]any{"name": names[i]}); err != nil {
			t.Fatal(err)
		}
	}
	for i, ctx := range contexts {
		var output bytes.Buffer
		cmd := newNovelHostelsSavedCmd(&rootFlags{asJSON: true})
		cmd.SetContext(ctx)
		cmd.SetOut(&output)
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(output.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		rows := v["results"].([]any)
		if len(rows) != 1 || rows[0].(map[string]any)["name"] != names[i] {
			t.Fatalf("profile cache crossed: %s", output.String())
		}
		for _, command := range [][]string{{"search", names[1-i], "--limit", "5"}, {"workflow", "status"}} {
			flags := &rootFlags{asJSON: true}
			root := &cobra.Command{Use: "test"}
			root.AddCommand(newSearchCmd(flags), newWorkflowCmd(flags))
			configurePlanningCache(root, flags)
			root.SetContext(ctx)
			root.SetArgs(command)
			output.Reset()
			root.SetOut(&output)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if command[0] == "search" {
				if len(result["results"].([]any)) != 0 {
					t.Fatal("CLI search crossed selected profile", output.String())
				}
			} else if result["provider_snapshot_refreshed"] != false || result["counts"].(map[string]any)["planning_snapshot"] != float64(1) {
				t.Fatal("workflow status lost manual/profile semantics", output.String())
			}
		}
		path, _ := PlanningDBPath(ctx, "")
		db, err := store.OpenReadOnlyContext(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		rowsRaw, err := db.Search(names[1-i], 10, "planning_snapshot")
		db.Close()
		if err != nil || len(rowsRaw) != 0 {
			t.Fatalf("search crossed profile: %v %v", rowsRaw, err)
		}
	}
	bad := platform.ContextWithSession(context.Background(), &platform.Session{Paths: platform.Paths{DataFile: filepath.Join(home, "bad.db")}})
	if err := savePlanning(bad, map[string]any{"name": "bad"}); err == nil {
		t.Fatal("unverified profile fell back to shared cache")
	}
	if _, err := PlanningDBPath(contexts[0], filepath.Join(home, "other.db")); err == nil {
		t.Fatal("profile database override escaped scope")
	}
}

func TestPlanningRetentionBoundsResourcesAndFTS(t *testing.T) {
	home := testenv.Isolate(t)
	ctx := planningProfileContext(t, home, "retention")
	path, _ := PlanningDBPath(ctx, "")
	db, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	// Seed a pre-fix orphan plus a different resource type that must survive.
	if _, err := db.DB().Exec("INSERT INTO resources_fts(rowid,id,resource_type,content) VALUES(123,'orphan','planning_snapshot','orphan')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("other", "preserve", json.RawMessage(`{"name":"preserve"}`)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for i := 0; i < 205; i++ {
		name := fmt.Sprintf("boundedcacheitem%03d", i)
		if i == 0 {
			name = "prunedneedleword"
		}
		if i == 204 {
			name = "recentneedleword"
		}
		if err := savePlanning(ctx, map[string]any{"name": name}); err != nil {
			t.Fatal(err)
		}
	}
	db, err = store.OpenReadOnlyContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"resources", "resources_fts"} {
		var count int
		if err := db.DB().QueryRow("SELECT count(*) FROM " + table + " WHERE resource_type='planning_snapshot'").Scan(&count); err != nil || count != 200 {
			t.Fatalf("%s retention count=%d: %v", table, count, err)
		}
	}
	for _, query := range []string{"prunedneedleword", "orphan"} {
		rows, err := db.Search(query, 10, "planning_snapshot")
		if err != nil || len(rows) != 0 {
			t.Fatalf("pruned snapshot matched %q: %v %v", query, rows, err)
		}
	}
	rows, err := db.Search("recentneedleword", 10, "planning_snapshot")
	if err != nil || len(rows) != 1 || !strings.Contains(string(rows[0]), "recentneedleword") {
		t.Fatalf("latest snapshot lost: %v %v", rows, err)
	}
	var others int
	if err := db.DB().QueryRow("SELECT count(*) FROM resources_fts WHERE resource_type='other'").Scan(&others); err != nil || others != 1 {
		t.Fatal("other FTS rows were deleted", err)
	}
}
