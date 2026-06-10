package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/config"
)

func TestResolveWorkspaceSource(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	cfg := &config.Config{DefaultWorkspace: "acme", TemplateVars: map[string]string{"slug": "acme"}}

	slug, src := resolveWorkspace(&rootFlags{}, cfg)
	if slug != "acme" || src != "config:default_workspace" {
		t.Fatalf("config source: got %q/%q", slug, src)
	}
	slug, src = resolveWorkspace(&rootFlags{workspace: "flag"}, cfg)
	if slug != "flag" || src != "flag:--workspace" {
		t.Fatalf("flag source: got %q/%q", slug, src)
	}
	t.Setenv("PLANE_SLUG", "envv")
	slug, src = resolveWorkspace(&rootFlags{}, cfg)
	if slug != "envv" || src != "env:PLANE_SLUG" {
		t.Fatalf("env source: got %q/%q", slug, src)
	}
}

func TestUpsertWorkspace(t *testing.T) {
	cfg := &config.Config{}
	upsertWorkspace(cfg, "acme", "id1")
	upsertWorkspace(cfg, "acme", "id2") // update id, no dup
	upsertWorkspace(cfg, "bravo", "")
	if len(cfg.Workspaces) != 2 {
		t.Fatalf("want 2 entries got %d", len(cfg.Workspaces))
	}
	if cfg.Workspaces[0].ID != "id2" {
		t.Fatalf("upsert should update id, got %q", cfg.Workspaces[0].ID)
	}
}

func TestBaseURLHasLiteralSlug(t *testing.T) {
	if !baseURLHasLiteralSlug("https://h/api/v1/workspaces/acme") {
		t.Fatal("literal slug not detected")
	}
	if baseURLHasLiteralSlug("https://h/api/v1/workspaces/{slug}") {
		t.Fatal("templated base wrongly flagged")
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"https://api.plane.so":                 "https://api.plane.so",
		"https://api.plane.so/":                "https://api.plane.so",
		"https://plane.acme.com/":              "https://plane.acme.com",
		"https://plane.acme.com/api/v1/workspaces/{slug}": "https://plane.acme.com",
		// TrimRight strips the trailing slash before the /api/ check, so
		// "  https://plane.acme.com/api/  " → TrimSpace → TrimRight("/") →
		// "https://plane.acme.com/api" (no trailing slash, so /api/ not found).
		"  https://plane.acme.com/api/  ": "https://plane.acme.com/api",
	}
	for in, want := range cases {
		if got := normalizeHost(in); got != want {
			t.Errorf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWorkspacesListBanner(t *testing.T) {
	var b strings.Builder
	cfg := &config.Config{
		DefaultWorkspace: "acme",
		Workspaces:       []config.WorkspaceEntry{{Slug: "acme", ID: "id1"}, {Slug: "bravo"}},
	}
	renderWorkspaceList(&b, cfg, "acme")
	out := b.String()
	if !strings.Contains(out, "acme") || !strings.Contains(out, "bravo") {
		t.Fatalf("list missing entries: %q", out)
	}
	if !strings.Contains(out, "cannot enumerate") {
		t.Fatalf("list missing API-limitation banner: %q", out)
	}
}
