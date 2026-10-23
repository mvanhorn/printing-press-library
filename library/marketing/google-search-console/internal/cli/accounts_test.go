package cli

import (
	"bytes"
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"GOOGLE_SEARCH_CONSOLE_CONFIG", "GOOGLE_SEARCH_CONSOLE_ACCOUNT", "GSC_ACCESS_TOKEN"} {
		t.Setenv(key, "")
	}
	t.Setenv("GOOGLE_SEARCH_CONSOLE_CLIENT_SECRET", "secret-env-value")
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

func TestCLIStoreAccountSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, k := range []string{"GOOGLE_SEARCH_CONSOLE_ACCOUNT", "GOOGLE_SEARCH_CONSOLE_CONFIG", "PRINTING_PRESS_STORE_DB", "GSC_ACCESS_TOKEN"} {
		t.Setenv(k, "")
	}
	for _, name := range []string{"bestself", "littlemight"} {
		s, err := store.Open(config.WithAccount(context.Background(), name), "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().Exec("CREATE TABLE account_fixture (name TEXT)"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().Exec("INSERT INTO account_fixture VALUES (?)", name); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"bestself", "littlemight", "bestself"} {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"sql", "SELECT name FROM account_fixture", "--account", name, "--json"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"name": "`+name+`"`) {
			t.Fatal("SQL used another account store")
		}
	}
}
