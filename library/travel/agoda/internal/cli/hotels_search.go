// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelHotelsSearchCmd(flags *rootFlags) *cobra.Command {
	sf := &searchFlags{}
	var advertisedOnly bool

	cmd := &cobra.Command{
		Use:   "search [destination]",
		Short: "Search hotels and show the true all-in price, not the teaser rate",
		Long: `Search Agoda for hotels and report what the stay will actually cost.

Agoda's API returns both the advertised price (before taxes and fees) and the
true all-in price in the same response, but its website renders only the
advertised one. This command reports both plus the gap between them, which on
live data routinely runs 20-30 percent.

Results keep Agoda's own ranking. Use 'hotels rank' to re-sort by real cost, and
'hotels fees' to find properties whose fee load is an outlier.`,
		Example: "  agoda-pp-cli hotels search Tokyo --checkin 2026-10-15 --nights 2 --adults 2 --currency USD --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "destination=Tokyo;--nights=2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hotels search")
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
			res, priced, err := runSearch(ctx, c, cmd.ErrOrStderr(), dest, sf, hasAgodaSession())
			if err != nil {
				return err
			}
			if advertisedOnly {
				res.Note = appendNote(res.Note, "showing Agoda's advertised ranking and price; all-in figures are still reported per property")
			}
			if strings.EqualFold(strings.TrimSpace(sf.sort), "true-price") {
				sortByAllIn(priced)
				res.Note = appendNote(res.Note, "sorted locally by true all-in cost")
			}
			priced = applyLimit(priced, sf.limit)
			res.Results = priced
			res.ReturnedProperties = len(priced)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), res, flags, "live")
			}
			return renderPropertyTable(cmd, res)
		},
	}
	bindSearchFlags(cmd, sf)
	cmd.Flags().BoolVar(&advertisedOnly, "advertised-order", false,
		"keep Agoda's advertised-price ordering without annotating the ranking as cost-ordered")
	return cmd
}

func appendNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}
