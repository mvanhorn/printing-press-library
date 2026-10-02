// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelDownloadHelpWires smoke-tests that the download command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDownloadHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"download", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("download --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "download"} {
		if !strings.Contains(help, want) {
			t.Fatalf("download --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLogitechDownloadRedirectBoundary(t *testing.T) {
	t.Parallel()

	client := newLogitechDownloadClient()
	for _, rawURL := range []string{
		"https://download.logi.com/manual.pdf",
		"https://download01.logi.com/firmware.bin",
	} {
		u, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("parse %q: %v", rawURL, err)
		}
		if err := client.CheckRedirect(&http.Request{URL: u}, nil); err != nil {
			t.Errorf("allowed redirect %q rejected: %v", rawURL, err)
		}
	}

	for _, rawURL := range []string{
		"http://download01.logi.com/firmware.bin",
		"https://download01.logi.com.evil.example/firmware.bin",
		"https://127.0.0.1/firmware.bin",
		"http://169.254.169.254/latest/meta-data/",
	} {
		u, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("parse %q: %v", rawURL, err)
		}
		if err := client.CheckRedirect(&http.Request{URL: u}, nil); err == nil {
			t.Errorf("unsafe redirect %q was accepted", rawURL)
		}
	}
}

func TestSaveNewDownloadRejectsExistingTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "manual.pdf")
	if err := os.WriteFile(existing, []byte("keep-existing"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	if err := saveNewDownload(existing, strings.NewReader("new-download")); err == nil {
		t.Fatal("publish unexpectedly replaced an existing file")
	}
	if got, err := os.ReadFile(existing); err != nil || string(got) != "keep-existing" {
		t.Fatalf("existing file changed: contents=%q err=%v", got, err)
	}

	target := filepath.Join(dir, "outside-target.bin")
	if err := os.WriteFile(target, []byte("keep-target"), 0o600); err != nil {
		t.Fatalf("seed symlink target: %v", err)
	}
	symlink := filepath.Join(dir, "firmware.bin")
	if err := os.Symlink(target, symlink); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}
	if err := saveNewDownload(symlink, strings.NewReader("new-download")); err == nil {
		t.Fatal("publish unexpectedly followed an existing symlink")
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "keep-target" {
		t.Fatalf("symlink target changed: contents=%q err=%v", got, err)
	}
}

type replacementDuringRead struct {
	dest  string
	wrote bool
}

func (r *replacementDuringRead) Read(p []byte) (int, error) {
	if r.wrote {
		return 0, errors.New("unexpected second read")
	}
	r.wrote = true
	replacement := r.dest + ".replacement"
	if err := os.WriteFile(replacement, []byte("replacement"), 0o600); err != nil {
		return 0, err
	}
	if err := os.Rename(replacement, r.dest); err != nil {
		return 0, err
	}
	return 0, errors.New("synthetic copy failure")
}

func TestSaveNewDownloadPreservesReplacementOnCopyError(t *testing.T) {
	t.Parallel()
	dest := filepath.Join(t.TempDir(), "manual.pdf")
	if err := saveNewDownload(dest, &replacementDuringRead{dest: dest}); err == nil || !strings.Contains(err.Error(), "partial download may remain") {
		t.Fatalf("download replaced an intervening file: %v", err)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "replacement" {
		t.Fatalf("intervening file changed: contents=%q err=%v", got, err)
	}
}

type failingDownloadReader struct{}

func (failingDownloadReader) Read([]byte) (int, error) {
	return 0, errors.New("synthetic read failure")
}

func TestSaveNewDownloadReportsPartialFileOnCopyFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dest := filepath.Join(dir, "manual.pdf")
	if err := saveNewDownload(dest, failingDownloadReader{}); err == nil || !strings.Contains(err.Error(), "partial download may remain") {
		t.Fatalf("copy failure or partial-file warning was hidden: %v", err)
	}
	if info, err := os.Lstat(dest); err != nil || info.Size() != 0 {
		t.Fatalf("partial download was removed or changed: info=%v err=%v", info, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed download left unexpected files: entries=%v err=%v", entries, err)
	}
}
