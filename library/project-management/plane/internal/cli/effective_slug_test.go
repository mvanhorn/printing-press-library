package cli

import (
	"path/filepath"
	"testing"
)

// effectiveSlug is the precedence resolver the novel commands (relations, module)
// share. It must rank --workspace (global) > local --slug/positional > $PLANE_SLUG
// > default_workspace. The earlier resolveSlug() stopped at $PLANE_SLUG and never
// saw --workspace, so applyClientSlug() clobbered the slug newClient() had already
// resolved from --workspace and the CLI issued cross-workspace 403s
// (plane-pp-cli feedback, 2026-06-11).
func TestEffectiveSlugPrecedence(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), "no-such-config.toml")

	tests := []struct {
		name      string
		workspace string
		localSlug string
		planeSlug string // "" means unset
		want      string
	}{
		{"workspace beats local and env", "wsflag", "localslug", "envslug", "wsflag"},
		{"workspace beats env only", "wsflag", "", "envslug", "wsflag"},
		{"local beats env", "", "localslug", "envslug", "localslug"},
		{"env when no flag", "", "", "envslug", "envslug"},
		{"sentinel when nothing set", "", "", "", "my-workspace"},
		// --workspace and PLANE_SLUG tolerate a pasted browser URL / API base,
		// resolving to the bare slug (same tolerance as config.Load).
		{"workspace URL form normalized", "https://app.plane.so/acme/projects", "", "", "acme"},
		{"env URL form normalized", "", "", "app.plane.so/bravo", "bravo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.planeSlug == "" {
				t.Setenv("PLANE_SLUG", "")
			} else {
				t.Setenv("PLANE_SLUG", tt.planeSlug)
			}
			f := &rootFlags{workspace: tt.workspace, configPath: missingCfg}
			if got := effectiveSlug(f, tt.localSlug); got != tt.want {
				t.Fatalf("effectiveSlug(workspace=%q, local=%q, PLANE_SLUG=%q) = %q, want %q",
					tt.workspace, tt.localSlug, tt.planeSlug, got, tt.want)
			}
		})
	}
}

// Regression for the relations/module workspace bug: the novel commands resolve
// the slug with effectiveSlug() and then call applyClientSlug() on the client
// newClient() built. With --workspace set, that final write must keep the
// flag's workspace — not silently fall back to $PLANE_SLUG — otherwise the
// request targets the wrong workspace (403).
func TestEffectiveSlugDoesNotClobberWorkspaceFlag(t *testing.T) {
	t.Setenv("PLANE_SLUG", "doctor-school")
	t.Setenv("PLANE_BASE_URL", "https://example.test/api/v1/workspaces/{slug}")

	f := &rootFlags{workspace: "bbm", dryRun: true}
	c, err := f.newClient()
	if err != nil {
		t.Fatal(err)
	}
	// Mirror the command flow: a local --slug left unset must not pull the slug
	// back to $PLANE_SLUG.
	applyClientSlug(c, effectiveSlug(f, ""))
	if got := c.Config.TemplateVars["slug"]; got != "bbm" {
		t.Fatalf("after applyClientSlug(effectiveSlug): slug = %q, want bbm (PLANE_SLUG=doctor-school must not win over --workspace)", got)
	}
}
