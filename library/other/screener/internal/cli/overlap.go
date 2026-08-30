// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/screener/internal/client"

	"text/tabwriter"

	"github.com/spf13/cobra"
)

type overlapResult struct {
	Screens  []string    `json:"screens"`
	Matches  []screenRow `json:"matches"`
	Required int         `json:"required_screens"`
}

func newNovelOverlapCmd(flags *rootFlags) *cobra.Command {
	var flagMin int
	var flagPage int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "overlap <screen_id> <screen_slug> [screen_id screen_slug...]",
		Short:       "Find companies that appear in two or more stock screens in one command, replacing spreadsheet dedup.",
		Long:        "Use this command to find companies that appear in multiple screens. Do NOT use it to re-score a single screen; use 'rank' instead.",
		Example:     "  screener-pp-cli overlap magic-formula bull-cartel --agent\n  screener-pp-cli overlap 59 magic-formula 1 the-bull-cartel --min 2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "1;the-bull-cartel;59;magic-formula"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "overlap")
			}
			if len(args) < 2 || len(args)%2 != 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("overlap requires screen id/slug pairs, e.g. 'overlap 1 the-bull-cartel 59 magic-formula'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			min := flagMin
			if min <= 0 {
				min = 2
			}
			page := flagPage
			if page <= 0 {
				page = 1
			}
			type screenFetch struct {
				name string
				rows []screenRow
				err  error
			}
			fetches := make([]screenFetch, 0, len(args)/2)
			for i := 0; i+1 < len(args); i += 2 {
				id := strings.TrimSpace(args[i])
				slug := strings.TrimSpace(args[i+1])
				rows, err := fetchScreenRows(ctx, c, id, slug, page)
				name := slug
				if name == "" {
					name = id
				}
				fetches = append(fetches, screenFetch{name: name, rows: rows, err: err})
			}
			counts := map[string]int{}
			rowsBySym := map[string]screenRow{}
			screenNames := make([]string, 0, len(fetches))
			for _, f := range fetches {
				if f.err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: screen %s failed: %v\n", f.name, f.err)
					continue
				}
				screenNames = append(screenNames, f.name)
				for _, row := range f.rows {
					if row.Symbol == "" {
						continue
					}
					counts[row.Symbol]++
					rowsBySym[row.Symbol] = row
				}
			}
			matches := make([]screenRow, 0)
			for sym, n := range counts {
				if n >= min {
					matches = append(matches, rowsBySym[sym])
				}
			}
			sort.Slice(matches, func(i, j int) bool {
				return matches[i].MarketCap > matches[j].MarketCap
			})
			out := overlapResult{Screens: screenNames, Matches: matches, Required: min}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No companies appear in", min, "or more of the requested screens.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "Name\tCMP\tP/E\tMarCap\tNP Qtr\tNP%\tSales Qtr\tROCE")
			for _, r := range matches {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Name, fnum(r.CMP), fnum(r.PE), fnum(r.MarketCap), fnum(r.NPQtr), pct(r.QtrProfitVar), fnum(r.SalesQtr), fnum(r.ROCE))
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&flagMin, "min", 2, "Minimum number of screens a company must appear in")
	cmd.Flags().IntVar(&flagPage, "page", 1, "Screen result page to intersect (1 = first 25 rows)")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (unused in live mode)")
	return cmd
}

// fetchScreenRows fetches a screen's result table rows.
func fetchScreenRows(ctx context.Context, c *client.Client, id, slug string, page int) ([]screenRow, error) {
	path := fmt.Sprintf("/screens/%s/%s/", id, slug)
	params := map[string]string{}
	if page > 1 {
		params["page"] = fmt.Sprintf("%d", page)
	}
	data, err := getWithRateRetry(ctx, c, path, params)
	if err != nil {
		return nil, err
	}
	return parseScreenTable(string(data)), nil
}
