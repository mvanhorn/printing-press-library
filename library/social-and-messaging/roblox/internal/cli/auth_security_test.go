// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCopyFileIfExistsCreatesPrivateDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	dst := filepath.Join(dir, "copy")
	if err := os.WriteFile(src, []byte("cookie database contents"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}
	if err := os.WriteFile(dst, []byte("stale contents"), 0o644); err != nil {
		t.Fatalf("writing pre-existing destination: %v", err)
	}

	if err := copyFileIfExists(src, dst); err != nil {
		t.Fatalf("copyFileIfExists() error = %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat destination: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("destination permissions = %04o, want 0600", got)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading destination: %v", err)
	}
	if string(data) != "cookie database contents" {
		t.Fatalf("destination contents = %q", data)
	}
}

func TestPycookiecheatScriptQuotesUntrustedPath(t *testing.T) {
	path := filepath.Join("Profile \"; __import__('os').system('false'); #", "Cookies")
	script := pycookiecheatScript(`.roblox.com\"); injected = True; #`, path)

	pathLiteral := strconv.Quote(filepath.ToSlash(path))
	domainLiteral := strconv.Quote(`https://roblox.com\"); injected = True; #`)
	if !strings.Contains(script, "cookie_file="+pathLiteral) {
		t.Fatalf("script does not contain safely quoted cookie path: %s", script)
	}
	if !strings.Contains(script, "chrome_cookies("+domainLiteral) {
		t.Fatalf("script does not contain safely quoted domain: %s", script)
	}
	want := `import json; from pycookiecheat import chrome_cookies; print(json.dumps(chrome_cookies(` + domainLiteral + `, cookie_file=` + pathLiteral + `)))`
	if script != want {
		t.Fatalf("script = %q, want safely quoted source %q", script, want)
	}
}
