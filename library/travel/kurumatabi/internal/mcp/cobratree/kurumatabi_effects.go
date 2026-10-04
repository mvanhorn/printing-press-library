// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import "github.com/spf13/cobra"

type mirroredEffects struct {
	readOnly, localWrite, openWorld bool
	description                     string
}

// Keep CLI annotations and behavior intact; MCP hints describe the actual
// companion effects, including its intentional local caches and learning state.
func kurumatabiMirroredEffects(cmd *cobra.Command, path []string) mirroredEffects {
	e := mirroredEffects{readOnly: isMCPReadOnly(cmd), localWrite: isMCPLocalWrite(cmd)}
	if cmd.Root().Name() != "kurumatabi-pp-cli" || !e.readOnly {
		return e
	}
	if len(path) == 2 && path[0] == "parks" {
		switch path[1] {
		case "filters", "near", "match":
			return e
		}
		return mirroredEffects{localWrite: true, openWorld: true, description: "May refresh the local source-evidence cache; no provider transaction."}
	}
	return mirroredEffects{localWrite: true, description: "Uses writable local CLI learning or journal state."}
}
