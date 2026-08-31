// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelScriptDraftPromptsHelpWires smoke-tests that the script draft-prompts command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelScriptDraftPromptsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"script", "draft-prompts", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("script draft-prompts --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "draft-prompts"} {
		if !strings.Contains(help, want) {
			t.Fatalf("script draft-prompts --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelScriptDraftPromptsDraftsQueueFromRealScribeShape(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "recap_script.json")
	script := radioPlayScript{
		Title: "The Stone Golem Awakens",
		Elements: []scriptElement{
			{ElementType: "dialogue", Speaker: "DM", CharacterName: "Fade", Text: "The golem's eyes flicker to life."},
			{ElementType: "sfx", Description: "stone grinding against stone"},
		},
	}
	raw, err := json.Marshal(script)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	imagesDir := filepath.Join(dir, "images")
	if err := os.Mkdir(imagesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "fade_portrait.png"), []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, "queue.json")
	cmd := RootCmd()
	cmd.SetArgs([]string{"script", "draft-prompts", scriptPath, "--images-dir", imagesDir, "--out", outPath, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("draft-prompts execute error = %v, output:\n%s", err, out.String())
	}

	queueRaw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected queue file at %s: %v", outPath, err)
	}
	var queue promptQueue
	if err := json.Unmarshal(queueRaw, &queue); err != nil {
		t.Fatalf("queue file is not valid JSON: %v", err)
	}
	if len(queue.Shots) != 2 {
		t.Fatalf("expected 2 shots, got %d", len(queue.Shots))
	}
	if queue.Shots[0].SeedImage == "" {
		t.Fatalf("expected shot 0 (character_name=Fade) to match fade_portrait.png, got no match")
	}
	if queue.Shots[1].ElementType != "sfx" || queue.Shots[1].SeedImage != "" {
		t.Fatalf("expected shot 1 to be an unmatched sfx element, got %+v", queue.Shots[1])
	}
}
