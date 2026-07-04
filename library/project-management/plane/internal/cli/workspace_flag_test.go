package cli

import "testing"

// The --workspace flag must override PLANE_SLUG/default_workspace by writing
// the client's TemplateVars["slug"]. We assert at the client level the dry-run
// URL targets the flag's workspace.
func TestWorkspaceFlagOverridesSlug(t *testing.T) {
	t.Setenv("PLANE_SLUG", "envslug")
	t.Setenv("PLANE_BASE_URL", "https://example.test/api/v1/workspaces/{slug}")
	f := &rootFlags{workspace: "flagslug", dryRun: true}
	c, err := f.newClient()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Config.TemplateVars["slug"]; got != "flagslug" {
		t.Fatalf("--workspace override: got %q want flagslug", got)
	}
}
