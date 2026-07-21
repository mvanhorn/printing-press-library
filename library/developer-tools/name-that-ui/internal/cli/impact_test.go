// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
)

// TestNovelImpactHelpWires smoke-tests that the impact command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelImpactHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"impact", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("impact --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "impact"} {
		if !strings.Contains(help, want) {
			t.Fatalf("impact --help missing %q in output:\n%s", want, help)
		}
	}
}

func impactComponent(id, name, symbol string) namethatui.Component {
	return namethatui.Component{ID: id, Platform: "swiftui", Slug: strings.ToLower(strings.ReplaceAll(name, " ", "-")), Name: name, API: []namethatui.API{{Framework: "SwiftUI", Symbol: symbol}}, SourceURL: "https://example.test/" + id}
}

func seedImpactDB(t *testing.T, current []namethatui.Component, snapshots []struct {
	component namethatui.Component
	at        time.Time
}) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "impact.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, component := range current {
		raw, _ := json.Marshal(component)
		if err := db.Upsert("components", component.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if len(current) > 0 {
		if err := db.SaveSyncState("components", "", len(current)); err != nil {
			t.Fatal(err)
		}
	}
	for _, snapshot := range snapshots {
		raw, _ := json.Marshal(snapshot.component)
		if _, err := db.AppendSnapshotIfChanged("component_snapshots", snapshot.component.ID, snapshot.at, impactHash(raw), raw, snapshot.component.SourceURL); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImpactChangedComponentSymbolAffectsOnlyMatchingFile(t *testing.T) {
	now := time.Now().UTC()
	before := impactComponent("swiftui/combobox", "Combobox", "OldPicker")
	after := impactComponent("swiftui/combobox", "Combobox", "Picker")
	db := seedImpactDB(t, []namethatui.Component{after}, []struct {
		component namethatui.Component
		at        time.Time
	}{{before, now.Add(-48 * time.Hour)}, {after, now.Add(-time.Hour)}})
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Affected.swift"), []byte("let view = Picker(\"Choice\")"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Unrelated.swift"), []byte("let value = 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "impact", root, "--since", "24h", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	var got impactResponse
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Changes) != 1 || got.Changes[0].Status != "changed" || len(got.Files) != 1 || got.Files[0].Path != "Affected.swift" || len(got.Files[0].Matches) != 1 || got.Files[0].Matches[0].Symbol != "Picker" {
		t.Fatalf("impact = %#v", got)
	}
}

func TestImpactNoBaselineAndRemovedEntity(t *testing.T) {
	now := time.Now().UTC()
	newer := impactComponent("swiftui/new", "New Control", "NewControl")
	noBaselineDB := seedImpactDB(t, []namethatui.Component{newer}, []struct {
		component namethatui.Component
		at        time.Time
	}{{newer, now.Add(-time.Hour)}})
	root := t.TempDir()
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "impact", root, "--since", "24h", "--db", noBaselineDB)
	if err != nil {
		t.Fatal(err)
	}
	var noBaseline impactResponse
	if err := json.Unmarshal([]byte(stdout), &noBaseline); err != nil {
		t.Fatal(err)
	}
	if len(noBaseline.Changes) != 0 || !strings.Contains(noBaseline.Reason, "no snapshot") {
		t.Fatalf("no baseline = %#v", noBaseline)
	}
	stable := impactComponent("swiftui/stable", "Stable", "Stable")
	removed := impactComponent("swiftui/removed", "Removed", "RemovedControl")
	removedDB := seedImpactDB(t, []namethatui.Component{stable}, []struct {
		component namethatui.Component
		at        time.Time
	}{{stable, now.Add(-48 * time.Hour)}, {removed, now.Add(-48 * time.Hour)}})
	stdout, _, err = runRootArgs(t, "--json", "--no-learn", "impact", root, "--since", now.Add(-24*time.Hour).Format("2006-01-02"), "--db", removedDB)
	if err != nil {
		t.Fatal(err)
	}
	var removedResult impactResponse
	if err := json.Unmarshal([]byte(stdout), &removedResult); err != nil {
		t.Fatal(err)
	}
	if len(removedResult.Changes) != 1 || removedResult.Changes[0].EntityID != removed.ID || removedResult.Changes[0].Status != "removed" {
		t.Fatalf("removed = %#v", removedResult)
	}
}

func TestImpactReportsEntityRemovedByAuthoritativeSync(t *testing.T) {
	present := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		entries := `[]`
		if present {
			entries = `[{"slug":"button","platform":"react","name":"Button","api":[],"parts":[]}]`
		} else {
			entries = `[{"slug":"select","platform":"react","name":"Select","api":[],"parts":[]}]`
		}
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"itemListElement":[]}</script><script>` + nameThatUIPush(`{"entries":`+entries+`}`) + `</script>`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "impact.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	old := impactComponent("react/button", "Button", "OldButton")
	oldRaw, _ := json.Marshal(old)
	if _, err := db.AppendSnapshotIfChanged("component_snapshots", old.ID, time.Now().Add(-48*time.Hour), impactHash(oldRaw), oldRaw, old.SourceURL); err != nil {
		t.Fatal(err)
	}
	if _, err := namethatui.Sync(context.Background(), server.Client(), server.URL, db, true, false); err != nil {
		t.Fatal(err)
	}
	present = false
	if _, err := namethatui.Sync(context.Background(), server.Client(), server.URL, db, true, false); err != nil {
		t.Fatal(err)
	}
	current, err := db.List("components", 0)
	if err != nil || len(current) != 1 || strings.Contains(string(current[0]), old.ID) {
		t.Fatalf("removed current mirror = %q, %v", current, err)
	}
	snapshots, err := db.List("component_snapshots", 0)
	if err != nil || len(snapshots) == 0 {
		t.Fatalf("immutable history was removed: %d, %v", len(snapshots), err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "impact", root, "--since", "24h", "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	var result impactResponse
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	foundRemoved := false
	for _, change := range result.Changes {
		if change.EntityID == old.ID && change.Status == "removed" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Fatalf("sync-removal impact = %#v", result.Changes)
	}
}

func TestImpactSinceParsingDryRunAndMissingSnapshotMirror(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if got, err := impactSince("48h", now); err != nil || !got.Equal(now.Add(-48*time.Hour)) {
		t.Fatalf("duration since = %v, %v", got, err)
	}
	if got, err := impactSince("2026-07-01", now); err != nil || got.Format("2006-01-02") != "2026-07-01" {
		t.Fatalf("date since = %v, %v", got, err)
	}
	if _, err := impactSince("0h", now); err == nil {
		t.Fatal("zero duration should fail")
	}
	root := t.TempDir()
	missing := filepath.Join(root, "missing.db")
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "impact", root, "--since", "7d", "--db", missing, "--dry-run")
	if err != nil || !strings.Contains(stdout, `"walked": false`) || !strings.Contains(stdout, `"sqlite_opened": false`) {
		t.Fatalf("dry run = %q, %v", stdout, err)
	}
	stdout, _, err = runRootArgs(t, "--json", "--no-learn", "impact", "--db", missing, "--dry-run")
	if err != nil {
		t.Fatalf("empty dry run error = %v", err)
	}
	var emptyDryRun impactResponse
	if err := json.Unmarshal([]byte(stdout), &emptyDryRun); err != nil {
		t.Fatalf("empty dry run JSON = %q: %v", stdout, err)
	}
	if emptyDryRun.Path != "" || emptyDryRun.Since != "" || !emptyDryRun.DryRun || emptyDryRun.Walked || emptyDryRun.SQLiteOpened || emptyDryRun.Changes == nil || emptyDryRun.Files == nil {
		t.Fatalf("empty dry run = %#v", emptyDryRun)
	}
	if _, _, err := runRootArgs(t, "--no-learn", "impact", root, "--since", "7d", "--db", missing); err == nil || !strings.Contains(err.Error(), "snapshot mirror is unavailable") {
		t.Fatalf("missing mirror error = %v", err)
	}
}
