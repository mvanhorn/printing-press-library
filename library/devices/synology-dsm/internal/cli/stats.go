// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// Hand-written insight command: storage statistics aggregated from volumes and pools.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newStatsCmd returns a command that aggregates storage capacity statistics
// across volumes and storage pools, reporting total, used, free, and usage
// percentage. Uses two API calls: volume/list and pool/list.
func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Storage statistics: total, used, and free space across volumes and pools",
		Long: `Aggregates storage capacity statistics from volumes and storage pools.
Calls list-storage-volumes and list-storage-pools to compute total, used,
and free capacity with percentage breakdowns.

Useful for capacity planning and monitoring disk utilization trends.`,
		Example: `  # Human-readable storage stats
  synology-dsm-pp-cli stats

  # JSON for scripting
  synology-dsm-pp-cli stats --json

  # Compact fields only
  synology-dsm-pp-cli stats --json --compact`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := map[string]any{}

			// ── 1. Storage Volumes ───────────────────────────────────────────────
			volData, _ := c.Get("/webapi/entry.cgi/storage/volume/list", map[string]string{
				"api": "SYNO.Core.Storage.Volume", "method": "list", "version": "1",
			})
			var volumes []map[string]any
			var totalBytes, usedBytes float64
			var volResp map[string]any
			if json.Unmarshal(volData, &volResp) == nil {
				if d, ok := volResp["data"].(map[string]any); ok {
					if vols, ok := d["volumes"].([]any); ok {
						for _, v := range vols {
							vm, ok := v.(map[string]any)
							if !ok {
								continue
							}
							volInfo := map[string]any{
								"id":     vm["id"],
								"status": vm["status"],
							}
							if sizeMap, ok := vm["size"].(map[string]any); ok {
								t, _ := sizeMap["total"].(float64)
								u, _ := sizeMap["used"].(float64)
								f := t - u
								totalBytes += t
								usedBytes += u
								volInfo["total_gb"] = fmt.Sprintf("%.1f", t/1073741824)
								volInfo["used_gb"] = fmt.Sprintf("%.1f", u/1073741824)
								volInfo["free_gb"] = fmt.Sprintf("%.1f", f/1073741824)
								if t > 0 {
									volInfo["usage_pct"] = fmt.Sprintf("%.1f%%", (u/t)*100)
								}
							}
							volumes = append(volumes, volInfo)
						}
					}
				}
			}
			result["volumes"] = volumes

			// ── 2. Storage Pools ─────────────────────────────────────────────────
			poolData, _ := c.Get("/webapi/entry.cgi/storage/pool/list", map[string]string{
				"api": "SYNO.Core.Storage.Pool", "method": "list", "version": "1",
			})
			var pools []map[string]any
			var poolResp map[string]any
			if json.Unmarshal(poolData, &poolResp) == nil {
				if d, ok := poolResp["data"].(map[string]any); ok {
					if ps, ok := d["pools"].([]any); ok {
						for _, p := range ps {
							pm, ok := p.(map[string]any)
							if !ok {
								continue
							}
							pools = append(pools, map[string]any{
								"id":     pm["id"],
								"status": pm["status"],
								"raid":   pm["raid_type"],
							})
						}
					}
				}
			}
			result["pools"] = pools

			// ── 3. Aggregate totals ──────────────────────────────────────────────
			freeBytes := totalBytes - usedBytes
			aggregate := map[string]any{
				"total_gb": fmt.Sprintf("%.1f", totalBytes/1073741824),
				"used_gb":  fmt.Sprintf("%.1f", usedBytes/1073741824),
				"free_gb":  fmt.Sprintf("%.1f", freeBytes/1073741824),
			}
			if totalBytes > 0 {
				usagePct := (usedBytes / totalBytes) * 100
				aggregate["usage_pct"] = fmt.Sprintf("%.1f%%", usagePct)
			}
			result["totals"] = aggregate

			// Output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				b := mustMarshal(result)
				if flags.selectFields != "" {
					b = filterFields(b, flags.selectFields)
				} else if flags.compact {
					b = compactFields(b)
				}
				return printOutput(cmd.OutOrStdout(), b, true)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			totals := result["totals"].(map[string]any)
			fmt.Fprintf(w, "  Total:  %s GB\n", totals["total_gb"])
			fmt.Fprintf(w, "  Used:   %s GB\n", totals["used_gb"])
			fmt.Fprintf(w, "  Free:   %s GB\n", totals["free_gb"])
			if pct, ok := totals["usage_pct"]; ok {
				fmt.Fprintf(w, "  Usage:  %s\n", pct)
			}
			fmt.Fprintf(w, "  Volumes: %d   Pools: %d\n", len(volumes), len(pools))

			return nil
		},
	}
	return cmd
}
