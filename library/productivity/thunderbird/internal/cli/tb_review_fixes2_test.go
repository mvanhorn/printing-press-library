package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTBForcedReplaceKeepsOriginalOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(p, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := tbWriteData
	tbWriteData = func(f *os.File, data []byte) error { return errors.New("disk full") }
	defer func() { tbWriteData = prev }()

	if err := tbWriteNewFile(p, []byte("replacement"), true); err == nil {
		t.Fatal("want write error")
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "original" {
		t.Fatalf("original lost: %q, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %d entries", len(entries))
	}

	tbWriteData = prev
	if err := tbWriteNewFile(p, []byte("replacement"), true); err != nil {
		t.Fatalf("forced replace: %v", err)
	}
	if got, _ := os.ReadFile(p); string(got) != "replacement" {
		t.Fatalf("want replacement, got %q", got)
	}
}

func TestTBForcedReplaceOverReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(p, []byte("original"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := tbWriteNewFile(p, []byte("replacement"), true); err != nil {
		t.Fatalf("forced replace of read-only file: %v", err)
	}
	if got, _ := os.ReadFile(p); string(got) != "replacement" {
		t.Fatalf("want replacement, got %q", got)
	}
}

func TestTBForcedReplaceRestoresModeWhenRetryFails(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(p, []byte("original"), 0o400); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Lstat(p)
	prev := tbRename
	tbRename = func(string, string) error { return errors.New("sharing violation") }
	defer func() { tbRename = prev }()
	if err := tbWriteNewFile(p, []byte("replacement"), true); err == nil {
		t.Fatal("want error when retry fails")
	}
	after, _ := os.Lstat(p)
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("mode changed: %v -> %v", before.Mode().Perm(), after.Mode().Perm())
	}
	if got, _ := os.ReadFile(p); string(got) != "original" {
		t.Fatalf("original lost: %q", got)
	}
}

func TestTBStorePathSameThroughSymlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "profile-link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	data := t.TempDir()
	if a, b := tbStoreDBPath(data, real), tbStoreDBPath(data, link); a != b {
		t.Fatalf("symlinked profile maps to a different store: %s vs %s", a, b)
	}
}
