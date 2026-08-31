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

// TestNovelEpisodeImportHelpWires smoke-tests that the episode import command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelEpisodeImportHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"episode", "import", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("episode import --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "import"} {
		if !strings.Contains(help, want) {
			t.Fatalf("episode import --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelEpisodeImportCombinesScribeAndImagesFolders(t *testing.T) {
	dir := t.TempDir()
	scribeDir := filepath.Join(dir, "scribe-output")
	imagesDir := filepath.Join(dir, "images")
	if err := os.Mkdir(scribeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(imagesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	script := radioPlayScript{
		Title: "The Stone Golem Awakens",
		Elements: []scriptElement{
			{ElementType: "dialogue", Speaker: "DM", CharacterName: "Fade", Text: "The golem's eyes flicker to life."},
		},
	}
	raw, err := json.Marshal(script)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scribeDir, "Session12_recap_script.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "fade_portrait.png"), []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, "queue.json")
	cmd := RootCmd()
	cmd.SetArgs([]string{"episode", "import", "--scribe-folder", scribeDir, "--images-folder", imagesDir, "--out", outPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("episode import execute error = %v, output:\n%s", err, out.String())
	}

	queueRaw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected queue file at %s: %v", outPath, err)
	}
	var queue promptQueue
	if err := json.Unmarshal(queueRaw, &queue); err != nil {
		t.Fatalf("queue file is not valid JSON: %v", err)
	}
	if len(queue.Shots) != 1 || queue.Shots[0].SeedImage == "" {
		t.Fatalf("expected 1 shot matched to fade_portrait.png, got %+v", queue.Shots)
	}
}
