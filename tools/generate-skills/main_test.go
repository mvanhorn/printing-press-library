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

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-yahoo-finance")
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

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-no-upstream")
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

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-yahoo-finance")
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

// TestInjectStaleBuildFallback covers the copy-time augmentation that
// teaches upstream (hand-authored) SKILL.md files the same @main
// fallback the generator template already emits. See the 2026-04-19
// stale-@latest bug for context.
func TestInjectStaleBuildFallback(t *testing.T) {
	t.Run("injects after go install @latest line in a code block", func(t *testing.T) {
		in := []byte("## CLI Installation\n\n```bash\ngo install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest\nyahoo-finance-pp-cli --version\n```\n")
		got := string(injectStaleBuildFallback(in))
		if !strings.Contains(got, "@main") {
			t.Errorf("expected @main fallback injected, got:\n%s", got)
		}
		if !strings.Contains(got, "GOPRIVATE='github.com/mvanhorn/*'") {
			t.Errorf("expected GOPRIVATE guidance injected, got:\n%s", got)
		}
		if !strings.Contains(got, "yahoo-finance-pp-cli@main") {
			t.Errorf("expected binary-specific @main line, got:\n%s", got)
		}
	})

	t.Run("is idempotent when @main already present", func(t *testing.T) {
		in := []byte("go install github.com/mvanhorn/printing-press-library/library/x/cmd/x-pp-cli@latest\n\n# fallback: go install ...@main\n")
		got := injectStaleBuildFallback(in)
		if string(got) != string(in) {
			t.Errorf("expected idempotent no-op when @main already present\nwant: %q\ngot:  %q", in, got)
		}
	})

	t.Run("no go install lines means no injection", func(t *testing.T) {
		in := []byte("# Title\n\nNarrative content with no install commands.\n")
		got := injectStaleBuildFallback(in)
		if string(got) != string(in) {
			t.Errorf("expected no-op when no @latest install line present")
		}
	})

	t.Run("skips metadata-JSON @latest since it is not a plain install line", func(t *testing.T) {
		// The frontmatter metadata line is long and uses JSON quoting.
		// Our regex requires a line starting with optional whitespace
		// then `go install` — metadata lines do NOT satisfy this.
		in := []byte("metadata: '{\"openclaw\":{\"install\":[{\"command\":\"go install github.com/mvanhorn/printing-press-library/library/x/cmd/x-pp-cli@latest\"}]}}'\n")
		got := injectStaleBuildFallback(in)
		if string(got) != string(in) {
			t.Errorf("expected metadata @latest to be left alone (it is not a user-runnable line)")
		}
	})
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

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-blank")
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
//   - plugin/.claude-plugin/plugin.json (so version bump can run without a warning)
//   - plugin/skills/ (output dir)
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

	// Also copy the command-template.md so the command-shim emitter works.
	cmdTmplSrc, err := os.ReadFile(filepath.Join(srcDir, "command-template.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "command-template.md"), cmdTmplSrc, 0644); err != nil {
		t.Fatal(err)
	}

	// Minimal plugin.json so the version-bump path doesn't warn.
	pluginDir := filepath.Join(root, "plugin", ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"version": "1.0.0"}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "plugin", "skills"), 0755); err != nil {
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
	upstreamSkill, err := os.ReadFile(filepath.Join(root, "plugin", "skills", "pp-with-upstream", "SKILL.md"))
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
	synthSkill, err := os.ReadFile(filepath.Join(root, "plugin", "skills", "pp-no-upstream", "SKILL.md"))
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
	staleDir := filepath.Join(root, "plugin", "skills", "pp-api")
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

func TestIntegration_CommandShimsWrittenForEverySkill(t *testing.T) {
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

	upstreamDir := filepath.Join(root, "library", "commerce", "with-upstream")
	if err := os.MkdirAll(upstreamDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstreamDir, "SKILL.md"),
		[]byte("---\nname: pp-with-upstream\ndescription: \"u\"\n---\nbody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "library", "commerce", "no-upstream"), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tool exited with error: %v\n%s", err, out)
	}

	// Both upstream and registry-only paths should emit a command shim.
	wantCommands := []string{"pp-with-upstream.md", "pp-no-upstream.md"}
	for _, name := range wantCommands {
		cmdFile := filepath.Join(root, "plugin", "commands", name)
		data, err := os.ReadFile(cmdFile)
		if err != nil {
			t.Errorf("command shim missing for %s: %v", name, err)
			continue
		}
		body := string(data)
		if !strings.Contains(body, "description:") {
			t.Errorf("command %s missing description frontmatter:\n%s", name, body)
		}
		skillName := strings.TrimSuffix(name, ".md")
		if !strings.Contains(body, "`"+skillName+"`") {
			t.Errorf("command %s should reference its skill %q in body:\n%s", name, skillName, body)
		}
		if !strings.Contains(body, "$ARGUMENTS") {
			t.Errorf("command %s should forward $ARGUMENTS:\n%s", name, body)
		}
	}
}

func TestIntegration_CommandShimIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	bin := buildTool(t)
	root := t.TempDir()

	entries := []RegistryEntry{
		{Name: "foo-pp-cli", Category: "commerce", API: "Foo",
			Description: "foo", Path: "library/commerce/foo"},
	}
	setupFixture(t, root, entries)
	if err := os.MkdirAll(filepath.Join(root, "library", "commerce", "foo"), 0755); err != nil {
		t.Fatal(err)
	}

	run := func() []byte {
		cmd := exec.Command(bin)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("tool exited with error: %v\n%s", err, out)
		}
		data, err := os.ReadFile(filepath.Join(root, "plugin", "commands", "pp-foo.md"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first := run()
	second := run()
	if string(first) != string(second) {
		t.Errorf("command shim should be idempotent across runs\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
