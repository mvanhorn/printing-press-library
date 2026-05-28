// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `budget` parent: show + reset.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Show paid-tier spend and reset spend tracking",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBudgetShowCmd(flags))
	cmd.AddCommand(newBudgetResetCmd(flags))
	return cmd
}

func newBudgetShowCmd(flags *rootFlags) *cobra.Command {
	var (
		flagByShow bool
		flagSince  int
	)
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show paid-tier spend (optionally pivoted by show)",
		Example: "  podcast-goat-pp-cli budget show\n" +
			"  podcast-goat-pp-cli budget show --by-show --since 30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.BudgetByShow(cmd.Context(), flagSince)
			if err != nil {
				return err
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no paid-tier spend recorded yet")
				return nil
			}
			if flagByShow {
				headers := []string{"show", "provider", "month", "episodes", "credits", "usd_estimate"}
				var data [][]string
				for _, r := range rows {
					data = append(data, []string{
						r.Show, r.Provider, r.Month,
						fmt.Sprintf("%d", r.Episodes),
						fmt.Sprintf("%.2f", r.TotalCredits),
						fmt.Sprintf("%.2f", r.TotalUSD),
					})
				}
				return flags.printTable(cmd, headers, data)
			}
			// Default: aggregate by provider+month
			type key struct{ Provider, Month string }
			agg := map[key]float64{}
			usd := map[key]float64{}
			eps := map[key]int{}
			for _, r := range rows {
				k := key{r.Provider, r.Month}
				agg[k] += r.TotalCredits
				usd[k] += r.TotalUSD
				eps[k] += r.Episodes
			}
			headers := []string{"provider", "month", "episodes", "credits", "usd_estimate"}
			// Sort keys before rendering — iterating a Go map yields a
			// runtime-randomized order, which makes table output
			// non-deterministic across runs and breaks both human diffing
			// and agent comparison. Sort newest-month-first, then provider.
			keys := make([]struct{ Provider, Month string }, 0, len(agg))
			for k := range agg {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				if keys[i].Month != keys[j].Month {
					return keys[i].Month > keys[j].Month
				}
				return keys[i].Provider < keys[j].Provider
			})
			var data [][]string
			for _, k := range keys {
				data = append(data, []string{
					k.Provider, k.Month,
					fmt.Sprintf("%d", eps[k]),
					fmt.Sprintf("%.2f", agg[k]),
					fmt.Sprintf("%.2f", usd[k]),
				})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	cmd.Flags().BoolVar(&flagByShow, "by-show", false, "Pivot rows by show+provider+month")
	cmd.Flags().IntVar(&flagSince, "since", 90, "Lookback window in days")
	return cmd
}

func newBudgetResetCmd(flags *rootFlags) *cobra.Command {
	var flagConfirm bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Clear spend log (destructive)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !flagConfirm && !flags.yes {
				return fmt.Errorf("`budget reset` is destructive; pass --confirm (or --yes)")
			}
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			if err := ps.ResetSpend(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "spend log cleared")
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Required confirmation flag")
	return cmd
}
