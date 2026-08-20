// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRpcCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "rpc",
		Short:       "Power-user passthrough: call any GL RPC method or read/write any UCI option",
		Long:        "rpc subcommands: call (any module.function), uci (get/set/show any config option).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRpcCallCmd(flags))
	cmd.AddCommand(newRpcUciCmd(flags))
	return cmd
}
