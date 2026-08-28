// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// Removal of the OAuth callback endpoints from the command surface.
//
// Two paths in the Rightbrain spec exist solely as OAuth redirect targets:
//
//	/integration/callback
//	/task_mcp_server/callback
//
// The generator turns every spec path into a command, so these arrive as the
// top-level `integration` and `task-mcp-server` commands. Neither can ever
// succeed when a person runs it: they exist to receive a `code` and `state` that
// the OAuth provider mints and hands to Rightbrain's redirect URI, so any
// hand-supplied value is rejected. Running them returns exit 5, and the live
// dogfood matrix flags four separate failures against them.
//
// A command that cannot succeed by construction is worse than a missing one — it
// invites an agent to try, burns a round trip, and returns an opaque API error.
// They are removed here rather than by editing the generated files, so a future
// regeneration does not resurrect them.
//
// This removes ONLY the two top-level callback commands. The real integration and
// MCP-server surfaces live under `project integration ...` and
// `project task-mcp-server ...` and are untouched.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// oauthCallbackPaths are the spec paths whose commands are redirect handlers.
// Matching on the annotation rather than the command name means a rename in the
// generator cannot silently reintroduce them.
var oauthCallbackPaths = map[string]bool{
	"/integration/callback":     true,
	"/task_mcp_server/callback": true,
	"/task-mcp-server/callback": true,
}

// isOAuthCallbackCommand reports whether a command's only purpose is to receive
// an OAuth provider redirect.
func isOAuthCallbackCommand(cmd *cobra.Command) bool {
	if cmd.Annotations == nil {
		return false
	}
	p, ok := cmd.Annotations["pp:path"]
	if !ok {
		return false
	}
	return oauthCallbackPaths[strings.TrimSpace(p)]
}

// removeOAuthCallbackCommands prunes redirect-handler commands from the tree.
func removeOAuthCallbackCommands(root *cobra.Command) {
	var doomed []*cobra.Command
	for _, sub := range root.Commands() {
		// Only the top level: the nested project-scoped integration and
		// task-mcp-server surfaces are real commands and must survive.
		if isOAuthCallbackCommand(sub) && !sub.HasSubCommands() {
			doomed = append(doomed, sub)
		}
	}
	for _, cmd := range doomed {
		root.RemoveCommand(cmd)
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		removeOAuthCallbackCommands(root)
	})
}
