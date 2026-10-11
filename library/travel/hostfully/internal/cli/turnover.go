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

type turnoverGap struct {
	Unit      int    `json:"unit"`
	GapStart  string `json:"gap_start"`
	GapEnd    string `json:"gap_end"`
	Hours     int    `json:"hours"`
	SameDay   bool   `json:"same_day_turnover"`
	PrevGuest string `json:"prev_guest"`
	NextGuest string `json:"next_guest"`
}

type turnoverResult struct {
	Period  string        `json:"period"`
	Gaps    []turnoverGap `json:"gaps"`
	SameDay int           `json:"same_day_turnovers"`
}

func newNovelTurnoverCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int

	cmd := &cobra.Command{
		Use:   "turnover",
		Short: "Identify cleaning windows between checkout and next checkin for each unit",
		Example: strings.Trim(`
  hostfully-pp-cli turnover
  hostfully-pp-cli turnover --days 14
  hostfully-pp-cli turnover --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "turnover")
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

			today := time.Now().Truncate(24 * time.Hour)
			endDate := today.Add(time.Duration(days) * 24 * time.Hour)

			// Group by unit and sort by check-in
			byUnit := make(map[int][]reservation)
			for _, r := range reservations {
				if r.CheckOut.Before(today) && r.CheckIn.Before(today) {
					continue // skip past reservations
				}
				if r.CheckIn.After(endDate) {
					continue
				}
				byUnit[r.Unit] = append(byUnit[r.Unit], r)
			}

			var gaps []turnoverGap
			sameDayCount := 0

			for unit, resos := range byUnit {
				sort.Slice(resos, func(i, j int) bool {
					return resos[i].CheckIn.Before(resos[j].CheckIn)
				})

				for i := 0; i < len(resos)-1; i++ {
					checkout := resos[i].CheckOut
					nextCheckin := resos[i+1].CheckIn

					if checkout.After(endDate) || nextCheckin.Before(today) {
						continue
					}

					gapHours := int(nextCheckin.Sub(checkout).Hours())
					sameDay := checkout.Truncate(24 * time.Hour).Equal(nextCheckin.Truncate(24 * time.Hour))

					if sameDay {
						sameDayCount++
					}

					gaps = append(gaps, turnoverGap{
						Unit:      unit,
						GapStart:  checkout.Format("Jan 2 3:04 PM"),
						GapEnd:    nextCheckin.Format("Jan 2 3:04 PM"),
						Hours:     gapHours,
						SameDay:   sameDay,
						PrevGuest: fmt.Sprintf("%s %s", resos[i].GuestFirst, resos[i].GuestLast),
						NextGuest: fmt.Sprintf("%s %s", resos[i+1].GuestFirst, resos[i+1].GuestLast),
					})
				}
			}

			// Sort by date
			sort.Slice(gaps, func(i, j int) bool {
				return gaps[i].GapStart < gaps[j].GapStart
			})

			result := turnoverResult{
				Period:  fmt.Sprintf("Next %d days", days),
				Gaps:    gaps,
				SameDay: sameDayCount,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "## Turnovers — Next %d Days\n\n", days)
			if sameDayCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠ %d same-day turnovers (4 hours to clean)\n\n", sameDayCount)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Unit\tCheckout\tNext Check-in\tGap\tAlert")
			for _, g := range gaps {
				alert := ""
				if g.SameDay {
					alert = "SAME-DAY"
				} else if g.Hours < 24 {
					alert = "TIGHT"
				}
				fmt.Fprintf(w, "#%d\t%s\t%s\t%dh\t%s\n",
					g.Unit, g.GapStart, g.GapEnd, g.Hours, alert)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 7, "Number of days to look ahead")

	return cmd
}
