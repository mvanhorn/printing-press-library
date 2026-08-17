// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newClientsPromotedCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "clients",
		Short:       "List connected clients (ip, mac, name, tx/rx, online)",
		Long:        "List connected clients (ip, mac, name, tx/rx, online) via the GL clients.get_list RPC.",
		Example:     "  gl-inet-pp-cli clients",
		Annotations: map[string]string{"pp:endpoint": "clients.get_list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, err := c.Call(ctx, "clients", "get_list", nil)
			if err != nil {
				return classifyGLError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jsonArrayField(data, "clients"), flags)
		},
	}
	return cmd
}
