// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/cliutil/testenv"
)

// PATCH(set-token-positional-compat): scripts written against earlier releases
// pass the token as an argument; it must keep working, with a nudge to stdin.
func TestSetTokenAcceptsLegacyPositionalToken(t *testing.T) {
	if restore, err := cliutil.SetHomeOverride(""); err == nil {
		t.Cleanup(restore)
	} else {
		t.Fatalf("reset home override: %v", err)
	}
	home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	t.Setenv("HOSTEX_ACCESS_TOKEN", "")

	const fakeToken = "test-token-not-a-real-credential"
	cmd := newAuthSetTokenCmd(&rootFlags{configPath: filepath.Join(home, ".config", "hostex-pp-cli", "config.toml")})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{fakeToken})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token with a positional token failed: %v", err)
	}
	if !strings.Contains(stderr.String(), "pipe it on stdin") {
		t.Errorf("stderr should recommend stdin, got %q", stderr.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), fakeToken) {
		t.Errorf("the token must never be echoed")
	}
	path, err := cliutil.CredentialsFilePath()
	if err != nil || !strings.HasPrefix(path, home) {
		t.Fatalf("credentials path %q (err %v) escaped the isolated home %q", path, err, home)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), fakeToken) {
		t.Fatalf("token was not saved to the isolated credentials file: %v", err)
	}
}

func TestSetTokenStillReadsStdin(t *testing.T) {
	if restore, err := cliutil.SetHomeOverride(""); err == nil {
		t.Cleanup(restore)
	} else {
		t.Fatalf("reset home override: %v", err)
	}
	home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	t.Setenv("HOSTEX_ACCESS_TOKEN", "")

	cmd := newAuthSetTokenCmd(&rootFlags{configPath: filepath.Join(home, ".config", "hostex-pp-cli", "config.toml")})
	var stderr bytes.Buffer
	cmd.SetIn(strings.NewReader("stdin-test-token\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token from stdin failed: %v", err)
	}
	if strings.Contains(stderr.String(), "warning") {
		t.Errorf("stdin form should not warn, got %q", stderr.String())
	}
}
