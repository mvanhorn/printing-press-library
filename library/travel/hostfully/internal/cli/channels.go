// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type channelCount struct {
	Channel string  `json:"channel"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type channelsResult struct {
	Period   string         `json:"period"`
	Total    int            `json:"total_bookings"`
	Channels []channelCount `json:"channels"`
}

func newNovelChannelsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var period string

	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Break down booking sources over time periods to see where guests come from",
		Example: strings.Trim(`
  hostfully-pp-cli channels
  hostfully-pp-cli channels --period 90d
  hostfully-pp-cli channels --period 365d --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "channels")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("hostfully-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run 'hostfully-pp-cli sync' first)", err)
			}
			defer db.Close()

			reservations, err := loadReservations(db)
			if err != nil {
				return fmt.Errorf("loading reservations: %w", err)
			}

			// Parse period
			days := 30
			if period != "" {
				period = strings.TrimSuffix(period, "d")
				if _, err := fmt.Sscanf(period, "%d", &days); err != nil {
					return fmt.Errorf("invalid period format (use Nd, e.g., 30d, 90d): %s", period)
				}
			}

			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

			// Count channels
			counts := make(map[string]int)
			total := 0
			for _, r := range reservations {
				if r.CheckIn.Before(cutoff) {
					continue
				}
				ch := formatChannel(r.Channel)
				if ch == "" {
					ch = "Unknown"
				}
				counts[ch]++
				total++
			}

			var channels []channelCount
			for ch, count := range counts {
				pct := 0.0
				if total > 0 {
					pct = float64(count) / float64(total) * 100
				}
				channels = append(channels, channelCount{
					Channel: ch,
					Count:   count,
					Percent: pct,
				})
			}

			sort.Slice(channels, func(i, j int) bool {
				return channels[i].Count > channels[j].Count
			})

			result := channelsResult{
				Period:   fmt.Sprintf("Last %d days", days),
				Total:    total,
				Channels: channels,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "## Channel Mix — Last %d Days (%d bookings)\n\n", days, total)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Channel\tBookings\tShare")
			for _, c := range channels {
				fmt.Fprintf(w, "%s\t%d\t%.1f%%\n", c.Channel, c.Count, c.Percent)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&period, "period", "30d", "Time period to analyze (e.g., 30d, 90d, 365d)")

	return cmd
}
