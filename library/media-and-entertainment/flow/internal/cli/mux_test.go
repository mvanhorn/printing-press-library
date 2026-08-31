// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelMuxHelpWires smoke-tests that the mux command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMuxHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"mux", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mux --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "mux"} {
		if !strings.Contains(help, want) {
			t.Fatalf("mux --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelMuxConcatenatesAndOverlaysAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	clip1 := filepath.Join(dir, "clip1.mp4")
	clip2 := filepath.Join(dir, "clip2.mp4")
	audio := filepath.Join(dir, "audio.mp3")
	out := filepath.Join(dir, "final.mp4")

	genClip := func(path string, seconds int) {
		cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=blue:s=32x32:d=%d", seconds), "-c:v", "libx264", "-pix_fmt", "yuv420p", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generating test clip %s: %v\n%s", path, err, out)
		}
	}
	genClip(clip1, 1)
	genClip(clip2, 1)
	genAudio := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-c:a", "libmp3lame", audio)
	if o, err := genAudio.CombinedOutput(); err != nil {
		t.Fatalf("generating test audio: %v\n%s", err, o)
	}

	beatsPath := filepath.Join(dir, "beats.json")
	beatsRaw, err := json.Marshal(beatSheet{Durations: []float64{1.0, 1.0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beatsPath, beatsRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"mux", clip1, clip2, "--audio", audio, "--beats", beatsPath, "--out", out, "--json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mux execute error = %v, output:\n%s", err, stdout.String())
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("expected output file at %s: %v", out, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty output video, got 0 bytes")
	}
}
