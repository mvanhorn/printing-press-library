// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// PATCH(novel1219-auth-compat): Codex novel1219-r8 P2: logout reports a legacy env name that still supplies the token,
// exactly as it reports CONDUYT_CRM_BEARER_AUTH, in JSON and in prose.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

func TestAuthLogoutReportsLegacyEnvNames(t *testing.T) {
	const placeholder = "${user_config.conduyt_crm_bearer_auth}"
	for _, tc := range []struct {
		name string
		env  map[string]string // "" value = set to blank
		want string            // "" = no env var still supplies a token
	}{
		{"current", map[string]string{"CONDUYT_CRM_BEARER_AUTH": "CURRENT-ENV-SECRET-6666"}, "CONDUYT_CRM_BEARER_AUTH"},
		{"bearer auth", map[string]string{"CONDUYT_BEARER_AUTH": "LEGACY-ENV-SECRET-6666"}, "CONDUYT_BEARER_AUTH"},
		{"api key", map[string]string{"CONDUYT_API_KEY": "LEGACY-ENV-SECRET-6666"}, "CONDUYT_API_KEY"},
		{"current placeholder, bearer auth set", map[string]string{"CONDUYT_CRM_BEARER_AUTH": placeholder, "CONDUYT_BEARER_AUTH": "LEGACY-ENV-SECRET-6666"}, "CONDUYT_BEARER_AUTH"},
		{"current placeholder, api key set", map[string]string{"CONDUYT_CRM_BEARER_AUTH": placeholder, "CONDUYT_API_KEY": "LEGACY-ENV-SECRET-6666"}, "CONDUYT_API_KEY"},
		{"current placeholder only", map[string]string{"CONDUYT_CRM_BEARER_AUTH": placeholder}, ""},
		{"blank current blocks a legacy name", map[string]string{"CONDUYT_CRM_BEARER_AUTH": "", "CONDUYT_BEARER_AUTH": "LEGACY-ENV-SECRET-6666"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, asJSON := range []bool{true, false} {
				testenv.Isolate(t)
				for _, n := range []string{"CONDUYT_CRM_BEARER_AUTH", "CONDUYT_BEARER_AUTH", "CONDUYT_API_KEY"} {
					t.Setenv(n, "")
					_ = os.Unsetenv(n)
				}
				for n, v := range tc.env {
					t.Setenv(n, v)
				}
				args := []string{"auth", "logout"}
				if asJSON {
					args = append(args, "--json")
				}
				// Not runNovel: it exports CONDUYT_CRM_BEARER_AUTH, which would decide before the legacy names.
				cmd := RootCmd()
				cmd.SetArgs(args)
				var outBuf, errBuf bytes.Buffer
				cmd.SetOut(&outBuf)
				cmd.SetErr(&errBuf)
				err := cmd.Execute()
				out, stderr := outBuf.String(), errBuf.String()
				if err != nil {
					t.Fatalf("%v: %v", args, err)
				}
				if tc.want != "" && (!strings.Contains(out, tc.want+" env var is still set") || strings.Contains(out, "Logged out. Credentials cleared.")) {
					t.Fatalf("%v: output %q does not name %s as still set", args, out, tc.want)
				}
				if tc.want == "" && strings.Contains(out, "env var is still set") {
					t.Fatalf("%v: output %q names an env var although none supplies a token", args, out)
				}
				if strings.Contains(out+stderr, "ENV-SECRET-6666") {
					t.Fatalf("%v leaks the env value", args)
				}
			}
		})
	}
}
