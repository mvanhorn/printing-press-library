package config

import (
	"os"
	"path/filepath"
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
