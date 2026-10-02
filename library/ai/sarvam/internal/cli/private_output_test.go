// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWritePrivateOutputFileTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}

	path := filepath.Join(t.TempDir(), "transcript.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateOutputFile(path, []byte("private")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "private" {
		t.Fatalf("content = %q, want private", got)
	}
}

func TestPrivateOutputPathRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	got, err := privateOutputPath(dir, "speaker.wav")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "speaker.wav"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	for _, name := range []string{"", ".", "..", "../secret", "nested/file.wav", `nested\file.wav`, filepath.Join(dir, "absolute.wav")} {
		if _, err := privateOutputPath(dir, name); err == nil {
			t.Errorf("privateOutputPath(%q) unexpectedly succeeded", name)
		}
	}
}

func TestWritePrivateOutputFileDoesNotFollowSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "result.txt")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateOutputFile(link, []byte("private")); err != nil {
		t.Fatal(err)
	}
	gotTarget, err := os.ReadFile(target)
	if err != nil || string(gotTarget) != "original" {
		t.Fatalf("symlink target changed: data=%q err=%v", gotTarget, err)
	}
	gotLink, err := os.ReadFile(link)
	if err != nil || string(gotLink) != "private" {
		t.Fatalf("result file missing: data=%q err=%v", gotLink, err)
	}
}
