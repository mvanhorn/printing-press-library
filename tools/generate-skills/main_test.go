package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	toolBinOnce sync.Once
	toolBinPath string
	toolBinDir  string
	toolBinErr  error
)

func TestCopyUpstreamSkill_Present(t *testing.T) {
	tmp := t.TempDir()

	entryPath := filepath.Join(tmp, "library", "commerce", "yahoo-finance")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}
	upstream := []byte("---\nname: pp-yahoo-finance\ndescription: \"Upstream content with `backticks` and \\\"quotes\\\"\"\n---\n\n# Yahoo Finance\n\nNarrative content.\n")
	if err := os.WriteFile(filepath.Join(entryPath, "SKILL.md"), upstream, 0644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "cli-skills", "pp-yahoo-finance")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !copied {
		t.Fatal("expected copied=true when upstream SKILL.md exists")
	}

	got, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("reading destination: %v", err)
	}
	if string(got) != string(upstream) {
		t.Errorf("destination content does not match upstream byte-for-byte\nwant: %q\ngot:  %q", upstream, got)
	}
}

func TestCopyUpstreamSkill_Absent(t *testing.T) {
	tmp := t.TempDir()

	entryPath := filepath.Join(tmp, "library", "commerce", "no-upstream")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "cli-skills", "pp-no-upstream")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error when upstream missing: %v", err)
	}
	if copied {
		t.Error("expected copied=false when upstream SKILL.md missing")
	}
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Errorf("expected destination not to exist, stat err=%v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("expected skill dir not to be created when no upstream, stat err=%v", err)
	}
}

func TestCopyUpstreamSkill_OverwritesExisting(t *testing.T) {
	tmp := t.TempDir()

	entryPath := filepath.Join(tmp, "library", "commerce", "yahoo-finance")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}
	upstream := []byte("UPSTREAM CONTENT")
	if err := os.WriteFile(filepath.Join(entryPath, "SKILL.md"), upstream, 0644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "cli-skills", "pp-yahoo-finance")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("STALE CONTENT"), 0644); err != nil {
		t.Fatal(err)
	}

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !copied {
		t.Fatal("expected copied=true")
	}

	got, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(upstream) {
		t.Errorf("upstream should overwrite stale content\nwant: %q\ngot:  %q", upstream, got)
	}
}

func TestCopyUpstreamSkill_EmptyTreatedAsMissing(t *testing.T) {
	tmp := t.TempDir()
	entryPath := filepath.Join(tmp, "library", "commerce", "blank")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entryPath, "SKILL.md"), []byte("   \n\t\n"), 0644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "cli-skills", "pp-blank")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if copied {
		t.Error("expected copied=false for empty/whitespace upstream")
	}
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Errorf("expected destination not to be written when upstream is empty, stat err=%v", err)
	}
}

// buildTool compiles the generate-skills binary into a tempdir and returns its path.
func buildTool(t *testing.T) string {
	t.Helper()
	toolBinOnce.Do(func() {
		srcDir, err := os.Getwd()
		if err != nil {
			toolBinErr = err
			return
		}
		binName := "generate-skills"
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		toolBinDir, err = os.MkdirTemp("", "generate-skills-test-*")
		if err != nil {
			toolBinErr = err
			return
		}
		toolBinPath = filepath.Join(toolBinDir, binName)
		cmd := exec.Command("go", "build", "-o", toolBinPath, ".")
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			toolBinErr = fmt.Errorf("go build failed: %v\n%s", err, out)
		}
	})
	if toolBinErr != nil {
		t.Fatal(toolBinErr)
	}
	return toolBinPath
}

func TestMain(m *testing.M) {
	code := m.Run()
	if toolBinDir != "" {
		_ = os.RemoveAll(toolBinDir)
	}
	os.Exit(code)
}

