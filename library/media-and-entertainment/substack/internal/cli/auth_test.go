// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCookieToolSupportsStandalonePycookiecheatProfiles(t *testing.T) {
	t.Parallel()
	if !cookieToolSupportsProfiles("pycookiecheat-cli") {
		t.Fatal("standalone pycookiecheat executable must support Chrome profile selection")
	}
}

func TestExtractViaPycookiecheatCLIUsesProfileCookieFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell script fake")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir, err := chromeDataDir()
	if err != nil {
		t.Fatal(err)
	}
	profileDir := "Profile 1"
	cookiePath := filepath.Join(dataDir, profileDir, "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookiePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookiePath, []byte("sqlite placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}

	argsPath := filepath.Join(t.TempDir(), "args.txt")
	fake := filepath.Join(t.TempDir(), "pycookiecheat")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"printf '{\"substack.sid\":\"sid-value\",\"connect.sid\":\"creator-value\"}'\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := extractViaPycookiecheatCLI(cookieTool{name: "pycookiecheat-cli", pyBin: fake}, ".substack.com", profileDir)
	if err != nil {
		t.Fatal(err)
	}
	cookies := parseCookieString(got)
	if cookies["substack.sid"] != "sid-value" || cookies["connect.sid"] != "creator-value" {
		t.Fatalf("extracted cookies = %q, want session and creator cookies", got)
	}

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	wantArgs := []string{"-c", cookiePath, "https://substack.com"}
	if strings.Join(args, "\n") != strings.Join(wantArgs, "\n") {
		t.Fatalf("pycookiecheat args = %#v, want %#v", args, wantArgs)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
