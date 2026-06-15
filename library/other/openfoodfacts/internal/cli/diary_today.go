// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: today's diary totals vs goal.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type diaryDayReport struct {
	Day       string      `json:"day"`
	Entries   int         `json:"entries"`
	Totals    nutriments  `json:"totals"`
	Goal      *nutriments `json:"goal,omitempty"`
	Remaining *nutriments `json:"remaining,omitempty"`
}

func newNovelDiaryTodayCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagDay string

	cmd := &cobra.Command{
		Use:         "today",
		Short:       "Show running kcal/macro totals for today against your daily goal.",
		Long:        "Sum the macros logged to the diary for today (or --day) and, if a goal is set, show how much of the daily kcal/protein/fat/carbs budget remains.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli diary today\n  openfoodfacts-pp-cli diary today --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			day := flagDay
			if strings.TrimSpace(day) == "" {
				day = time.Now().Format("2006-01-02")
			}
			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			totals, count, err := sumDiaryDay(cmd.Context(), db.DB(), day)
			if err != nil {
				return fmt.Errorf("reading diary: %w", err)
			}
			report := diaryDayReport{Day: day, Entries: count, Totals: totals}
			if goal, ok, err := loadGoal(cmd.Context(), db.DB()); err != nil {
				return err
			} else if ok {
				rem := nutriments{
					Kcal:    goal.Kcal - totals.Kcal,
					Protein: goal.Protein - totals.Protein,
					Fat:     goal.Fat - totals.Fat,
					Carbs:   goal.Carbs - totals.Carbs,
				}
				report.Goal = &goal
				report.Remaining = &rem
			}

			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s — %d entries\n", day, count)
			fmt.Fprintf(out, "  totals: %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
				totals.Kcal, totals.Protein, totals.Fat, totals.Carbs)
			if report.Goal != nil {
				fmt.Fprintf(out, "  goal:   %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
					report.Goal.Kcal, report.Goal.Protein, report.Goal.Fat, report.Goal.Carbs)
				fmt.Fprintf(out, "  left:   %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
					report.Remaining.Kcal, report.Remaining.Protein, report.Remaining.Fat, report.Remaining.Carbs)
			} else {
				fmt.Fprintln(out, "  (no goal set — run `diary goal --kcal ...`)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().StringVar(&flagDay, "day", "", "Day to report (YYYY-MM-DD); defaults to today")
	return cmd
}

// sumDiaryDay returns summed macros and entry count for a single day.
func sumDiaryDay(ctx context.Context, db *sql.DB, day string) (nutriments, int, error) {
	row := db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(kcal),0), COALESCE(SUM(protein),0), COALESCE(SUM(fat),0),
		        COALESCE(SUM(satfat),0), COALESCE(SUM(carbs),0), COALESCE(SUM(sugars),0),
		        COALESCE(SUM(fiber),0), COALESCE(SUM(salt),0), COALESCE(SUM(sodium),0)
		 FROM diary_entry WHERE day = ?`, day)
	var count int
	var n nutriments
	if err := row.Scan(&count, &n.Kcal, &n.Protein, &n.Fat, &n.SatFat, &n.Carbs, &n.Sugars, &n.Fiber, &n.Salt, &n.Sodium); err != nil {
		return nutriments{}, 0, err
	}
	return n, count, nil
}
