package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadSlugPrecedence(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	t.Setenv("PLANE_BASE_URL", "")
	p := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\ndefault_workspace = \"acme\"\n")

	// default_workspace wins when PLANE_SLUG unset
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TemplateVars["slug"]; got != "acme" {
		t.Fatalf("slug from default_workspace: got %q want acme", got)
	}

	// sentinel branch: no default_workspace and PLANE_SLUG unset → "my-workspace"
	p2 := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\n")
	cfg2, err := Load(p2)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg2.TemplateVars["slug"]; got != "my-workspace" {
		t.Fatalf("sentinel slug: got %q want my-workspace", got)
	}

	// PLANE_SLUG env overrides default_workspace
	t.Setenv("PLANE_SLUG", "bravo")
	cfg, err = Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TemplateVars["slug"]; got != "bravo" {
		t.Fatalf("slug from env: got %q want bravo", got)
	}
}

func TestLoadDefaultWorkspaceNormalized(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	t.Setenv("PLANE_BASE_URL", "")
	p := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\ndefault_workspace = \"https://acme/\"\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TemplateVars["slug"]; got != "acme" {
		t.Fatalf("default_workspace should be normalized: got %q want acme", got)
	}

	// A full browser URL pasted from the address bar resolves to the slug, not
	// the host+path (the domain-style normalizer would have yielded
	// "app.plane.so/acme/settings").
	for _, raw := range []string{
		"https://app.plane.so/acme/settings/",
		"https://plane.self.host/api/v1/workspaces/acme",
		"acme/",
	} {
		p := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\ndefault_workspace = \""+raw+"\"\n")
		cfg, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.TemplateVars["slug"]; got != "acme" {
			t.Fatalf("default_workspace %q should normalize to acme: got %q", raw, got)
		}
	}
}

func TestWorkspaceEntryJSONTags(t *testing.T) {
	b, err := json.Marshal(WorkspaceEntry{Slug: "acme", ID: "uuid-1"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"slug":"acme"`) || !strings.Contains(got, `"id":"uuid-1"`) {
		t.Fatalf("WorkspaceEntry JSON should use snake_case keys, got %s", got)
	}
	// omitempty: empty ID must be dropped
	b2, _ := json.Marshal(WorkspaceEntry{Slug: "bravo"})
	if strings.Contains(string(b2), `"id"`) {
		t.Fatalf("empty ID should be omitted, got %s", string(b2))
	}
}

func TestSaveWorkspaceRegistryRoundTrip(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	p := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultWorkspace = "acme"
	cfg.Workspaces = []WorkspaceEntry{{Slug: "acme", ID: "uuid-1"}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DefaultWorkspace != "acme" || len(reloaded.Workspaces) != 1 || reloaded.Workspaces[0].ID != "uuid-1" {
		t.Fatalf("round trip lost data: %+v", reloaded)
	}
}
