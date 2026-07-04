// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type revenueGroup struct {
	Group     string  `json:"group"`
	Jobs      int     `json:"jobs"`
	Total     float64 `json:"total"`
	AmountDue float64 `json:"amount_due"`
}

func newNovelJobRevenueCmd(flags *rootFlags) *cobra.Command {
	var flagGroupBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "revenue",
		Short:       "Roll up total and outstanding job value by lead source and job status.",
		Long:        "Use this for dollar-value rollups by source/status. For lead-to-job conversion counts/rates, use 'lead funnel' instead.",
		Example:     "  workiz-pp-cli job revenue --group-by source --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would roll up job revenue from the local mirror")
				return nil
			}
			groupBy := flagGroupBy
			if groupBy == "" {
				groupBy = "source"
			}
			if groupBy != "source" && groupBy != "status" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be \"source\" or \"status\", got %q", groupBy))
			}
			ctx := cmd.Context()
			var bail bool
			if dbPath, bail = checkNovelMirror(cmd, flags, dbPath, "job", []revenueGroup{}); bail {
				return nil
			}
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			jobs, err := loadJobs(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading jobs: %w", err)
			}

			type agg struct {
				jobs      int
				total     float64
				amountDue float64
			}
			byGroup := map[string]*agg{}
			for _, j := range jobs {
				key := j.JobSource
				if groupBy == "status" {
					key = j.Status
				}
				if key == "" {
					key = "(unknown)"
				}
				a, ok := byGroup[key]
				if !ok {
					a = &agg{}
					byGroup[key] = a
				}
				a.jobs++
				a.total += parseMoney(j.JobTotalPrice)
				a.amountDue += parseMoney(j.JobAmountDue)
			}

			groups := make([]string, 0, len(byGroup))
			for g := range byGroup {
				groups = append(groups, g)
			}
			sort.Strings(groups)

			results := make([]revenueGroup, 0, len(groups))
			for _, g := range groups {
				a := byGroup[g]
				results = append(results, revenueGroup{Group: g, Jobs: a.jobs, Total: a.total, AmountDue: a.amountDue})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].Total > results[j].Total })

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d jobs\t$%.2f total\t$%.2f due\n", r.Group, r.Jobs, r.Total, r.AmountDue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "source", "Group revenue by \"source\" (JobSource) or \"status\" (job Status)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/workiz-pp-cli/data.db)")
	return cmd
}
