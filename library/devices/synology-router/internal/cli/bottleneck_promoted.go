// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func newBottleneckPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagThreshold float64
	cmd := &cobra.Command{
		Use:         "bottleneck",
		Short:       "Identify network bottlenecks from traffic and device data",
		Long:        "Fetches traffic and device data from the API, computes per-device bandwidth utilization, and identifies devices consuming a disproportionate share of total bandwidth.",
		Example:     "  synology-router-pp-cli bottleneck --threshold 50",
		Annotations: map[string]string{"pp:endpoint": "bottleneck", "pp:method": "GET", "pp:path": "/bottleneck", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			trafficData, _, trafficErr := resolveRead(cmd.Context(), c, flags, "traffic", false, "/traffic", map[string]string{"interval": "day"}, nil)
			if trafficErr != nil {
				return classifyAPIError(trafficErr, flags)
			}
			trafficData = extractResponseData(trafficData)

			var trafficItems []map[string]any
			if json.Unmarshal(trafficData, &trafficItems) != nil || len(trafficItems) == 0 {
				fmt.Fprintln(os.Stderr, "No traffic data available")
				return nil
			}

			devData, _, devErr := resolveRead(cmd.Context(), c, flags, "devices", false, "/devices", nil, nil)
			deviceMap := map[string]map[string]any{}
			if devErr == nil {
				devData = extractResponseData(devData)
				var devItems []map[string]any
				if json.Unmarshal(devData, &devItems) == nil {
					for _, d := range devItems {
						for _, k := range []string{"name", "hostname", "device_name", "mac"} {
							if v, ok := d[k]; ok && v != nil {
								deviceMap[fmt.Sprintf("%v", v)] = d
								break
							}
						}
					}
				}
			}

			totalBandwidth := int64(0)
			for _, item := range trafficItems {
				for _, k := range []string{"download", "rx", "bytes_in"} {
					if v, ok := item[k]; ok {
						totalBandwidth += toInt64(v)
						break
					}
				}
				for _, k := range []string{"upload", "tx", "bytes_out"} {
					if v, ok := item[k]; ok {
						totalBandwidth += toInt64(v)
						break
					}
				}
			}

			type deviceBW struct {
				Name      string
				Download  int64
				Upload    int64
				Total     int64
				Pct       float64
				Device    map[string]any
			}

			var entries []deviceBW
			for _, item := range trafficItems {
				name := ""
				for _, k := range []string{"name", "hostname", "device_name", "mac", "ip"} {
					if v, ok := item[k]; ok && v != nil {
						name = fmt.Sprintf("%v", v)
						break
					}
				}
				var dl, ul int64
				for _, k := range []string{"download", "rx", "bytes_in"} {
					if v, ok := item[k]; ok {
						dl = toInt64(v)
						break
					}
				}
				for _, k := range []string{"upload", "tx", "bytes_out"} {
					if v, ok := item[k]; ok {
						ul = toInt64(v)
						break
					}
				}
				total := dl + ul
				pct := 0.0
				if totalBandwidth > 0 {
					pct = float64(total) * 100 / float64(totalBandwidth)
				}
				dev := deviceMap[name]
				entries = append(entries, deviceBW{Name: name, Download: dl, Upload: ul, Total: total, Pct: pct, Device: dev})
			}

			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Total > entries[j].Total
			})

			var bottlenecks []map[string]any
			for _, e := range entries {
				if e.Pct < flagThreshold {
					continue
				}
				entry := map[string]any{
					"name":         e.Name,
					"download":     e.Download,
					"upload":       e.Upload,
					"download_hr":  formatBytes(e.Download),
					"upload_hr":    formatBytes(e.Upload),
					"total_hr":     formatBytes(e.Total),
					"pct_of_total": fmt.Sprintf("%.1f%%", e.Pct),
				}
				if e.Device != nil {
					if status, ok := e.Device["online"]; ok {
						entry["device_online"] = status
					}
					if ip, ok := e.Device["ip"]; ok {
						entry["device_ip"] = ip
					}
				}
				bottlenecks = append(bottlenecks, entry)
			}

			result := map[string]any{
				"total_bandwidth": formatBytes(totalBandwidth),
				"threshold_pct":   fmt.Sprintf("%.0f%%", flagThreshold),
				"bottlenecks":     bottlenecks,
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && len(bottlenecks) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Total bandwidth: %s (threshold: %.0f%%)\n\n", formatBytes(totalBandwidth), flagThreshold)
				return printAutoTable(cmd.OutOrStdout(), bottlenecks)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
		},
	}
	cmd.Flags().Float64Var(&flagThreshold, "threshold", 50, "Show devices consuming at least this percentage of total bandwidth")
	_ = cmd.MarkFlagRequired("threshold")
	return cmd
}
