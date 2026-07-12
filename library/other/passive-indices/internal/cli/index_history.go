// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newIndexHistoryCmd(flags *rootFlags) *cobra.Command {
	var from, to string

	cmd := &cobra.Command{
		Use:     "history <name>",
		Short:   "Historical index price series (OHLC) for a date range",
		Example: "  passive-indices-pp-cli index history \"NIFTY 50\" --from 01-Jun-2026 --to 01-Jul-2026 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch historical index data")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required"))
			}
			fromT, toT, err := parseHistoryRange(from, to)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newNiftyIndicesClient(flags)
			rows, err := c.History(ctx, args[0], fromT, toT)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (DD-Mon-YYYY, e.g. 01-Jun-2026), defaults to 90 days before --to")
	cmd.Flags().StringVar(&to, "to", "", "end date (DD-Mon-YYYY, e.g. 01-Jul-2026), defaults to today")
	return cmd
}

const niftyDateLayout = "02-Jan-2006"

// parseHistoryRange parses --from/--to flags (DD-Mon-YYYY), defaulting --to
// to today and --from to 90 days before --to when omitted.
func parseHistoryRange(from, to string) (time.Time, time.Time, error) {
	toT := time.Now()
	if to != "" {
		parsed, err := time.Parse(niftyDateLayout, to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to date %q, expected DD-Mon-YYYY (e.g. 01-Jul-2026): %w", to, err)
		}
		toT = parsed
	}
	fromT := toT.AddDate(0, 0, -90)
	if from != "" {
		parsed, err := time.Parse(niftyDateLayout, from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from date %q, expected DD-Mon-YYYY (e.g. 01-Jun-2026): %w", from, err)
		}
		fromT = parsed
	}
	return fromT, toT, nil
}
