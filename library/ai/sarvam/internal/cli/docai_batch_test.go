// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDocaiBatchDocumentsSkipsSymlinkOutsideFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on Windows")
	}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(inside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked.pdf")); err != nil {
		t.Fatal(err)
	}
	docs, err := docaiBatchDocuments(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0] != inside {
		t.Fatalf("documents=%v, want only %q", docs, inside)
	}
}

func TestOpenDocaiBatchDocumentRejectsSwappedSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on Windows")
	}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(path, []byte("document"), 0o600); err != nil {
		t.Fatal(err)
	}
	listed, err := docaiBatchDocuments(dir)
	if err != nil || len(listed) != 1 {
		t.Fatalf("enumerating original document: paths=%v err=%v", listed, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if file, err := openDocaiBatchDocument(listed[0]); err == nil {
		_ = file.Close()
		t.Fatal("swapped symlink was opened for upload")
	}
}

// TestDocaiBatchFailureReason locks down that any status other than a
// genuine success ("completed"/"partially_completed") is treated as a
// failure, not a silent success with no saved result — whether the poll
// loop stopped because of a rejected/failed job, exhausting all polls, or
// the command's own context deadline firing mid-poll (ctx.Done()).
func TestDocaiBatchFailureReason(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"completed", "completed", false},
		{"partially_completed", "partially_completed", false},
		{"failed status", "failed", true},
		{"rejected status", "rejected", true},
		{"poll exhausted", "processing", true},
		{"ctx deadline fired mid-poll", "processing", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := docaiBatchFailureReason(tc.status)
			if (got != "") != tc.wantErr {
				t.Fatalf("docaiBatchFailureReason(%q) = %q, want non-empty=%v", tc.status, got, tc.wantErr)
			}
		})
	}
}

func TestDocaiResultFileNamePreservesSourceExtension(t *testing.T) {
	if got, want := docaiResultFileName("/tmp/invoice.pdf"), "invoice.pdf.json"; got != want {
		t.Fatalf("docaiResultFileName() = %q, want %q", got, want)
	}
	if docaiResultFileName("invoice.pdf") == docaiResultFileName("invoice.png") {
		t.Fatal("distinct source extensions produced the same result filename")
	}
}

// TestNovelDocaiBatchHelpWires smoke-tests that the docai batch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDocaiBatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"docai", "batch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("docai batch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "batch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("docai batch --help missing %q in output:\n%s", want, help)
		}
	}
}
