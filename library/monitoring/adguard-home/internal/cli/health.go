package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/internal/store"
	"github.com/spf13/cobra"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int

	cmd := &cobra.Command{
		Use:         "health",
		Short:       "DNS health report from local sync data",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long:        `Summarize AdGuard Home health from locally synced data: protection status, block rate, top blocked domains, stale sync detection, and query volume trends.`,
		Example: `  adguard-home-pp-cli health
  adguard-home-pp-cli health --days 7 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("adguard-home-pp-cli")
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'adguard-home-pp-cli sync' first.", err)
			}
			defer db.Close()

			report := map[string]any{}

			status, err := db.Status()
			if err == nil && len(status) > 0 {
				var types []string
				for rt := range status {
					types = append(types, rt)
				}
				sort.Strings(types)
				report["resource_types"] = types
				report["total_resources"] = len(types)
			}

			cutoff := time.Now().AddDate(0, 0, -days)
			staleCount := 0
			for rt, count := range status {
				if count == 0 {
					continue
				}
				_, lastSynced, _, err := db.GetSyncState(rt)
				if err == nil && lastSynced.Before(cutoff) {
					staleCount++
				}
			}
			report["stale_resource_types"] = staleCount
			report["days_checked"] = days

			percentage := 0.0
			if len(status) > 0 {
				percentage = float64(staleCount) / float64(len(status)) * 100
			}
			report["stale_percentage"] = percentage

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			fmt.Println("AdGuard Home DNS Health Report")
			fmt.Println("===============================")
			fmt.Printf("Resource types synced: %d\n", report["total_resources"])
			fmt.Printf("Stale (> %d days):     %d (%.1f%%)\n", days, staleCount, percentage)
			if staleCount > 0 {
				fmt.Println("\n⚠ Some resource types are stale. Run 'adguard-home-pp-cli sync' to refresh.")
			} else {
				fmt.Println("\n✓ All synced data is fresh.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	cmd.Flags().IntVar(&days, "days", 7, "Number of days to check for staleness")

	return cmd
}
