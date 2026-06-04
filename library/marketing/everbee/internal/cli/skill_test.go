// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillDestinationsAllTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dests, err := skillDestinations([]string{"all"}, defaultSkillName)
	if err != nil {
		t.Fatalf("skillDestinations() error = %v", err)
	}
	wantClaude := filepath.Join(home, ".claude", "skills", defaultSkillName, "SKILL.md")
	wantCodex := filepath.Join(home, ".codex", "skills", defaultSkillName, "SKILL.md")
	if dests["claude"] != wantClaude {
		t.Fatalf("claude destination = %q, want %q", dests["claude"], wantClaude)
	}
	if dests["codex"] != wantCodex {
		t.Fatalf("codex destination = %q, want %q", dests["codex"], wantCodex)
	}
}

func TestInstallSkillFilesWritesSkill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", defaultSkillName, "SKILL.md")
	results, err := installSkillFiles("skill body", map[string]string{"codex": path}, true, false)
	if err != nil {
		t.Fatalf("installSkillFiles() error = %v", err)
	}
	if len(results) != 1 || results[0].Status != "installed" {
		t.Fatalf("results = %+v, want installed result", results)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(got) != "skill body" {
		t.Fatalf("installed skill = %q", got)
	}
}

func TestInstallSkillFilesHonorsDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", defaultSkillName, "SKILL.md")
	results, err := installSkillFiles("skill body", map[string]string{"claude": path}, true, true)
	if err != nil {
		t.Fatalf("installSkillFiles(dryRun) error = %v", err)
	}
	if len(results) != 1 || results[0].Status != "would-install" {
		t.Fatalf("results = %+v, want dry-run result", results)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote file or stat failed: %v", err)
	}
}

func TestSkillContentUsesBundledSkill(t *testing.T) {
	content, err := skillContent("")
	if err != nil {
		t.Fatalf("skillContent() error = %v", err)
	}
	if content == "" || !containsAll(content, []string{"name: pp-everbee", "EverBee"}) {
		t.Fatalf("bundled skill content missing expected markers")
	}
}

func containsAll(value string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}
