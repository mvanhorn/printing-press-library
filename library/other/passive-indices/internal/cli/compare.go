// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/niftyindices"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var topN int

	cmd := &cobra.Command{
		Use:         "compare <schemeId> <index>",
		Short:       "See a single fund's NAV/AUM/expense next to its benchmark index's level and top constituents, side by side.",
		Long:        "Use for a single fund vs. an index side-by-side. If the fund reports a benchmark, it must match the requested index. For tracking funds ranked by disclosed expense ratio, use 'index tracking'; for plain membership, use 'index funds'.",
		Example:     "  passive-indices-pp-cli compare 1150 \"NIFTY 50\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both schemeId and index name are required, e.g. compare 1150 \"NIFTY 50\""))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare fund against index")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			schemeID, indexName := args[0], args[1]

			fundClient := newIndiaPassiveFundsClient(flags)
			fd, fundErr := fundClient.FundDetail(ctx, schemeID)
			if fundErr == nil {
				if err := validateBenchmarkIdentity(schemeID, fd.BenchmarkText, indexName); err != nil {
					return usageErr(err)
				}
			}

			niftyClient := newNiftyIndicesClient(flags)
			quotes, indexErr := niftyClient.LiveWatch(ctx)

			if fundErr != nil && indexErr != nil {
				return fmt.Errorf("fetching fund: %w; fetching index: %v", fundErr, indexErr)
			}

			out := map[string]any{"scheme_id": schemeID, "index": indexName}
			var fetchFailures []map[string]string

			if fundErr != nil {
				fetchFailures = append(fetchFailures, map[string]string{"source": "fund", "error": fundErr.Error()})
			} else {
				out["fund"] = fundDetailToView(fd)
			}

			if indexErr != nil {
				fetchFailures = append(fetchFailures, map[string]string{"source": "index", "error": indexErr.Error()})
			} else {
				var matchedQuote *niftyindices.LiveQuote
				matchedQuote = findLiveQuote(quotes, indexName)
				if matchedQuote != nil {
					out["index_quote"] = matchedQuote
				} else {
					fetchFailures = append(fetchFailures, map[string]string{"source": "index", "error": fmt.Sprintf("no live quote found for index name %q", indexName)})
				}

				slug := niftyindices.Slugify(indexName)
				constituents, err := niftyClient.Constituents(ctx, slug)
				fetchFailures = addConstituentResult(out, fetchFailures, constituents, err, topN)
			}

			if len(fetchFailures) > 0 {
				out["fetch_failures"] = fetchFailures
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&topN, "constituents-sample", 10, "how many index constituents to include in the comparison")
	return cmd
}

var indexIdentityBoundaryRE = regexp.MustCompile(`([a-z])(\d)|(\d)([a-z])`)

func canonicalIndexIdentity(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, " total return index")
	name = strings.TrimSuffix(name, " tri")
	name = indexIdentityBoundaryRE.ReplaceAllString(name, "$1$3 $2$4")
	return strings.Join(strings.Fields(name), " ")
}

func validateBenchmarkIdentity(schemeID, benchmark, requested string) error {
	if strings.TrimSpace(benchmark) == "" || canonicalIndexIdentity(benchmark) == canonicalIndexIdentity(requested) {
		return nil
	}
	return fmt.Errorf("fund %s declares benchmark %q, which does not match requested index %q", schemeID, benchmark, requested)
}

func findLiveQuote(quotes []niftyindices.LiveQuote, requested string) *niftyindices.LiveQuote {
	target := canonicalIndexIdentity(requested)
	for i := range quotes {
		if canonicalIndexIdentity(quotes[i].IndexName) == target {
			return &quotes[i]
		}
	}
	return nil
}

func addConstituentResult(out map[string]any, failures []map[string]string, constituents []niftyindices.ConstituentRow, err error, topN int) []map[string]string {
	if err != nil {
		return append(failures, map[string]string{"source": "index_constituents", "error": err.Error()})
	}
	if topN > 0 && len(constituents) > topN {
		constituents = constituents[:topN]
	}
	out["index_constituents_sample"] = constituents
	return failures
}
