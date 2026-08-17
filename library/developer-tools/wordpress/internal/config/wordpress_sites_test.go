package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWordPressSiteRegistryPreservesConfigAndSecretsStayPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := []byte("base_url = \"https://legacy.example/wp-json\"\ncustom_key = \"keep-me\"\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadWordPressSites(path)
	if err != nil {
		t.Fatal(err)
	}
	registry.Active = "example-com"
	registry.Sites["example-com"] = WordPressSite{
		Name: "example-com", BaseURL: "https://example.com/wp-json",
		SiteURL: "https://example.com", AppPassword: "never-print-this",
		Namespaces: make([]string, 0), AddedAt: time.Unix(1, 0).UTC(),
	}
	if err := SaveWordPressSites(registry); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `custom_key = 'keep-me'`) && !strings.Contains(text, `custom_key = "keep-me"`) {
		t.Fatalf("unrelated config key was not preserved:\n%s", text)
	}
	if !strings.Contains(text, "[sites.example-com]") {
		t.Fatalf("site table missing:\n%s", text)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}

	encoded, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "never-print-this") {
		t.Fatalf("JSON leaked app password: %s", encoded)
	}
}
