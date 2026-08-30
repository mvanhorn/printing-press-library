// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: system — gateway bootstrap commands.

package cli

import (
	"github.com/spf13/cobra"
)

func newSystemCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "system",
		Short:       "Gateway bootstrap and health commands",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:api-resource": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSystemCsrfCmd(flags))
	return cmd
}
