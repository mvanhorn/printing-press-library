// Package cli provides the snapshot command for archiving analytics to local SQLite.
// Calls GET /analytics with multiple groupBy values to capture a full analytics snapshot.
// Stores results in the analytics_snapshots table.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/CLI-PP-Library/dub-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagStart string
	var flagEnd string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Archive analytics data to local SQLite for offline analysis",
		Long: `Capture a full analytics snapshot by querying multiple groupBy dimensions
(count, timeseries, countries, devices, browsers, os, referers, top_links)
and storing the results locally in SQLite.`,
		Example: `  # Snapshot last 30 days of analytics
  dub-pp-cli snapshot

  # Snapshot last 90 days
  dub-pp-cli snapshot --days 90

  # Snapshot a specific date range
  dub-pp-cli snapshot --start 2025-01-01 --end 2025-01-31

  # Output summary as JSON
  dub-pp-cli snapshot --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Determine date range
			var periodStart, periodEnd string
			if flagStart != "" {
				periodStart = flagStart
			} else {
				periodStart = time.Now().AddDate(0, 0, -flagDays).Format("2006-01-02")
			}
			if flagEnd != "" {
				periodEnd = flagEnd
			} else {
				periodEnd = time.Now().Format("2006-01-02")
			}

			// Open local store
			if dbPath == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".local", "share", "dub-pp-cli", "data.db")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			dimensions := []string{
				"count", "timeseries", "countries", "devices",
				"browsers", "os", "referers", "top_links",
			}

			var capturedCount int

			for _, dim := range dimensions {
				fmt.Fprintf(cmd.ErrOrStderr(), "Fetching analytics groupBy=%s...\n", dim)

				params := map[string]string{
					"groupBy": dim,
					"start":   periodStart,
					"end":     periodEnd,
				}

				data, err := c.Get("/analytics", params)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  warning: %s: %v\n", dim, err)
					continue
				}

				if err := s.UpsertAnalyticsSnapshot(dim, "clicks", periodStart, periodEnd, data); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  warning: storing %s: %v\n", dim, err)
					continue
				}

				capturedCount++
			}

			// Output
			if flags.asJSON {
				output := map[string]any{
					"dimensions_captured": capturedCount,
					"dimensions_total":    len(dimensions),
					"period_start":        periodStart,
					"period_end":          periodEnd,
					"store_path":          dbPath,
					"timestamp":           time.Now().UTC().Format(time.RFC3339),
				}
				data, _ := json.Marshal(output)
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot captured: %d dimensions, %s to %s\n",
				capturedCount, periodStart, periodEnd)
			return nil
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days to look back (default 30)")
	cmd.Flags().StringVar(&flagStart, "start", "", "Start date (YYYY-MM-DD), overrides --days")
	cmd.Flags().StringVar(&flagEnd, "end", "", "End date (YYYY-MM-DD), defaults to today")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/dub-pp-cli/data.db)")

	return cmd
}
