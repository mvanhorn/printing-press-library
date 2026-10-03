// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import "github.com/spf13/cobra"

func newDestinationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "destinations", Short: "Resolve exact source city and property IDs", Example: "  hostelworld-pp-cli destinations search Osaka --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newDestinationsSearchCmd(flags))
	return cmd
}
