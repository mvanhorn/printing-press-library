// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored unit tests for here.now novel-feature pure logic: file
// classification (publish dir), sha256 drive-diff classification (drives
// sync/diff), and expiry computation (claims).
package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// writeTestFile writes content to dir/rel, creating parent directories, and
// returns the absolute path.
func writeTestFile(t *testing.T, dir, rel string, content []byte) string {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return abs
}

func TestClassifyForPublishInlineVsUpload(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "index.html", []byte("<!doctype html><h1>hi</h1>"))
	writeTestFile(t, dir, "notes/readme.txt", []byte("hello world"))
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01}
	writeTestFile(t, dir, "img/logo.png", pngHeader)
	big := make([]byte, inlineTextMaxBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	writeTestFile(t, dir, "big.txt", big)

	plan, err := classifyForPublish(dir)
	if err != nil {
		t.Fatalf("classifyForPublish: %v", err)
	}

	// Every file is sent as a size-bearing descriptor with a sha256 hash.
	descriptors := map[string]publishFileReq{}
	for _, f := range plan.Files {
		descriptors[f.Path] = f
		if f.Hash == "" {
			t.Errorf("file %s missing sha256 hash", f.Path)
		}
		if f.Size <= 0 {
			t.Errorf("file %s has non-positive size %d", f.Path, f.Size)
		}
	}
	for _, p := range []string{"index.html", "notes/readme.txt", "img/logo.png", "big.txt"} {
		if _, ok := descriptors[p]; !ok {
			t.Errorf("expected %s in files[]; got %v", p, descriptors)
		}
	}

	// The informational small-text-vs-binary split (dry-run preview only).
	if plan.InlineEligible != 2 {
		t.Errorf("InlineEligible = %d, want 2 (index.html, readme.txt)", plan.InlineEligible)
	}
	if plan.BinaryOrLarge != 2 {
		t.Errorf("BinaryOrLarge = %d, want 2 (logo.png, big.txt)", plan.BinaryOrLarge)
	}

	wantTotal := int64(len("<!doctype html><h1>hi</h1>") + len("hello world") + len(pngHeader) + len(big))
	if plan.TotalSize != wantTotal {
		t.Errorf("TotalSize = %d, want %d", plan.TotalSize, wantTotal)
	}
}

func TestClassifyForPublishForwardSlashPaths(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a/b/c.txt", []byte("nested"))
	plan, err := classifyForPublish(dir)
	if err != nil {
		t.Fatalf("classifyForPublish: %v", err)
	}
	if len(plan.Files) != 1 || plan.Files[0].Path != "a/b/c.txt" {
		t.Fatalf("expected forward-slash path a/b/c.txt, got %+v", plan.Files)
	}
}

func TestClassifyForPublishEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := classifyForPublish(dir); err == nil {
		t.Fatal("expected error for empty directory, got nil")
	}
}

func TestComputeDriveDiffClassification(t *testing.T) {
	dir := t.TempDir()
	unchangedAbs := writeTestFile(t, dir, "unchanged.txt", []byte("same content"))
	writeTestFile(t, dir, "changed.txt", []byte("new local content"))
	writeTestFile(t, dir, "new.txt", []byte("brand new"))

	unchangedHash, err := sha256File(unchangedAbs)
	if err != nil {
		t.Fatalf("hash unchanged: %v", err)
	}

	remote := map[string]driveFileMeta{
		"unchanged.txt": {Path: "unchanged.txt", SHA256: unchangedHash, Size: 12},
		"changed.txt":   {Path: "changed.txt", SHA256: "deadbeef", Size: 5},
		"gone.txt":      {Path: "gone.txt", SHA256: "abc123", Size: 3},
	}

	diff, err := computeDriveDiff(dir, remote, true)
	if err != nil {
		t.Fatalf("computeDriveDiff: %v", err)
	}

	uploadSet := relSet(diff.Upload)
	for _, want := range []string{"changed.txt", "new.txt"} {
		if !uploadSet[want] {
			t.Errorf("expected %s in upload set; got %v", want, uploadSet)
		}
	}
	if uploadSet["unchanged.txt"] {
		t.Errorf("unchanged.txt should not be uploaded")
	}

	unchangedSet := relSet(diff.Unchanged)
	if !unchangedSet["unchanged.txt"] {
		t.Errorf("expected unchanged.txt in unchanged set; got %v", unchangedSet)
	}

	sort.Strings(diff.Delete)
	if len(diff.Delete) != 1 || diff.Delete[0] != "gone.txt" {
		t.Errorf("expected delete=[gone.txt], got %v", diff.Delete)
	}
}

func TestComputeDriveDiffNoDeletesWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "keep.txt", []byte("x"))
	remote := map[string]driveFileMeta{
		"gone.txt": {Path: "gone.txt", SHA256: "abc", Size: 1},
	}
	diff, err := computeDriveDiff(dir, remote, false)
	if err != nil {
		t.Fatalf("computeDriveDiff: %v", err)
	}
	if len(diff.Delete) != 0 {
		t.Errorf("expected no deletes when includeDeletes=false, got %v", diff.Delete)
	}
}

func TestComputeDriveDiffNormalizesSHAPrefix(t *testing.T) {
	dir := t.TempDir()
	abs := writeTestFile(t, dir, "f.bin", []byte("payload"))
	hash, err := sha256File(abs)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	remote := map[string]driveFileMeta{
		"f.bin": {Path: "f.bin", SHA256: "SHA256:" + hash, Size: 7},
	}
	diff, err := computeDriveDiff(dir, remote, false)
	if err != nil {
		t.Fatalf("computeDriveDiff: %v", err)
	}
	if len(diff.Upload) != 0 {
		t.Errorf("prefixed/identical hash should be unchanged, but got upload=%v", relSet(diff.Upload))
	}
	if len(diff.Unchanged) != 1 {
		t.Errorf("expected 1 unchanged, got %d", len(diff.Unchanged))
	}
}

func relSet(files []localFile) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		out[f.RelPath] = true
	}
	return out
}

func TestRemainingUntil(t *testing.T) {
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		expiresAt string
		want      string
	}{
		{"empty is permanent", "", "permanent"},
		{"unparseable is unknown", "not-a-time", "unknown"},
		{"past is expired", now.Add(-time.Hour).Format(time.RFC3339), "expired"},
		{"two hours out", now.Add(2 * time.Hour).Format(time.RFC3339), "2h0m0s"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := remainingUntil(tc.expiresAt, now)
			if got != tc.want {
				t.Errorf("remainingUntil(%q) = %q, want %q", tc.expiresAt, got, tc.want)
			}
		})
	}
}

func TestNewMetricOverLimit(t *testing.T) {
	m := newMetric(600, 500, "x")
	if !m.OverLimit {
		t.Error("expected OverLimit=true for used>limit")
	}
	if m.Pct != 120 {
		t.Errorf("Pct = %d, want 120", m.Pct)
	}
	m2 := newMetric(250, 500, "x")
	if m2.OverLimit {
		t.Error("expected OverLimit=false for used<limit")
	}
	if m2.Pct != 50 {
		t.Errorf("Pct = %d, want 50", m2.Pct)
	}
}
