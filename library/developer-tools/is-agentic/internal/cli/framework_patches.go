// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

package cli

import "github.com/spf13/cobra"

// Fill the framework feedback command's missing example so the live matrix
// treats its help as complete. This is a generator-template compatibility
// patch, not an API-specific behavior change.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if cmd, _, err := root.Find([]string{"feedback"}); err == nil && cmd != nil && cmd.Example == "" {
			cmd.Example = "  is-agentic-pp-cli feedback \"the report output helped\""
		}
	})
}
