// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
)

// rankedResult extends the search envelope with the ranking movement, which is
// the whole point of the command: it shows that ordering by real cost is not the
// same as ordering by the advertised price.
type rankedResult struct {
	agodaSearchResult
	RankChanges int    `json:"rank_changes"`
	RankNote    string `json:"rank_note,omitempty"`
}

func newNovelHotelsRankCmd(flags *rootFlags) *cobra.Command {
	sf := &searchFlags{}

	cmd := &cobra.Command{
		Use:   "rank [destination]",
		Short: "Re-rank results by true all-in price instead of Agoda's teaser ranking",
		Long: `Re-sort a destination's hotels by what they actually cost.

Agoda's hidden tax-and-fee markup is not a flat percentage: on a single live
search it ranged from 21 to over 30 percent across properties. That means the
hotel listed cheapest is frequently not the cheapest once fees land, and sorting
by the advertised price actively misleads.

This command holds both price bases for the whole result set, sorts by the
all-in figure, and reports how many properties changed position.`,
		Example: "  agoda-pp-cli hotels rank Tokyo --checkin 2026-10-15 --nights 2 --limit 10 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "destination=Tokyo;--nights=2;--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hotels rank")
			}
			dest := ""
			if len(args) > 0 {
				dest = args[0]
			}
			if dest == "" && sf.cityID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a destination argument or --city-id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newAgodaClient(flags)
			base, priced, err := runSearch(ctx, c, cmd.ErrOrStderr(), dest, sf, hasAgodaSession())
			if err != nil {
				return err
			}

			// Capture the advertised ordering before re-sorting so the movement
			// count is measured, not asserted.
			advertisedOrder := make([]int, len(priced))
			for i, p := range priced {
				advertisedOrder[i] = p.PropertyID
			}
			sortByAllIn(priced)
			changes := countRankChanges(advertisedOrder, priced)

			priced = applyLimit(priced, sf.limit)
			out := rankedResult{agodaSearchResult: base}
			out.Results = priced
			out.ReturnedProperties = len(priced)
			out.RankChanges = changes
			if changes > 0 {
				out.RankNote = fmt.Sprintf("%d properties changed position once taxes and fees were included", changes)
			} else if len(priced) > 0 {
				out.RankNote = "all-in ordering matched the advertised ordering for this search"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), out, flags, "live")
			}
			if err := renderPropertyTable(cmd, out.agodaSearchResult); err != nil {
				return err
			}
			if out.RankNote != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", out.RankNote)
			}
			return nil
		},
	}
	bindSearchFlags(cmd, sf)
	return cmd
}

// countRankChanges counts properties whose index differs between the advertised
// ordering and the all-in ordering. It is the measured evidence behind this
// command's claim, not an assumption.
func countRankChanges(before []int, after []agoda.Property) int {
	pos := make(map[int]int, len(before))
	for i, id := range before {
		pos[id] = i
	}
	changed := 0
	for i, p := range after {
		if was, ok := pos[p.PropertyID]; ok && was != i {
			changed++
		}
	}
	return changed
}
