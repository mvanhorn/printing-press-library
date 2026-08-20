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
func newNovelRegshoVolumeCmd(flags *rootFlags) *cobra.Command {
	var flagSymbol string
	var flagSince string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Reg SHO daily short sale volume records for a symbol over a trailing window",
		Long: "Reg SHO daily short sale volume records for --symbol over the trailing --since window (default 30d).\n\n" +
			"When a returned record has both shortParQuantity and totalParQuantity, a short_volume_ratio\n" +
			"is computed per record.",
		Example:     "--symbol GME --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--symbol=AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s short-volume records for %s over the trailing %s\n", flagGroup, flagName, flagSymbol, flagSince)
				return nil
			}
			if strings.TrimSpace(flagSymbol) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--symbol is required"))
			}

			since := flagSince
			if strings.TrimSpace(since) == "" {
				since = "30d"
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
				// securitiesInformationProcessorSymbolIdentifier and
				// tradeReportDate are this dataset's confirmed symbol and
				// partition fields; a live probe confirmed the API filters
				// on both server-side (an unfiltered fetch bounded only by
				// --limit rarely contains the requested symbol at all, since
				// results are not sorted by symbol).
				"compareFilters":   equalCompareFilter("securitiesInformationProcessorSymbolIdentifier", flagSymbol),
				"dateRangeFilters": dateRangeFilter("tradeReportDate", start, now),
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

			view := regshoVolumeView{
				Symbol: flagSymbol,
				Since:  since,
				Count:  len(records),
			}
			for _, rec := range records {
				view.Records = append(view.Records, decorateShortVolumeRatio(rec))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d record(s) over the trailing %s\n", view.Symbol, view.Count, view.Since)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSymbol, "symbol", "", "Security symbol to pull Reg SHO short-volume records for (required)")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "How far back to look for short-volume records (e.g. 24h, 30d, 4w)")
	cmd.Flags().StringVar(&flagGroup, "group", "OTCMARKET", "Dataset group for Reg SHO Daily Short Sale Volume (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "REGSHODAILY", "Dataset name for Reg SHO Daily Short Sale Volume (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type regshoVolumeView struct {
	Symbol  string           `json:"symbol"`
	Since   string           `json:"since"`
	Records []map[string]any `json:"records"`
	Count   int              `json:"count"`
}

// decorateShortVolumeRatio returns a copy of rec with a short_volume_ratio
// field added when both shortParQuantity and totalParQuantity are present
// numeric fields. These are the confirmed field names for this dataset —
// the API reports "ParQuantity", not "Volume", so short_volume_ratio is
// computed from shortParQuantity/totalParQuantity directly rather than by
// scanning for keys containing "short"/"total" and "volume".
func decorateShortVolumeRatio(rec map[string]any) map[string]any {
	shortVol, haveShort := rec["shortParQuantity"].(float64)
	totalVol, haveTotal := rec["totalParQuantity"].(float64)
	if !haveShort || !haveTotal || totalVol == 0 {
		return rec
	}
	out := make(map[string]any, len(rec)+1)
	for k, v := range rec {
		out[k] = v
	}
	out["short_volume_ratio"] = shortVol / totalVol
	return out
}
