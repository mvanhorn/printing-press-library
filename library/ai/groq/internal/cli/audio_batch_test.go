// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
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

// TestNovelAudioBatchHelpWires smoke-tests that the audio batch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAudioBatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"audio", "batch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audio batch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "batch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audio batch --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestIsAudioExt(t *testing.T) {
	cases := map[string]bool{
		".mp3": true, ".wav": true, ".m4a": true, ".ogg": true,
		".flac": true, ".webm": true, ".txt": false, ".json": false, ".md": false,
	}
	for ext, want := range cases {
		if got := isAudioExt(ext); got != want {
			t.Fatalf("isAudioExt(%q) = %v, want %v", ext, got, want)
		}
	}
}

func TestExpandAudioInputs(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.mp3", "b.wav", "note.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	audio, err := expandAudioInputs([]string{dir}, "transcribe")
	if err != nil {
		t.Fatalf("expandAudioInputs transcribe error = %v", err)
	}
	if len(audio) != 2 {
		t.Fatalf("audio files = %v, want 2", audio)
	}
	text, err := expandAudioInputs([]string{dir}, "speech")
	if err != nil {
		t.Fatalf("expandAudioInputs speech error = %v", err)
	}
	if len(text) != 1 || !strings.HasSuffix(text[0], "note.txt") {
		t.Fatalf("text files = %v, want note.txt only", text)
	}
}

func TestUnwrapBinaryResponse(t *testing.T) {
	env, _ := json.Marshal(map[string]any{"_pp_binary": true, "content_type": "audio/wav", "encoding": "base64", "bytes": 4, "data": "d2F2ZQ=="})
	if got := unwrapBinaryResponse(env); string(got) != "wave" {
		t.Fatalf("unwrapBinaryResponse(envelope) = %q, want %q", got, "wave")
	}
	plain := []byte("not-an-envelope")
	if got := unwrapBinaryResponse(plain); string(got) != "not-an-envelope" {
		t.Fatalf("unwrapBinaryResponse(plain) = %q, want passthrough", got)
	}
}
