// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type unitRevenue struct {
	Unit   int `json:"unit"`
	Nights int `json:"nights_booked"`
}

type revenueResult struct {
	Period      string        `json:"period"`
	TotalNights int           `json:"total_nights"`
	Units       []unitRevenue `json:"units"`
}

func newNovelRevenueCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var month string
	var days int

	cmd := &cobra.Command{
		Use:   "revenue",
		Short: "Estimate nightly revenue from occupancy patterns across units",
		Long: `Shows booked nights per unit for a given period. Since the Hostfully API
does not expose pricing data, this reports night counts which can be
multiplied by your nightly rate externally.`,
		Example: strings.Trim(`
  hostfully-pp-cli revenue --month 2026-05
  hostfully-pp-cli revenue --days 90 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "revenue")
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

			var start, end time.Time
			if month != "" {
				t, err := time.Parse("2006-01", month)
				if err != nil {
					return fmt.Errorf("invalid month format (use YYYY-MM): %w", err)
				}
				start = t
				end = t.AddDate(0, 1, 0)
			} else {
				end = time.Now().Truncate(24 * time.Hour)
				start = end.Add(-time.Duration(days) * 24 * time.Hour)
			}

			// Count nights per unit
			nightsByUnit := make(map[int]int)
			for _, u := range unitNumbers() {
				nightsByUnit[u] = 0
			}

			for _, r := range reservations {
				ciDate := r.CheckIn.Truncate(24 * time.Hour)
				coDate := r.CheckOut.Truncate(24 * time.Hour)
				for d := ciDate; d.Before(coDate); d = d.Add(24 * time.Hour) {
					if !d.Before(start) && d.Before(end) {
						nightsByUnit[r.Unit]++
					}
				}
			}

			totalNights := 0
			var units []unitRevenue
			for _, u := range unitNumbers() {
				units = append(units, unitRevenue{Unit: u, Nights: nightsByUnit[u]})
				totalNights += nightsByUnit[u]
			}

			result := revenueResult{
				Period:      fmt.Sprintf("%s – %s", start.Format("Jan 2, 2006"), end.Add(-24*time.Hour).Format("Jan 2, 2006")),
				TotalNights: totalNights,
				Units:       units,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "## Revenue (Nights) — %s\n\n", result.Period)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Unit\tNights Booked")
			for _, u := range units {
				fmt.Fprintf(w, "#%d\t%d\n", u.Unit, u.Nights)
			}
			fmt.Fprintf(w, "\nTotal\t%d nights\n", totalNights)
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&month, "month", "", "Month to analyze (YYYY-MM)")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to look back (used when --month not set)")

	return cmd
}
