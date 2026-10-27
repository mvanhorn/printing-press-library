// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
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
