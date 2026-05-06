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
	if err := os.WriteFile(skillFile, []byte("STALE SYNTHESIS"), 0644); err != nil {
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
		t.Errorf("upstream should overwrite stale synthesis\nwant: %q\ngot:  %q", upstream, got)
	}
}

func TestCopyUpstreamSkill_EmptyFallsThrough(t *testing.T) {
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
		t.Error("expected copied=false for empty/whitespace upstream so synthesis can take over")
	}
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Errorf("expected destination not to exist when upstream is empty, stat err=%v", err)
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

// setupFixture writes a minimal working tree that main() expects:
//   - registry.json at root
//   - tools/generate-skills/skill-template.md (copied from the real template)
//   - cli-skills/ (output dir)
func setupFixture(t *testing.T, root string, entries []RegistryEntry) {
	t.Helper()

	registry := Registry{SchemaVersion: 1, Entries: entries}
	regJSON := fmt.Sprintf(`{"schema_version":1,"entries":[`)
	for i, e := range entries {
		if i > 0 {
			regJSON += ","
		}
		regJSON += fmt.Sprintf(`{"name":%q,"category":%q,"api":%q,"description":%q,"path":%q}`,
			e.Name, e.Category, e.API, e.Description, e.Path)
	}
	regJSON += `]}`
	_ = registry // silence unused warning when fields unused
	if err := os.WriteFile(filepath.Join(root, "registry.json"), []byte(regJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy the real template so the synthesis path works.
	srcDir, _ := os.Getwd()
	tmplSrc, err := os.ReadFile(filepath.Join(srcDir, "skill-template.md"))
	if err != nil {
		t.Fatal(err)
	}
	tmplDir := filepath.Join(root, "tools", "generate-skills")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "skill-template.md"), tmplSrc, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "cli-skills"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_UpstreamPreferredOverSynthesis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	entries := []RegistryEntry{
		{Name: "with-upstream-pp-cli", Category: "commerce", API: "With Upstream",
			Description: "Has upstream skill", Path: "library/commerce/with-upstream"},
		{Name: "no-upstream-pp-cli", Category: "commerce", API: "No Upstream",
			Description: "No upstream skill", Path: "library/commerce/no-upstream"},
	}
	setupFixture(t, root, entries)

	// Only the first entry ships an upstream SKILL.md.
	upstreamDir := filepath.Join(root, "library", "commerce", "with-upstream")
	if err := os.MkdirAll(upstreamDir, 0755); err != nil {
		t.Fatal(err)
	}
	upstreamContent := "---\nname: pp-with-upstream\ndescription: \"Authored upstream with research context.\"\n---\n\n# Upstream Skill\n\nNovel features and narrative.\n"
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"), []byte(upstreamContent), 0644); err != nil {
		t.Fatal(err)
	}
	noUpstreamDir := filepath.Join(root, "library", "commerce", "no-upstream")
	if err := os.MkdirAll(noUpstreamDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tool exited with error: %v\n%s", err, out)
	}
	outStr := string(out)

	// Upstream entry should be copied byte-for-byte.
	upstreamSkill, err := os.ReadFile(filepath.Join(root, "cli-skills", "pp-with-upstream", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading upstream-copied skill: %v", err)
	}
	if string(upstreamSkill) != upstreamContent {
		t.Errorf("upstream skill not copied byte-for-byte\nwant: %q\ngot:  %q", upstreamContent, upstreamSkill)
	}
	if !strings.Contains(outStr, "(upstream)") {
		t.Errorf("expected (upstream) status in output, got:\n%s", outStr)
	}

	// Non-upstream entry should be synthesized from the template.
	synthSkill, err := os.ReadFile(filepath.Join(root, "cli-skills", "pp-no-upstream", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading synthesized skill: %v", err)
	}
	synthStr := string(synthSkill)
	if !strings.Contains(synthStr, "name: pp-no-upstream") {
		t.Errorf("synthesized skill missing expected frontmatter name:\n%s", synthStr)
	}
	if !strings.Contains(synthStr, "Printing Press CLI for No Upstream") {
		t.Errorf("synthesized skill missing expected description:\n%s", synthStr)
	}
	if strings.Contains(synthStr, "Authored upstream") {
		t.Errorf("synthesized skill should not leak upstream content:\n%s", synthStr)
	}

	// Summary should count one upstream and one registry-only.
	if !strings.Contains(outStr, "1 upstream") {
		t.Errorf("expected summary to report 1 upstream, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "1 registry-only") {
		t.Errorf("expected summary to report 1 registry-only, got:\n%s", outStr)
	}
}

func TestIntegration_UpstreamOverwritesStaleSynthesis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	entries := []RegistryEntry{
		{Name: "api-pp-cli", Category: "commerce", API: "API",
			Description: "Has upstream skill", Path: "library/commerce/api"},
	}
	setupFixture(t, root, entries)

	// Pre-seed a stale synthesized skill.
	staleDir := filepath.Join(root, "cli-skills", "pp-api")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, "SKILL.md"), []byte("STALE SYNTHESIZED CONTENT"), 0644); err != nil {
		t.Fatal(err)
	}

	// Ship a fresh upstream SKILL.md.
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
		t.Errorf("upstream should overwrite stale synthesis\nwant: %q\ngot:  %q", upstreamContent, got)
	}
}

