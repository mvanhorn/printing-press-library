package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"CLOUD_RUN_ADMIN_CONFIG", "CLOUD_RUN_ADMIN_ACCOUNT", "CLOUD_RUN_ADMIN_OAUTH2C"} {
		t.Setenv(key, "")
	}
	t.Setenv("CLOUD_RUN_ADMIN_CLIENT_SECRET", "secret-env-value")
	run := func(args ...string) (string, error) {
		root := RootCmd()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs(args)
		err := root.Execute()
		out := buf.String()
		for _, secret := range []string{"secret-env-value", "access-hidden", "refresh-hidden"} {
			if strings.Contains(out, secret) {
				t.Fatal("credential leaked")
			}
		}
		return out, err
	}
	p := filepath.Join(t.TempDir(), "oauth.toml")
	if err := os.WriteFile(p, []byte("base_url=''\naccess_token='access-hidden'\nrefresh_token='refresh-hidden'"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bestself", "littlemight"} {
		if _, err := run("auth", "add", name, "--from-config", p); err != nil {
			t.Fatal(err)
		}
		out, err := run("--account", name, "auth", "status", "--json")
		if err != nil || !strings.Contains(out, `"authenticated": true`) || !strings.Contains(out, name) {
			t.Fatal(out, err)
		}
	}
	if _, err := run("auth", "set-default", "bestself"); err != nil {
		t.Fatal(err)
	}
	out, err := run("auth", "accounts")
	if err != nil || !strings.Contains(out, `"active_account": "bestself"`) {
		t.Fatal(out, err)
	}
	out, err = run("--account", "littlemight", "agent-context")
	if err != nil || !strings.Contains(out, `"account":"littlemight"`) {
		t.Fatal(err)
	}
	out, err = run("--account", "littlemight", "doctor", "--json")
	if err != nil || !strings.Contains(out, `"account": "littlemight"`) {
		t.Fatal(out, err)
	}
	if _, err := run("auth", "remove", "littlemight"); err == nil {
		t.Fatal("missing confirmation")
	}
	if _, err := run("auth", "remove", "littlemight", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := run("auth", "add", "bestself", "--from-config", p); err == nil {
		t.Fatal("overwrote account")
	}
}