// writeManifest writes a minimal .printing-press.json fixture for a library CLI.
func writeManifest(t *testing.T, root, category, slug, apiName string) string {
	t.Helper()
	dir := filepath.Join(root, "library", category, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(PrintManifest{APIName: apiName})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".printing-press.json"), manifest, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestIntegration_CopiesUpstreamVerbatim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	upstreamDir := writeManifest(t, root, "commerce", "yahoo-finance", "yahoo-finance")
	upstreamContent := "---\nname: pp-yahoo-finance\ndescription: \"Authored upstream with research context.\"\n---\n\n# Upstream Skill\n\nNovel features and narrative.\n"
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"), []byte(upstreamContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tool exited with error: %v\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(root, "cli-skills", "pp-yahoo-finance", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading copied skill: %v", err)
	}
	if string(got) != upstreamContent {
		t.Errorf("upstream skill not copied byte-for-byte\nwant: %q\ngot:  %q", upstreamContent, got)
	}
	if !strings.Contains(string(out), "Mirrored 1 skill") {
		t.Errorf("expected mirror summary in output, got:\n%s", out)
	}
}

func TestIntegration_DiscoversNewCLIWithoutRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	upstreamDir := writeManifest(t, root, "marketing", "customer-io", "customer-io")
	upstreamContent := "---\nname: pp-customer-io\n---\n\n# Customer.io\n"
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"), []byte(upstreamContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tool exited with error: %v\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(root, "cli-skills", "pp-customer-io", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading copied skill: %v", err)
	}
	if string(got) != upstreamContent {
		t.Errorf("new CLI skill not copied byte-for-byte\nwant: %q\ngot:  %q", upstreamContent, got)
	}
}

func TestIntegration_FailsWhenUpstreamMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	upstreamDir := writeManifest(t, root, "commerce", "with-upstream", "with-upstream")
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"), []byte("---\nname: pp-with-upstream\n---\n\n# Has Upstream\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, root, "commerce", "no-upstream", "no-upstream")

	cmd := exec.Command(bin)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("tool should have exited non-zero when an entry has no upstream SKILL.md\noutput:\n%s", out)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "no-upstream") {
		t.Errorf("expected missing entry to be named in error output, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Missing or empty library SKILL.md") {
		t.Errorf("expected missing-skill error message, got:\n%s", outStr)
	}
}

func TestIntegration_UpstreamOverwritesStaleMirror(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	staleDir := filepath.Join(root, "cli-skills", "pp-api")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, "SKILL.md"), []byte("STALE MIRROR CONTENT"), 0644); err != nil {
		t.Fatal(err)
	}

	upstreamDir := writeManifest(t, root, "commerce", "api", "api")
	upstreamContent := "---\nname: pp-api\ndescription: \"Fresh upstream.\"\n---\n\n# Fresh\n"
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"), []byte(upstreamContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tool exited with error: %v\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(staleDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != upstreamContent {
		t.Errorf("upstream should overwrite stale mirror\nwant: %q\ngot:  %q", upstreamContent, got)
	}
}

func TestPruneOrphanSkills(t *testing.T) {
	tmp := t.TempDir()

	// Layout: two registry-backed skills, one orphan, one non-pp dir, one
	// stray file. Only the orphan pp-* dir should be removed.
	mustMkdir := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(tmp, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir("pp-flight-goat")
	mustMkdir("pp-recipe-goat")
	mustMkdir("pp-flightgoat") // orphan: library no longer has it
	mustMkdir("not-a-pp-dir")  // unrelated content, must be preserved
	if err := os.WriteFile(filepath.Join(tmp, "stray.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	expected := map[string]struct{}{
		"pp-flight-goat": {},
		"pp-recipe-goat": {},
	}
	removed := pruneOrphanSkills(tmp, expected)
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(tmp, "pp-flightgoat")); !os.IsNotExist(err) {
		t.Errorf("pp-flightgoat should have been removed, stat err = %v", err)
	}
	for _, keep := range []string{"pp-flight-goat", "pp-recipe-goat", "not-a-pp-dir", "stray.txt"} {
		if _, err := os.Stat(filepath.Join(tmp, keep)); err != nil {
			t.Errorf("%s should still exist: %v", keep, err)
		}
	}
}

func TestPruneOrphanSkills_DirMissing(t *testing.T) {
	// Fresh checkout where cli-skills/ doesn't exist yet should not error.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	removed := pruneOrphanSkills(missing, map[string]struct{}{})
	if removed != 0 {
		t.Fatalf("removed = %d, want 0 for missing dir", removed)
	}
}
