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
		"https://api.plane.so":                            "https://api.plane.so",
		"https://api.plane.so/":                           "https://api.plane.so",
		"https://plane.acme.com/":                         "https://plane.acme.com",
		"https://plane.acme.com/api/v1/workspaces/{slug}": "https://plane.acme.com",
		// A trailing "/api/" survives TrimRight as "…/api" and is stripped by
		// the HasSuffix("/api") arm, so the caller never doubles the prefix.
		"  https://plane.acme.com/api/  ": "https://plane.acme.com",
		"https://plane.acme.com/api":      "https://plane.acme.com",
	}
	for in, want := range cases {
		if got := normalizeHost(in); got != want {
			t.Errorf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestInitCmdDoesNotShadowWorkspaceFlag guards the regression where init
// registered a local --workspace flag whose name collided with the root's
// persistent --workspace/-w. Cobra's AddFlagSet skips a persistent flag when a
// local flag of the same name exists, which silently dropped the `-w`
// shorthand from `init`. init must therefore expose NO local --workspace flag.
func TestInitCmdDoesNotShadowWorkspaceFlag(t *testing.T) {
	cmd := newInitCmd(&rootFlags{})
	if f := cmd.Flags().Lookup("workspace"); f != nil {
		t.Fatalf("init must not register a local --workspace flag (shadows persistent -w); found %q", f.Name)
	}
}

func TestNormalizeSlugList(t *testing.T) {
	got := normalizeSlugList([]string{
		"app.plane.so/acme",                  // browser-URL paste
		"https://app.plane.so/acme/projects", // same slug, full URL → dedup
		"   ",                                // blank → dropped
		"https://h/api/v1/workspaces/bravo",  // API-base paste
	})
	want := []string{"acme", "bravo"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestChooseDefaultWorkspace guards against init writing an unreachable slug as
// default_workspace: a --default that never enrolled (failed its probe) must be
// dropped in favor of the first reachable slug, signalled by ok=false.
func TestChooseDefaultWorkspace(t *testing.T) {
	enrolled := []string{"acme", "bravo"}

	if got, ok := chooseDefaultWorkspace("", enrolled); got != "acme" || !ok {
		t.Fatalf("empty default: got %q/%v, want acme/true", got, ok)
	}
	if got, ok := chooseDefaultWorkspace("bravo", enrolled); got != "bravo" || !ok {
		t.Fatalf("enrolled default: got %q/%v, want bravo/true", got, ok)
	}
	// URL-form --default that resolves to an enrolled slug is honored.
	if got, ok := chooseDefaultWorkspace("https://app.plane.so/bravo/", enrolled); got != "bravo" || !ok {
		t.Fatalf("URL-form enrolled default: got %q/%v, want bravo/true", got, ok)
	}
	// A --default that never enrolled falls back to the first reachable slug.
	if got, ok := chooseDefaultWorkspace("ghost", enrolled); got != "acme" || ok {
		t.Fatalf("unenrolled default: got %q/%v, want acme/false", got, ok)
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
