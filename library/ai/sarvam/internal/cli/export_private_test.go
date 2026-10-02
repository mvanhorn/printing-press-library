// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExportOutputIsPrivateAndReplacedOnlyOnSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions and symlink replacement are not portable to Windows")
	}

	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/text-to-speech/pronunciation-dictionary" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if fail.Load() {
			http.Error(w, "synthetic failure", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"dictionaries":[{"dictionary_id":"test-dictionary","name":"fixture"}]}`))
	}))
	defer server.Close()
	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	configPath := filepath.Join(t.TempDir(), "missing.toml")
	runExport := func(output string) error {
		cmd := RootCmd()
		cmd.SetArgs([]string{"export", "text-to-speech", "--output", output, "--no-cache", "--config", configPath})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		return cmd.Execute()
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "dictionaries.jsonl")
	if err := os.WriteFile(output, []byte("old data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runExport(output); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("export mode = %o, want 600", got)
	}
	data, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(data), "test-dictionary") {
		t.Fatalf("successful export missing: err=%v", err)
	}

	fail.Store(true)
	if err := runExport(output); err == nil {
		t.Fatal("failed API request unexpectedly succeeded")
	}
	unchanged, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(unchanged, data) {
		t.Fatalf("failed export changed existing file: err=%v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".sarvam-private-*")); err != nil || len(matches) != 0 {
		t.Fatalf("failed export left temporary files: count=%d err=%v", len(matches), err)
	}

	fail.Store(false)
	target := filepath.Join(dir, "target.jsonl")
	link := filepath.Join(dir, "link.jsonl")
	if err := os.WriteFile(target, []byte("keep target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := runExport(link); err != nil {
		t.Fatal(err)
	}
	targetData, err := os.ReadFile(target)
	if err != nil || string(targetData) != "keep target" {
		t.Fatalf("export followed destination symlink: err=%v", err)
	}
	if info, err := os.Lstat(link); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("export did not replace symlink with regular file: err=%v", err)
	}
}