// TestBuildOpenClawMetadata_NoAuth locks in the canonical multi-line YAML
// shape for a CLI without auth env vars. The byte-shape here must match
// what cli-printing-press's tools/migrate-skill-metadata/ emits — see
// the migration tool's emitMetadataBlock for the source of truth.
func TestBuildOpenClawMetadata_NoAuth(t *testing.T) {
	ctx := SkillContext{
		CLIBinary:   "dub-pp-cli",
		InstallPath: "library/marketing/dub",
	}
	want := "metadata:\n" +
		"  openclaw:\n" +
		"    requires:\n" +
		"      bins:\n" +
		"        - dub-pp-cli\n" +
		"    install:\n" +
		"      - kind: go\n" +
		"        bins: [dub-pp-cli]\n" +
		"        module: github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli\n"
	got := buildOpenClawMetadata(ctx)
	if got != want {
		t.Errorf("byte-shape mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestBuildOpenClawMetadata_ApiKey locks in the env+primaryEnv shape for
// api_key auth flavor.
func TestBuildOpenClawMetadata_ApiKey(t *testing.T) {
	ctx := SkillContext{
		CLIBinary:   "kalshi-pp-cli",
		InstallPath: "library/payments/kalshi",
		AuthType:    "api_key",
		EnvVars:     []string{"KALSHI_API_KEY", "KALSHI_PRIVATE_KEY_PATH"},
	}
	want := "metadata:\n" +
		"  openclaw:\n" +
		"    requires:\n" +
		"      bins:\n" +
		"        - kalshi-pp-cli\n" +
		"      env:\n" +
		"        - KALSHI_API_KEY\n" +
		"        - KALSHI_PRIVATE_KEY_PATH\n" +
		"    primaryEnv: KALSHI_API_KEY\n" +
		"    install:\n" +
		"      - kind: go\n" +
		"        bins: [kalshi-pp-cli]\n" +
		"        module: github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-cli\n"
	got := buildOpenClawMetadata(ctx)
	if got != want {
		t.Errorf("byte-shape mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestBuildOpenClawMetadata_AgentCaptureBareName confirms the bare-name
// install convention (cli_name without the -pp-cli suffix) is preserved.
func TestBuildOpenClawMetadata_AgentCaptureBareName(t *testing.T) {
	ctx := SkillContext{
		CLIBinary:   "agent-capture",
		InstallPath: "library/developer-tools/agent-capture",
	}
	got := buildOpenClawMetadata(ctx)
	if !strings.Contains(got, "        - agent-capture\n") {
		t.Error("requires.bins should contain bare name")
	}
	if !strings.Contains(got, "        bins: [agent-capture]\n") {
		t.Error("install[].bins should contain bare name")
	}
	if !strings.Contains(got, "/cmd/agent-capture\n") {
		t.Error("module should end with bare-name cmd path")
	}
}

// TestBuildOpenClawMetadata_NeverEmitsLegacyShape locks out the old
// JSON-string-blob and kind: shell shape in case future maintainers
// accidentally regress the function.
func TestBuildOpenClawMetadata_NeverEmitsLegacyShape(t *testing.T) {
	ctx := SkillContext{
		CLIBinary:   "x-pp-cli",
		InstallPath: "library/other/x",
	}
	got := buildOpenClawMetadata(ctx)
	if strings.Contains(got, `'{"openclaw"`) {
		t.Error("must not emit JSON-string blob")
	}
	if strings.Contains(got, `kind: shell`) {
		t.Error("must not emit kind: shell")
	}
	if strings.Contains(got, "command:") {
		t.Error("must not emit command: field")
	}
	if strings.Contains(got, "id:") {
		t.Error("must not emit id: field")
	}
	if strings.Contains(got, "label:") {
		t.Error("must not emit label: field")
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
