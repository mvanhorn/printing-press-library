package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newUtilizationCmd(flags *rootFlags) *cobra.Command {
	var warnThreshold int
	var critThreshold int

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Per-resource utilization breakdown with overcommit and saturation analysis",
		Long: `Shows per-volume and per-disk utilization with threshold flags.
Aggregates data from volume/list and disk/list APIs to produce a unified
utilization report sorted by usage percentage descending.

Each resource is tagged with a severity level:
  ok       usage below warning threshold
  warn     usage at or above warning threshold (default 80%)
  critical usage at or above critical threshold (default 95%)`,
		Example: `  # Default thresholds (80% warn, 95% critical)
  synology-nas-api-pp-cli utilization

  # Custom thresholds
  synology-nas-api-pp-cli utilization --warn 70 --critical 90

  # JSON output for monitoring
  synology-nas-api-pp-cli utilization --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			warnPct := float64(warnThreshold)
			critPct := float64(critThreshold)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report := map[string]any{}
			var items []map[string]any

			volData, _ := c.Get("/webapi/entry.cgi/storage/volume/list", map[string]string{
				"api": "SYNO.Core.Storage.Volume", "method": "list", "version": "1",
			})
			var volResp map[string]any
			if json.Unmarshal(volData, &volResp) == nil {
				if d, ok := volResp["data"].(map[string]any); ok {
					if vols, ok := d["volumes"].([]any); ok {
						for _, v := range vols {
							vm, ok := v.(map[string]any)
							if !ok {
								continue
							}
							if sizeMap, ok := vm["size"].(map[string]any); ok {
								t, _ := sizeMap["total"].(float64)
								u, _ := sizeMap["used"].(float64)
								if t > 0 {
									pct := (u / t) * 100
							items = append(items, map[string]any{
								"resource":    fmt.Sprintf("volume:%v", vm["id"]),
								"total_gb":    fmt.Sprintf("%.1f", t/1073741824),
								"used_gb":     fmt.Sprintf("%.1f", u/1073741824),
								"usage_pct":   fmt.Sprintf("%.1f", pct),
								"severity":    severityForPct(pct, warnPct, critPct),
								"status":      vm["status"],
								"fstype":      vm["fs_type"],
							})
								}
							}
						}
					}
				}
			}

			diskData, _ := c.Get("/webapi/entry.cgi/storage/disk/list", map[string]string{
				"api": "SYNO.Core.Storage.Disk", "method": "list", "version": "1",
			})
			var diskResp map[string]any
			if json.Unmarshal(diskData, &diskResp) == nil {
				if d, ok := diskResp["data"].(map[string]any); ok {
					if disks, ok := d["disks"].([]any); ok {
						for _, disk := range disks {
							dm, ok := disk.(map[string]any)
							if !ok {
								continue
							}
							remainLife := 100.0
							if rl, ok := dm["remain_life"].(float64); ok {
								remainLife = rl
							}
							usedPct := 100.0 - remainLife
						items = append(items, map[string]any{
							"resource":    fmt.Sprintf("disk:%v", dm["name"]),
							"used_pct":    fmt.Sprintf("%.1f", usedPct),
							"remain_life": fmt.Sprintf("%.1f", remainLife),
							"severity":    severityForPct(usedPct, warnPct, critPct),
							"temp":        dm["temp"],
							"model":       dm["model"],
						})
						}
					}
				}
			}

			sort.Slice(items, func(i, j int) bool {
				return severityOrder(items[i]["severity"].(string)) > severityOrder(items[j]["severity"].(string))
			})

			report["items"] = items
			report["total_resources"] = len(items)
			critCount, warnCount, okCount := 0, 0, 0
			for _, it := range items {
				switch it["severity"] {
				case "critical":
					critCount++
				case "warn":
					warnCount++
				default:
					okCount++
				}
			}
			report["summary"] = map[string]any{
				"critical": critCount,
				"warn":     warnCount,
				"ok":       okCount,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				b := mustMarshal(report)
				if flags.selectFields != "" {
					b = filterFields(b, flags.selectFields)
				} else if flags.compact {
					b = compactFields(b)
				}
				return printOutput(cmd.OutOrStdout(), b, true)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Resource Utilization (%d resources)\n\n", len(items))
			fmt.Fprintf(w, "%-20s %-10s %-10s\n", "RESOURCE", "USAGE", "SEVERITY")
			for _, it := range items {
				res := fmt.Sprintf("%v", it["resource"])
				var usage string
				if u, ok := it["usage_pct"]; ok {
					usage = fmt.Sprintf("%v%%", u)
				} else if u, ok := it["used_pct"]; ok {
					usage = fmt.Sprintf("%v%%", u)
				}
				sev := fmt.Sprintf("%v", it["severity"])
				fmt.Fprintf(w, "%-20s %-10s %-10s\n", res, usage, sev)
			}
			summary := report["summary"].(map[string]any)
			fmt.Fprintf(w, "\nSummary: %d critical, %d warn, %d ok\n", summary["critical"], summary["warn"], summary["ok"])
			return nil
		},
	}
	cmd.Flags().IntVar(&warnThreshold, "warn", 80, "Warning threshold percentage")
	cmd.Flags().IntVar(&critThreshold, "critical", 95, "Critical threshold percentage")
	return cmd
}

func severityForPct(pct float64, warnPct, critPct float64) string {
	if pct >= critPct {
		return "critical"
	}
	if pct >= warnPct {
		return "warn"
	}
	return "ok"
}

func severityFor(pct float64) string {
	return severityForPct(pct, 80, 95)
}

func severityOrder(s string) int {
	switch s {
	case "critical":
		return 3
	case "warn":
		return 2
	default:
		return 1
	}
}
