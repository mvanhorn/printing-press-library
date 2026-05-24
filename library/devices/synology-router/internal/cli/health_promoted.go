// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newHealthPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Network health overview: device counts, bandwidth summary, and WAN status",
		Long:        "Aggregates device, traffic, and WAN data into a single health snapshot. Shows online/offline counts, top bandwidth consumers, and WAN connectivity.",
		Example:     "  synology-router-pp-cli health",
		Annotations: map[string]string{"pp:endpoint": "health", "pp:method": "GET", "pp:path": "/health", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report := map[string]any{}

			devData, _, devErr := resolveRead(cmd.Context(), c, flags, "devices", false, "/devices", nil, nil)
			if devErr == nil {
				devData = extractResponseData(devData)
				var items []map[string]any
				if json.Unmarshal(devData, &items) == nil {
					online, offline := deviceOnlineSummary(items)
					report["devices"] = map[string]any{
						"total":   len(items),
						"online":  online,
						"offline": offline,
					}
				}
			}

			trafficData, _, trafficErr := resolveRead(cmd.Context(), c, flags, "traffic", false, "/traffic", map[string]string{"interval": "live"}, nil)
			if trafficErr == nil {
				trafficData = extractResponseData(trafficData)
				var items []map[string]any
				if json.Unmarshal(trafficData, &items) == nil {
					sortTrafficByDownload(items)
					if len(items) > 5 {
						items = items[:5]
					}
					items = enrichTrafficItems(items)
					report["top_talkers"] = items
				}
			}

			wanData, _, wanErr := resolveRead(cmd.Context(), c, flags, "wan", false, "/wan/status", nil, nil)
			if wanErr == nil {
				wanData = extractResponseData(wanData)
				var wan map[string]any
				if json.Unmarshal(wanData, &wan) == nil {
					report["wan"] = wan
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if dev, ok := report["devices"].(map[string]any); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "Devices: %d online, %d offline (%d total)\n", dev["online"], dev["offline"], dev["total"])
				}
				if topTalkers, ok := report["top_talkers"].([]map[string]any); ok && len(topTalkers) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\nTop Talkers:\n")
					printAutoTable(cmd.OutOrStdout(), topTalkers)
				}
				if wan, ok := report["wan"].(map[string]any); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "\nWAN: %v\n", wan)
				}
				return nil
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(report), flags)
		},
	}
	return cmd
}
