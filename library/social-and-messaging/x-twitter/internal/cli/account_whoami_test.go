package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAccountWhoamiReportsCookieUserID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	dir := filepath.Join(home, ".config", "x-twitter-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir cookies dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cookies.json"), []byte(`{"auth_token":"a","ct0":"c","web_bearer":"w","twid":"u%3D98765"}`), 0o600); err != nil {
		t.Fatalf("write cookies: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"account", "whoami", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("account whoami failed: %v\n%s", err, out.String())
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	cookieLane := payload["x_articles_cookie"]
	if cookieLane["status"] != "ok" || cookieLane["user_id"] != "98765" {
		t.Fatalf("unexpected cookie lane: %#v", cookieLane)
	}
}
