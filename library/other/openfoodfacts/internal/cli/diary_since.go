// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: per-day diary report over a date range.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type dayTotal struct {
	Day     string     `json:"day"`
	Entries int        `json:"entries"`
	Totals  nutriments `json:"totals"`
}

type diaryRangeReport struct {
	Since   string     `json:"since"`
	Until   string     `json:"until"`
	Days    []dayTotal `json:"days"`
	Average nutriments `json:"average_per_logged_day"`
}

func newNovelDiarySinceCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "since <date>",
		Short:       "Per-day macro totals and averages from a start date through today.",
		Long:        "Aggregate the diary by day from <date> (YYYY-MM-DD) through today, showing each day's totals plus the average across days that have entries.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli diary since 2026-06-08\n  openfoodfacts-pp-cli diary since 2026-06-08 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("start date is required\nUsage: %s <YYYY-MM-DD>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			since := args[0]
			if _, err := time.Parse("2006-01-02", since); err != nil {
				return usageErr(fmt.Errorf("invalid date %q: expected YYYY-MM-DD", since))
			}
			until := time.Now().Format("2006-01-02")

			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT day, COUNT(*), COALESCE(SUM(kcal),0), COALESCE(SUM(protein),0), COALESCE(SUM(fat),0),
				        COALESCE(SUM(satfat),0), COALESCE(SUM(carbs),0), COALESCE(SUM(sugars),0),
				        COALESCE(SUM(fiber),0), COALESCE(SUM(salt),0), COALESCE(SUM(sodium),0)
				 FROM diary_entry WHERE day >= ? AND day <= ? GROUP BY day ORDER BY day`, since, until)
			if err != nil {
				return fmt.Errorf("reading diary: %w", err)
			}
			defer rows.Close()

			report := diaryRangeReport{Since: since, Until: until, Days: []dayTotal{}}
			var sum nutriments
			for rows.Next() {
				var d dayTotal
				if err := rows.Scan(&d.Day, &d.Entries, &d.Totals.Kcal, &d.Totals.Protein, &d.Totals.Fat,
					&d.Totals.SatFat, &d.Totals.Carbs, &d.Totals.Sugars, &d.Totals.Fiber, &d.Totals.Salt, &d.Totals.Sodium); err != nil {
					return err
				}
				sum = sum.add(d.Totals)
				report.Days = append(report.Days, d)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if n := len(report.Days); n > 0 {
				report.Average = sum.scaled(1.0 / float64(n))
			}

			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(report.Days) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no diary entries between %s and %s\n", since, until)
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "DAY\tENTRIES\tKCAL\tPROT\tFAT\tCARBS")
			for _, d := range report.Days {
				fmt.Fprintf(tw, "%s\t%d\t%.0f\t%.1f\t%.1f\t%.1f\n", d.Day, d.Entries, d.Totals.Kcal, d.Totals.Protein, d.Totals.Fat, d.Totals.Carbs)
			}
			fmt.Fprintf(tw, "AVG\t-\t%.0f\t%.1f\t%.1f\t%.1f\n", report.Average.Kcal, report.Average.Protein, report.Average.Fat, report.Average.Carbs)
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}
