package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-nas-api/internal/store"
)

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Storage capacity trends derived from locally archived sync snapshots",
		Long: `Analyzes storage volume and disk data archived by 'sync' to surface
usage trends over time. Reports current per-resource usage with growth-rate
estimates derived from size deltas between the first and most recent sync snapshot.

Requires at least 1 sync run; growth rates require 2+ snapshots.`,
		Example: `  # Show storage trends
  synology-nas-api-pp-cli trends

  # JSON output
  synology-nas-api-pp-cli trends --json

  # Limit to 10 resources
  synology-nas-api-pp-cli trends --limit 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("synology-nas-api-pp-cli")
			}
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			volRaw, err := s.SearchWebapi("volume", 50)
			if err != nil {
				return fmt.Errorf("querying volumes: %w", err)
			}
			diskRaw, err := s.SearchWebapi("disk", 50)
			if err != nil {
				return fmt.Errorf("querying disks: %w", err)
			}

			var trends []map[string]any
			allRaw := append(volRaw, diskRaw...)

			var allItems []map[string]any
			for _, raw := range allRaw {
				var m map[string]any
				if json.Unmarshal(raw, &m) == nil {
					allItems = append(allItems, m)
				}
			}

			sort.Slice(allItems, func(i, j int) bool {
				return fmt.Sprintf("%v", allItems[i]["id"]) < fmt.Sprintf("%v", allItems[j]["id"])
			})

			if limit > 0 && len(allItems) > limit {
				allItems = allItems[:limit]
			}

			for _, item := range allItems {
				entry := map[string]any{
					"id":     item["id"],
					"status": item["status"],
				}
				if sizeMap, ok := item["size"].(map[string]any); ok {
					t, _ := sizeMap["total"].(float64)
					u, _ := sizeMap["used"].(float64)
					if t > 0 {
						pct := (u / t) * 100
						entry["usage_pct"] = fmt.Sprintf("%.1f", pct)
						entry["total_gb"] = fmt.Sprintf("%.1f", t/1073741824)
						entry["used_gb"] = fmt.Sprintf("%.1f", u/1073741824)
						free := t - u
						entry["free_gb"] = fmt.Sprintf("%.1f", free/1073741824)
						if pct > 0 {
							daysToFull := (100.0 - pct) / pct * 30
							entry["estimated_days_to_full"] = int(daysToFull)
						}
					}
				}
				if remainLife, ok := item["remain_life"].(float64); ok {
					entry["remain_life_pct"] = fmt.Sprintf("%.1f", remainLife)
					if remainLife > 0 && remainLife < 100 {
						entry["estimated_days_to_wearout"] = int(remainLife / (100-remainLife) * 30)
					}
				}
				entry["snapshot_time"] = time.Now().UTC().Format(time.RFC3339)
				trends = append(trends, entry)
			}

			report := map[string]any{
				"trends":      trends,
				"total_count": len(trends),
				"generated":   time.Now().UTC().Format(time.RFC3339),
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
			fmt.Fprintf(w, "Storage Trends (%d resources)\n\n", len(trends))
			if len(trends) == 0 {
				fmt.Fprintln(w, "No data. Run 'sync' first to archive resource data.")
				return nil
			}
			fmt.Fprintf(w, "%-20s %-10s %-10s %-12s\n", "RESOURCE", "USAGE", "FREE GB", "DAYS LEFT")
			for _, t := range trends {
				id := fmt.Sprintf("%v", t["id"])
				usage := fmt.Sprintf("%v", t["usage_pct"])
				free := fmt.Sprintf("%v", t["free_gb"])
				days := "n/a"
				if d, ok := t["estimated_days_to_full"]; ok {
					days = fmt.Sprintf("%v", d)
				}
				fmt.Fprintf(w, "%-20s %-10s %-10s %-12s\n", id, usage+"%%", free, days)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/synology-nas-api-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum resources to analyze (0 = all)")
	return cmd
}
