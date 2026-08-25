// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel wiring. Registers the preserved novel surfaces through
// the generated registerNovelCommand / registerClientHook seams so a future
// regen keeps this file and its wiring without edits to root.go.

package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/config"

	"github.com/spf13/cobra"
)

// resolveActiveWorkspaceID maps the active slug to the enrolled workspace's
// cached UUID via the local [[workspaces]] registry. Empty result means
// "unknown tenant": flat reconcile skips rather than guessing a scope.
func resolveActiveWorkspaceID(slug string, workspaces []config.WorkspaceEntry) string {
	slug = strings.TrimSpace(slug)
	if slug == "" || slug == "my-workspace" {
		return ""
	}
	for _, w := range workspaces {
		if w.Slug == slug {
			return w.ID
		}
	}
	return ""
}

// novelWorkspace is the top of the slug precedence chain: it overrides
// PLANE_SLUG and default_workspace (both resolved by config.Load into
// TemplateVars["slug"]) for a single invocation. The MCP server has the same
// per-call override via applyWorkspaceArg.
var novelWorkspace string

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		rootCmd.PersistentFlags().StringVarP(&novelWorkspace, "workspace", "w", "", "Workspace slug to target (overrides PLANE_SLUG and default_workspace)")

		rootCmd.AddCommand(newAttachFileCmd(flags))
		rootCmd.AddCommand(newInitCmd(flags))
		rootCmd.AddCommand(newWorkspacesCmd(flags))

		// Plane's issue serializer never returns module membership, so module
		// enrichment must run at the tail of sync. Re-wrap the generated sync
		// command instead of editing its registration in root.go.
		for _, c := range rootCmd.Commands() {
			if c.Name() == "sync" {
				rootCmd.RemoveCommand(c)
				rootCmd.AddCommand(withModuleEnrichment(c, flags))
				break
			}
		}
	})

	registerClientHook(func(c *client.Client) error {
		if novelWorkspace == "" {
			return nil
		}
		if c.Config.TemplateVars == nil {
			c.Config.TemplateVars = map[string]string{}
		}
		// Normalize like every other slug source (PLANE_SLUG / default_workspace
		// via config.Load, the MCP workspace arg via applyWorkspaceArg) so a
		// pasted browser URL or API base (app.plane.so/acme, .../workspaces/acme)
		// resolves to the bare slug instead of a malformed {slug} path → 404/403.
		c.Config.TemplateVars["slug"] = config.NormalizeWorkspaceSlug(novelWorkspace)
		return nil
	})

	// PATCH(tenant-resolver-seam): supply the generated no-arg resolver seam
	// with the active workspace UUID from the local [[workspaces]] registry.
	// Dependent fan-out scopes to it; flat reconcile skips when "" (unknown
	// tenant). Set at client construction so both the flat and dependent sync
	// phases observe it. See .printing-press-patches/tenant-resolver-seam.json.
	registerClientHook(func(c *client.Client) error {
		if c.Config == nil {
			return nil
		}
		slug := c.Config.TemplateVars["slug"]
		ws := c.Config.Workspaces
		resolveTenantID = func() string { return resolveActiveWorkspaceID(slug, ws) }
		return nil
	})
}
