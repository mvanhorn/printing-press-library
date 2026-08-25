// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelOtcWeeklyCmd(flags *rootFlags) *cobra.Command {
	var flagSymbol string
	var flagSince string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:         "weekly",
		Short:       "OTC weekly summary volume records for a symbol over a trailing window",
		Long:        "OTC weekly summary volume records for --symbol over the trailing --since window (default 90d).",
		Example:     "--symbol GME --since 90d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--symbol=AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s weekly-summary records for %s over the trailing %s\n", flagGroup, flagName, flagSymbol, flagSince)
				return nil
			}
			if strings.TrimSpace(flagSymbol) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--symbol is required"))
			}

			since := flagSince
			if strings.TrimSpace(since) == "" {
				since = "90d"
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			start := now.Add(-window)
			path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", flagGroup), "name", flagName)
			body := map[string]any{
				// issueSymbolIdentifier and weekStartDate are this dataset's
				// confirmed symbol and partition fields; a live probe
				// confirmed the API filters on both server-side (an
				// unfiltered fetch bounded only by --limit rarely contains
				// the requested symbol at all, since results are not sorted
				// by symbol).
				"compareFilters":   equalCompareFilter("issueSymbolIdentifier", flagSymbol),
				"dateRangeFilters": dateRangeFilter("weekStartDate", start, now),
				"limit":            1000,
			}
			data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			records, err := parseDatasetRecords(data)
			if err != nil {
				return apiErr(fmt.Errorf("parsing %s/%s response: %w", flagGroup, flagName, err))
			}

			view := otcWeeklyView{
				Symbol:  flagSymbol,
				Since:   since,
				Records: records,
				Count:   len(records),
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d weekly record(s) over the trailing %s\n", view.Symbol, view.Count, view.Since)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSymbol, "symbol", "", "Security symbol to pull OTC weekly summary records for (required)")
	cmd.Flags().StringVar(&flagSince, "since", "90d", "How far back to look for weekly summary records (e.g. 30d, 90d, 6w)")
	cmd.Flags().StringVar(&flagGroup, "group", "OTCMARKET", "Dataset group for OTC Weekly Summary (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "WEEKLYSUMMARY", "Dataset name for OTC Weekly Summary (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type otcWeeklyView struct {
	Symbol  string           `json:"symbol"`
	Since   string           `json:"since"`
	Records []map[string]any `json:"records"`
	Count   int              `json:"count"`
}
