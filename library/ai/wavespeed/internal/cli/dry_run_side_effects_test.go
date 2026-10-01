// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
)

// Regression: download, library export and library tag used to ignore
// --dry-run and write files / tags anyway. Found when the publish live gate
// ran them with --allow-destructive and they dropped files into the CLI tree.

func runDryRunPreview(t *testing.T, args ...string) map[string]any {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\nstderr: %s", args, err, errOut.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("%v: invalid JSON %q: %v", args, out.String(), err)
	}
	if payload["dry_run"] != true {
		t.Fatalf("%v: dry_run = %v, want true", args, payload["dry_run"])
	}
	if a, _ := payload["action"].(string); a == "" {
		t.Fatalf("%v: empty action in %v", args, payload)
	}
	return payload
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return dir
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("--dry-run wrote %d entries into %s", len(entries), dir)
	}
}

func TestDownloadDryRunWritesNothing(t *testing.T) {
	testenv.Isolate(t)
	dir := chdirTemp(t)
	// Unroutable URL: a real fetch would fail, so success proves no request.
	runDryRunPreview(t, "download", "https://127.0.0.1:9/out.png", "--json", "--dry-run")
	assertEmptyDir(t, dir)
}

func TestLibraryExportDryRunWritesNothing(t *testing.T) {
	testenv.Isolate(t)
	dir := chdirTemp(t)
	if err := recordGeneration(store.Generation{Command: "run", ModelID: "m", Prompt: "p", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	runDryRunPreview(t, "library", "export", "exp", "--agent", "--dry-run")
	if _, err := os.Stat(filepath.Join(dir, "exp")); !os.IsNotExist(err) {
		t.Fatalf("--dry-run created the export dir (err=%v)", err)
	}
}

func TestLibraryTagDryRunDoesNotTag(t *testing.T) {
	testenv.Isolate(t)
	g := store.Generation{ID: newGenerationID(), Command: "run", ModelID: "m", Prompt: "p", Status: "completed"}
	if err := recordGeneration(g); err != nil {
		t.Fatal(err)
	}
	runDryRunPreview(t, "library", "tag", g.ID, "--add", "hero", "--agent", "--dry-run")
	s, err := openLibrary()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tags, err := s.TagsFor(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Fatalf("--dry-run added tags: %v", tags)
	}
}

func TestRunDryRunNamesAction(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_API_KEY", "test-key")
	runDryRunPreview(t, "run", "wavespeed-ai/z-image/turbo", "--prompt", "x", "--json", "--dry-run")
}
