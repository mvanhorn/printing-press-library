// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelConfigSummaryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "summary",
		Short:       "A structured, per-subsystem report of the router's current configuration.",
		Long:        "Render a per-subsystem digest (system, network/WAN mode, wifi radios + region, vpn, clients) from the live GL RPC and uci config.",
		Example:     "  gl-inet-pp-cli config summary --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if cliutil.IsVerifyEnv() {
				raw, _ := json.Marshal(map[string]any{"status": "noop", "reason": "verify_short_circuit"})
				return printOutputWithFlags(out, raw, flags)
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			summary := map[string]any{}

			// System.
			info, _, infoErr := fetchSystemInfo(ctx, c)
			if infoErr == nil {
				summary["system"] = map[string]any{
					"model":         info.Model,
					"firmware":      info.FirmwareVersion,
					"openwrt":       info.BoardInfo.OpenwrtVersion,
					"country_code":  info.CountryCode,
					"architecture":  info.BoardInfo.Architecture,
					"software_feat": info.SoftwareFeature,
				}
			}

			// Network mode.
			netInfo := map[string]any{}
			if data, err := c.Call(ctx, "netmode", "get_mode", nil); err == nil {
				if m := jsonObjField(data, "mode"); m != nil {
					var mode string
					_ = json.Unmarshal(m, &mode)
					netInfo["mode"] = mode
				}
			}

			// WiFi radios.
			if data, err := c.Call(ctx, "wifi", "get_status", nil); err == nil {
				var radios []map[string]any
				_ = json.Unmarshal(jsonArrayField(data, "res"), &radios)
				summary["wifi"] = map[string]any{"radios": radios}
			}

			// VPN.
			tunnels := collectVPNTunnels(ctx, c)
			summary["vpn"] = map[string]any{"count": len(tunnels), "tunnels": tunnels}

			// Clients count.
			if data, err := c.Call(ctx, "clients", "get_list", nil); err == nil {
				var clients []map[string]any
				_ = json.Unmarshal(jsonArrayField(data, "clients"), &clients)
				summary["clients"] = map[string]any{"count": len(clients)}
			}

			// uci network + wireless (region per radio).
			cfg, sshErr := glSSHConfig(c)
			if sshErr == nil {
				if wireless, err := glssh.UCIShow(ctx, cfg, "wireless"); err == nil {
					region := extractRadioCountries(parseUCIShow(wireless))
					if wm, ok := summary["wifi"].(map[string]any); ok {
						wm["region"] = region
					} else {
						summary["wifi"] = map[string]any{"region": region}
					}
				}
				if network, err := glssh.UCIShow(ctx, cfg, "network"); err == nil {
					netInfo["uci"] = parseUCIShow(network)
				}
			}
			summary["network"] = netInfo

			raw, _ := json.Marshal(summary)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	return cmd
}
