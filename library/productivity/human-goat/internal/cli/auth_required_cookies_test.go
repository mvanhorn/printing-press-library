// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TaskRabbit's API answers 401 to session + XSRF-TOKEN alone; tr6 carries the
// logged-in identity, so the login filter must keep it.
func TestAuthLoginRequiresTr6Cookie(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		wantErr string
	}{
		{"all three", "session=s; XSRF-TOKEN=x; tr6=t; _ga=junk", ""},
		{"missing tr6", "session=s; XSRF-TOKEN=x; _ga=junk", `cookie "tr6" not found`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			cookieFile := filepath.Join(t.TempDir(), "cookie.txt")
			if err := os.WriteFile(cookieFile, []byte(tc.header), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := RootCmd()
			cmd.SetArgs([]string{"--home", home, "auth", "login", "--cookies-file", cookieFile})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v\n%s", err, out.String())
				}
				if !strings.Contains(out.String(), "Found 3 cookies") {
					t.Fatalf("expected 3 kept cookies, got:\n%s", out.String())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// TaskRabbit sets its auth cookies host-only on www, so the Chrome lookup must
// ask the cookie reader for the www host. A fake `cookies` binary stands in for
// the reader and only returns the auth cookies for that exact URL.
func TestAuthLoginChromeReadsWWWHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script cookie reader")
	}
	bin := t.TempDir()
	argFile := filepath.Join(bin, "url")
	script := `#!/bin/sh
[ "$1" = "--help" ] && exit 0
printf '%s' "$1" > "` + argFile + `"
if [ "$1" = "https://www.taskrabbit.com" ]; then
  echo "Cookie: session=s; XSRF-TOKEN=x; tr6=t; _ga=junk"
else
  echo "Cookie: _ga=junk"
fi
`
	if err := os.WriteFile(filepath.Join(bin, "cookies"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	cmd := RootCmd()
	cmd.SetArgs([]string{"--home", t.TempDir(), "auth", "login", "--chrome"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth login --chrome: %v\n%s", err, out.String())
	}
	if got, _ := os.ReadFile(argFile); string(got) != "https://www.taskrabbit.com" {
		t.Fatalf("cookie reader asked for %q, want https://www.taskrabbit.com", got)
	}
	if !strings.Contains(out.String(), "Found 3 cookies") {
		t.Fatalf("expected the three auth cookies, got:\n%s", out.String())
	}
}
