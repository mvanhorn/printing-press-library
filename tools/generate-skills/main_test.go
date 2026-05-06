package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	srcDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	binName := "generate-skills"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}

// writeRegistry writes a minimal registry.json fixture at root.
func writeRegistry(t *testing.T, root string, entries []RegistryEntry) {
	t.Helper()
	regJSON := `{"schema_version":1,"entries":[`
	for i, e := range entries {
		if i > 0 {
			regJSON += ","
		}
		regJSON += fmt.Sprintf(`{"name":%q,"path":%q}`, e.Name, e.Path)
	}
	regJSON += `]}`
	if err := os.WriteFile(filepath.Join(root, "registry.json"), []byte(regJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cli-skills"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_CopiesUpstreamVerbatim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	entries := []RegistryEntry{
		{Name: "yahoo-finance-pp-cli", Path: "library/commerce/yahoo-finance"},
	}
	writeRegistry(t, root, entries)

	upstreamDir := filepath.Join(root, "library", "commerce", "yahoo-finance")
	if err := os.MkdirAll(upstreamDir, 0755); err != nil {
		t.Fatal(err)
	}
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

func TestIntegration_FailsWhenUpstreamMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	entries := []RegistryEntry{
		{Name: "with-upstream-pp-cli", Path: "library/commerce/with-upstream"},
		{Name: "no-upstream-pp-cli", Path: "library/commerce/no-upstream"},
	}
	writeRegistry(t, root, entries)

	upstreamDir := filepath.Join(root, "library", "commerce", "with-upstream")
	if err := os.MkdirAll(upstreamDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"), []byte("---\nname: pp-with-upstream\n---\n\n# Has Upstream\n"), 0644); err != nil {
		t.Fatal(err)
	}
	noUpstreamDir := filepath.Join(root, "library", "commerce", "no-upstream")
	if err := os.MkdirAll(noUpstreamDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("tool should have exited non-zero when an entry has no upstream SKILL.md\noutput:\n%s", out)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "no-upstream-pp-cli") {
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

	entries := []RegistryEntry{
		{Name: "api-pp-cli", Path: "library/commerce/api"},
	}
	writeRegistry(t, root, entries)

	staleDir := filepath.Join(root, "cli-skills", "pp-api")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, "SKILL.md"), []byte("STALE MIRROR CONTENT"), 0644); err != nil {
		t.Fatal(err)
	}

	upstreamDir := filepath.Join(root, "library", "commerce", "api")
	if err := os.MkdirAll(upstreamDir, 0755); err != nil {
		t.Fatal(err)
	}
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
	mustMkdir("pp-flightgoat")     // orphan: registry no longer has it
	mustMkdir("not-a-pp-dir")      // unrelated content, must be preserved
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
