// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newSystemStatusCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Live CPU, memory, clients, services, WAN/internet state",
		Example:     "  gl-inet-pp-cli system status --agent",
		Annotations: map[string]string{"pp:endpoint": "system.get_status", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, err := c.Call(ctx, "system", "get_status", nil)
			if err != nil {
				return classifyGLError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
