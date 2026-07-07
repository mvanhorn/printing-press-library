// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
)

func newVpnPromotedCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "vpn",
		Short:       "Configured VPN client tunnels and their status",
		Long:        "List WireGuard and OpenVPN client tunnels, merged from wg-client and ovpn-client config lists.",
		Example:     "  gl-inet-pp-cli vpn",
		Annotations: map[string]string{"pp:endpoint": "vpn.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			tunnels := collectVPNTunnels(ctx, c)
			raw, _ := json.Marshal(tunnels)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}

	cmd.AddCommand(newNovelVpnToggleCmd(flags))
	cmd.AddCommand(newNovelVpnVerifyCmd(flags))

	return cmd
}

// collectVPNTunnels merges wg-client and ovpn-client config lists into one
// tagged array. Errors from either module are swallowed (a tunnel type may be
// unconfigured or unavailable on the firmware) so the surviving list still
// renders.
func collectVPNTunnels(ctx context.Context, c interface {
	Call(ctx context.Context, module, function string, args any) (json.RawMessage, error)
}) []map[string]any {
	tunnels := []map[string]any{}
	for _, m := range []struct{ module, typ string }{
		{"wg-client", "wireguard"},
		{"ovpn-client", "openvpn"},
	} {
		data, err := c.Call(ctx, m.module, "get_all_config_list", nil)
		if err != nil {
			continue
		}
		var list []map[string]any
		if json.Unmarshal(jsonArrayField(data, "config_list"), &list) != nil {
			continue
		}
		for _, t := range list {
			if t == nil {
				t = map[string]any{}
			}
			t["type"] = m.typ
			tunnels = append(tunnels, t)
		}
	}
	return tunnels
}
