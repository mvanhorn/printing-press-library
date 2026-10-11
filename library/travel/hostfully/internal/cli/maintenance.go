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

type maintWindow struct {
	Unit  int    `json:"unit"`
	Start string `json:"start"`
	End   string `json:"end"`
	Days  int    `json:"days"`
}

type maintenanceResult struct {
	Period  string        `json:"period"`
	MinDays int           `json:"min_days"`
	Windows []maintWindow `json:"windows"`
}

func newNovelMaintenanceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var units string
	var minDays int
	var days int

	cmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Find optimal maintenance slots across all units considering upcoming bookings",
		Example: strings.Trim(`
  hostfully-pp-cli maintenance
  hostfully-pp-cli maintenance --units 3,4 --min-days 2
  hostfully-pp-cli maintenance --days 60 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "maintenance")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("hostfully-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run 'hostfully-pp-cli sync' first)", err)
			}
			defer db.Close()

			targetUnits, err := parseUnits(units)
			if err != nil {
				return err
			}

			reservations, err := loadReservations(db)
			if err != nil {
				return fmt.Errorf("loading reservations: %w", err)
			}

			today := time.Now().Truncate(24 * time.Hour)

			// Build occupancy per unit
			occ := make(map[int]map[string]bool)
			for _, u := range targetUnits {
				occ[u] = make(map[string]bool)
			}

			for _, r := range reservations {
				if _, ok := occ[r.Unit]; !ok {
					continue
				}
				ciDate := r.CheckIn.Truncate(24 * time.Hour)
				coDate := r.CheckOut.Truncate(24 * time.Hour)
				for d := ciDate; d.Before(coDate); d = d.Add(24 * time.Hour) {
					occ[r.Unit][d.Format("2006-01-02")] = true
				}
			}

			// Find vacant windows per unit
			var windows []maintWindow
			for _, u := range targetUnits {
				i := 0
				for i < days {
					day := today.Add(time.Duration(i) * 24 * time.Hour)
					dayStr := day.Format("2006-01-02")
					if occ[u][dayStr] {
						i++
						continue
					}
					// Start of a gap
					gapStart := day
					j := i + 1
					for j < days {
						next := today.Add(time.Duration(j) * 24 * time.Hour)
						if occ[u][next.Format("2006-01-02")] {
							break
						}
						j++
					}
					gapEnd := today.Add(time.Duration(j-1) * 24 * time.Hour)
					gapDays := j - i

					if gapDays >= minDays {
						windows = append(windows, maintWindow{
							Unit:  u,
							Start: gapStart.Format("Mon Jan 2"),
							End:   gapEnd.Format("Mon Jan 2"),
							Days:  gapDays,
						})
					}
					i = j
				}
			}

			// Sort by days descending (longest windows first)
			sort.Slice(windows, func(i, j int) bool {
				return windows[i].Days > windows[j].Days
			})

			result := maintenanceResult{
				Period:  fmt.Sprintf("Next %d days", days),
				MinDays: minDays,
				Windows: windows,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "## Maintenance Windows — Next %d Days (%d+ day slots)\n\n", days, minDays)
			if len(windows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No %d+ day maintenance windows found.\n", minDays)
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Unit\tWindow\tDays")
			for _, mw := range windows {
				fmt.Fprintf(w, "#%d\t%s – %s\t%d\n", mw.Unit, mw.Start, mw.End, mw.Days)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&units, "units", "all", "Comma-separated unit numbers or 'all'")
	cmd.Flags().IntVar(&minDays, "min-days", 1, "Minimum gap size to show")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to search ahead")

	return cmd
}
