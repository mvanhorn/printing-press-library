// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelWwdcCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "wwdc",
		Short:       "WWDC session lookups: reverse-index sessions to the doc pages that cite them",
		Long:        "WWDC session lookups. 'wwdc symbols <session-id>' returns the list of doc pages whose references{} cite a given WWDC session.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelWwdcSymbolsCmd(flags))
	return cmd
}
