// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: search constrained to remaining daily kcal budget.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type budgetResult struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kcal100g   float64 `json:"kcal_100g"`
	NutriScore string  `json:"nutriscore_grade"`
}

type budgetReport struct {
	Query         string         `json:"query"`
	GoalKcal      float64        `json:"goal_kcal"`
	LoggedKcal    float64        `json:"logged_kcal_today"`
	RemainingKcal float64        `json:"remaining_kcal"`
	Results       []budgetResult `json:"results"`
}

func newNovelBudgetCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "budget <query>",
		Short:       "Search constrained to the macros left in today's goal after what you've logged.",
		Long:        "Compute the kcal remaining in today's goal (goal minus what the diary has logged today), then search the given category and keep only products whose energy per 100g fits the remaining budget. Requires a goal set with `diary goal --kcal`.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli budget biscuits\n  openfoodfacts-pp-cli budget snacks --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("a category query is required\nUsage: %s <query>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]

			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			goal, ok, err := loadGoal(cmd.Context(), db.DB())
			if err != nil {
				db.Close()
				return err
			}
			if !ok || goal.Kcal <= 0 {
				db.Close()
				return configErr(fmt.Errorf("no kcal goal set — run `diary goal --kcal 2000` first"))
			}
			logged, _, err := sumDiaryDay(cmd.Context(), db.DB(), time.Now().Format("2006-01-02"))
			db.Close()
			if err != nil {
				return err
			}
			remaining := goal.Kcal - logged.Kcal

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			products, err := offSearch(cmd.Context(), c, map[string]string{
				"categories_tags_en": query,
				"sort_by":            "nutriscore_score",
				"page_size":          "50",
				"fields":             "code,product_name,nutriscore_grade,nutriments",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			report := budgetReport{Query: query, GoalKcal: goal.Kcal, LoggedKcal: logged.Kcal, RemainingKcal: remaining, Results: []budgetResult{}}
			for _, p := range products {
				kcal := nutrFromObject(p).Kcal
				if kcal <= 0 || kcal > remaining {
					continue
				}
				report.Results = append(report.Results, budgetResult{
					Code:       prodString(p, "code"),
					Name:       prodString(p, "product_name"),
					Kcal100g:   kcal,
					NutriScore: prodString(p, "nutriscore_grade"),
				})
			}
			sort.SliceStable(report.Results, func(i, j int) bool { return report.Results[i].Kcal100g < report.Results[j].Kcal100g })
			if flagLimit > 0 && len(report.Results) > flagLimit {
				report.Results = report.Results[:flagLimit]
			}

			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%.0f kcal remaining today (goal %.0f − logged %.0f); %q fitting:\n",
				remaining, goal.Kcal, logged.Kcal, query)
			if len(report.Results) == 0 {
				fmt.Fprintln(out, "  (no products fit the remaining budget)")
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tNAME\tKCAL/100g\tNUTRI")
			for _, r := range report.Results {
				fmt.Fprintf(tw, "%s\t%s\t%.0f\t%s\n", r.Code, truncate(r.Name, 36), r.Kcal100g, strings.ToUpper(grade(r.NutriScore)))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of results")
	return cmd
}
