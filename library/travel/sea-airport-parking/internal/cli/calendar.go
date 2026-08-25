// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: entry-date availability calendar. Hand-authored.
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"

	"github.com/spf13/cobra"
)

type calendarDay struct {
	Date      string  `json:"date"` // entry date YYYY-MM-DD
	Available bool    `json:"available"`
	SoldOut   bool    `json:"sold_out"`
	Price     float64 `json:"price,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}

type calendarResult struct {
	Month       string        `json:"month"`
	Nights      int           `json:"nights"`
	ScannedDays int           `json:"scanned_days"`
	Days        []calendarDay `json:"days"`
	Note        string        `json:"note,omitempty"`
}

func newNovelCalendarCmd(flags *rootFlags) *cobra.Command {
	var month, tod, promo string
	var nights, maxScanDays int

	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Render price and open/sold-out across many entry days for a fixed-length stay.",
		Long: "Render price and open/sold-out across every entry day in a month for a\n" +
			"fixed-length stay. Loops the live quote flow over each candidate entry date and\n" +
			"returns the whole grid, persisting each quote.\n\n" +
			"Unlike 'sweep' it does not vary stay length or hunt a single winner — it returns\n" +
			"the whole grid for eyeballing.",
		Example: "  sea-airport-parking-pp-cli calendar --month 2026-11 --nights 3 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would render the availability calendar for the month")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if month == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--month is required (YYYY-MM)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			first, err := time.ParseInLocation("2006-01", month, time.Local)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --month %q (use YYYY-MM)", month))
			}
			hour, minute, err := parseTimeOfDay(tod)
			if err != nil {
				return usageErr(err)
			}
			if cliutil.IsDogfoodEnv() && maxScanDays > 1 {
				maxScanDays = 1
			}

			pc, err := newParkingClient(flags)
			if err != nil {
				return err
			}

			res := calendarResult{Month: month, Nights: nights, Days: []calendarDay{}}
			day := time.Date(first.Year(), first.Month(), 1, hour, minute, 0, 0, time.Local)
			for day.Month() == first.Month() && res.ScannedDays < maxScanDays {
				exitT := day.AddDate(0, 0, nights)
				q, qerr := pc.Quote(ctx, day, exitT, promo)
				if qerr != nil {
					return fmt.Errorf("calendar %s: %w", day.Format("2006-01-02"), qerr)
				}
				res.ScannedDays++
				if perr := persistQuote(ctx, defaultDBPath(dbName), q); perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist quote for %s: %v\n", day.Format("2006-01-02"), perr)
				}
				res.Days = append(res.Days, calendarDay{
					Date:      day.Format("2006-01-02"),
					Available: q.Available,
					SoldOut:   q.SoldOut,
					Price:     q.TotalPrice,
					Reason:    q.Reason,
				})
				day = day.AddDate(0, 0, 1)
			}
			if day.Month() == first.Month() {
				res.Note = fmt.Sprintf("stopped at --max-scan-days (%d); raise it to cover the whole month", maxScanDays)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			renderCalendarHuman(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "Month to scan (YYYY-MM)")
	cmd.Flags().IntVar(&nights, "nights", 3, "Length of stay in nights")
	cmd.Flags().StringVar(&tod, "time", "11:00", "Entry/exit time of day (HH:MM)")
	cmd.Flags().StringVar(&promo, "promo", "", "Optional promo code to apply")
	cmd.Flags().IntVar(&maxScanDays, "max-scan-days", 31, "Maximum entry days to scan")
	return cmd
}

func renderCalendarHuman(cmd *cobra.Command, res calendarResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — %d-night stays (%d day(s) scanned):\n", res.Month, res.Nights, res.ScannedDays)
	for _, d := range res.Days {
		switch {
		case d.Available:
			fmt.Fprintf(w, "  %s  available  $%.2f\n", d.Date, d.Price)
		case d.SoldOut:
			fmt.Fprintf(w, "  %s  SOLD OUT\n", d.Date)
		default:
			label := d.Reason
			if label == "" {
				label = "unavailable"
			}
			fmt.Fprintf(w, "  %s  %s\n", d.Date, label)
		}
	}
	if res.Note != "" {
		fmt.Fprintf(w, "%s\n", res.Note)
	}
}
