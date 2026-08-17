// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(mcp-workspace): per-call workspace targeting for the MCP execute path.
//
// Plane's REST API is workspace-scoped (…/api/v1/workspaces/<slug>/…). The CLI
// resolves the active slug with the precedence --workspace > $PLANE_SLUG >
// default_workspace, but the MCP server had no equivalent to the --workspace
// flag: newMCPClient() bakes the slug from $PLANE_SLUG / default_workspace at
// config-load time, so an agent could not retarget a single call. With a
// process-wide PLANE_SLUG set in the environment, every MCP request silently
// hit that one workspace regardless of the agent's intent — surfacing as a
// confusing HTTP 403 against the workspace the agent actually wanted.
//
// applyWorkspaceArg restores the missing flag: a top-level "workspace" argument
// overrides the client's {slug} template var for that one call, matching the
// CLI's --workspace precedence.

package mcp

import (
	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/config"
)

// workspaceArgKey is the public MCP argument name that targets a workspace for
// a single execute call.
const workspaceArgKey = "workspace"

// applyWorkspaceArg honors a per-call "workspace" argument by overriding the
// client's {slug} template var, mirroring the CLI's --workspace flag (top
// precedence over $PLANE_SLUG and config default_workspace). The argument is
// removed from args so it never leaks into the query string or request body of
// the underlying endpoint. A blank or absent value is a no-op, leaving the
// config-resolved slug in place.
func applyWorkspaceArg(c *client.Client, args map[string]any) {
	if args == nil {
		return
	}
	raw, ok := args[workspaceArgKey]
	if !ok {
		return
	}
	delete(args, workspaceArgKey)
	ws, ok := raw.(string)
	if !ok {
		return
	}
	ws = config.NormalizeWorkspaceSlug(ws)
	if ws == "" || c == nil || c.Config == nil {
		return
	}
	if c.Config.TemplateVars == nil {
		c.Config.TemplateVars = map[string]string{}
	}
	c.Config.TemplateVars["slug"] = ws
}
