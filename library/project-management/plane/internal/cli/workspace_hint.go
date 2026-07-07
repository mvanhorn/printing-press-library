// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(workspace-error-hint): make a "no workspace selected" 403/404 say so.
//
// Plane's REST API is workspace-scoped (…/api/v1/workspaces/<slug>/…). When no
// workspace is selected (no --workspace, no $PLANE_SLUG, no default_workspace),
// the {slug} template var falls to the "my-workspace" sentinel and the API
// rejects the request — typically a 403 ("permission denied") on collection
// endpoints, sometimes a 404. The generic 403 hint then misdirects the user
// toward their API key, when the real cause is simply an unselected workspace.
//
// noWorkspaceSelected detects that state from the effective slug so
// classifyAPIError can lead with the actionable workspace hint instead.

package cli

import "fmt"

// workspaceSentinelSlug is the placeholder {slug} value config.Load falls back
// to when nothing selects a workspace; a request carrying it never reaches a
// real workspace.
const workspaceSentinelSlug = "my-workspace"

// noWorkspaceSelected reports whether the effective workspace slug is unset or
// the sentinel — i.e. the caller never named a workspace, so a 403/404 is far
// more likely a missing --workspace than an API-key or access problem.
func noWorkspaceSelected(flags *rootFlags) bool {
	if flags == nil {
		return false
	}
	slug := effectiveSlug(flags, "")
	return slug == "" || slug == workspaceSentinelSlug
}

// workspaceHintErr wraps a 403/404 that was issued without a selected workspace
// in a usage error (exit 2) whose hint points at workspace selection rather than
// the API key.
func workspaceHintErr(err error) error {
	return usageErr(fmt.Errorf("%w\nhint: no workspace selected — this is almost certainly a missing workspace, not an API-key problem."+
		"\n      Plane's API is workspace-scoped; with no --workspace / $PLANE_SLUG / default_workspace the request hits the '"+workspaceSentinelSlug+"' sentinel and is rejected."+
		"\n      Pass --workspace <slug> (top precedence), or run 'plane-pp-cli workspaces use <slug>' to set a default."+
		"\n      Run 'plane-pp-cli workspaces list' to see enrolled slugs, or take one from the browser URL app.plane.so/<slug>/.", err))
}
