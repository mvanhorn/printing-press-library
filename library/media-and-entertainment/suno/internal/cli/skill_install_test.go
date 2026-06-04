// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillEmbeddedMatchesRepo guards against drift between the canonical
// repo-root SKILL.md and the copy embedded into the binary for `skill install`.
func TestSkillEmbeddedMatchesRepo(t *testing.T) {
	repo, err := os.ReadFile(filepath.Join("..", "..", "SKILL.md"))
	if err != nil {
		t.Fatalf("read repo SKILL.md: %v", err)
	}
	if string(repo) != sunoSkillMarkdown {
		t.Errorf("embedded skill_suno.md is out of sync with repo SKILL.md; re-copy SKILL.md to internal/cli/skill_suno.md")
	}
}

func TestResolveSkillAgents(t *testing.T) {
	all, err := resolveSkillAgents("all")
	if err != nil || len(all) != 3 {
		t.Fatalf("all = %v (err %v), want 3 agents", all, err)
	}
	one, err := resolveSkillAgents("claude")
	if err != nil || len(one) != 1 || one[0] != "claude" {
		t.Errorf("claude = %v (err %v)", one, err)
	}
	if _, err := resolveSkillAgents("emacs"); err == nil {
		t.Errorf("expected error for unknown agent")
	}
}

func TestSkillPathForAgent(t *testing.T) {
	t.Setenv("HOME", "/tmp/fake-home")
	claude, _ := skillPathForAgent("claude")
	if !strings.HasSuffix(claude, filepath.Join(".claude", "skills", "suno", "SKILL.md")) {
		t.Errorf("claude path = %q", claude)
	}
	codex, _ := skillPathForAgent("codex")
	if !strings.HasSuffix(codex, filepath.Join(".codex", "skills", "suno", "SKILL.md")) {
		t.Errorf("codex path = %q", codex)
	}
	cursor, _ := skillPathForAgent("cursor")
	if cursor != filepath.Join(".cursor", "rules", "suno.mdc") {
		t.Errorf("cursor path = %q, want workspace-relative .cursor/rules/suno.mdc", cursor)
	}
}

func TestSkillContentForAgent_Cursor(t *testing.T) {
	c := skillContentForAgent("cursor")
	if !strings.HasPrefix(c, "---\ndescription: ") {
		t.Errorf("cursor content missing Cursor frontmatter: %.40q", c)
	}
	if !strings.Contains(c, "alwaysApply: false") {
		t.Errorf("cursor content missing alwaysApply")
	}
	// The original SKILL frontmatter (name: pp-suno) must be stripped.
	if strings.Contains(c, "name: pp-suno") {
		t.Errorf("cursor content should not retain the SKILL frontmatter")
	}
}

func TestSplitFrontmatter(t *testing.T) {
	front, body := splitFrontmatter("---\nname: x\ndescription: hi\n---\n\nhello body")
	if !strings.Contains(front, "description: hi") {
		t.Errorf("front = %q", front)
	}
	if !strings.HasPrefix(body, "hello body") {
		t.Errorf("body = %q", body)
	}
	if _, b := splitFrontmatter("no frontmatter here"); b != "no frontmatter here" {
		t.Errorf("body without frontmatter = %q", b)
	}
}

func TestSkillInstall_WriteSkipForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dest := filepath.Join(home, ".claude", "skills", "suno", "SKILL.md")

	run := func(extra ...string) string {
		cmd := newSkillInstallCmd(&rootFlags{})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"--agent", "claude"}, extra...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", extra, err)
		}
		return out.String()
	}

	if got := run(); !strings.Contains(got, "wrote claude") {
		t.Errorf("first install output = %q", got)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("SKILL.md not written: %v", err)
	}
	if got := run(); !strings.Contains(got, "skipped claude") {
		t.Errorf("second install (no force) = %q, want skipped", got)
	}
	if got := run("--force"); !strings.Contains(got, "wrote claude") {
		t.Errorf("force install = %q, want wrote", got)
	}
}
