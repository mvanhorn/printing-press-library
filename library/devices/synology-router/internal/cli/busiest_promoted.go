// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newBusiestPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagInterval string
	var flagTop int

	cmd := &cobra.Command{
		Use:         "busiest",
		Short:       "Rank devices by bandwidth consumption with percentage of total",
		Long:        "Fetches traffic data, ranks by download, and calculates each device's percentage of total bandwidth. Includes human-readable sizes.",
		Example:     "  synology-router-pp-cli busiest --interval day --top 10",
		Annotations: map[string]string{"pp:endpoint": "busiest", "pp:method": "GET", "pp:path": "/busiest", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{"interval": flagInterval}
			data, _, err := resolveRead(cmd.Context(), c, flags, "traffic", false, "/traffic", params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			var items []map[string]any
			if json.Unmarshal(data, &items) != nil || len(items) == 0 {
				fmt.Fprintln(os.Stderr, "No traffic data available")
				return nil
			}

			sortTrafficByDownload(items)
			if flagTop > 0 && flagTop < len(items) {
				items = items[:flagTop]
			}

			totalDownload := int64(0)
			for _, item := range items {
				for _, k := range []string{"download", "rx", "bytes_in"} {
					if v, ok := item[k]; ok {
						totalDownload += toInt64(v)
						break
					}
				}
			}

			for _, item := range items {
				var dl int64
				for _, k := range []string{"download", "rx", "bytes_in"} {
					if v, ok := item[k]; ok {
						dl = toInt64(v)
						break
					}
				}
				if totalDownload > 0 {
					pct := float64(dl) * 100 / float64(totalDownload)
					item["percentage"] = fmt.Sprintf("%.1f%%", pct)
				}
				item["download_hr"] = formatBytes(dl)
				var ul int64
				for _, k := range []string{"upload", "tx", "bytes_out"} {
					if v, ok := item[k]; ok {
						ul = toInt64(v)
						break
					}
				}
				item["upload_hr"] = formatBytes(ul)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(items), flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(items), flags)
		},
	}
	cmd.Flags().StringVar(&flagInterval, "interval", "day", "Time window for measuring device bandwidth usage: live, day, week, month")
	cmd.Flags().IntVar(&flagTop, "top", 10, "Maximum number of top bandwidth-consuming devices to display")
	_ = cmd.MarkFlagRequired("interval")
	return cmd
}
