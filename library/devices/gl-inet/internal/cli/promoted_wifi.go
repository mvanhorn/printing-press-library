// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newWifiPromotedCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "wifi",
		Short:       "Radio states, channels, bands",
		Long:        "Per-radio WiFi state (name, channel, state) via the GL wifi.get_status RPC.",
		Example:     "  gl-inet-pp-cli wifi",
		Annotations: map[string]string{"pp:endpoint": "wifi.get_status", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, err := c.Call(ctx, "wifi", "get_status", nil)
			if err != nil {
				return classifyGLError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jsonArrayField(data, "res"), flags)
		},
	}

	// Wire travel-macro subcommands.
	cmd.AddCommand(newNovelWifiRegionCmd(flags))

	return cmd
}
