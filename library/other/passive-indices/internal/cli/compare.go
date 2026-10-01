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
		Short:       "See a fund's NAV/AUM/expense next to a requested index's level and top constituents.",
		Long:        "Use for a single fund vs. an index side-by-side. If the fund reports a benchmark, it must match the requested index; otherwise the result marks benchmark validation unavailable. For tracking funds ranked by disclosed expense ratio, use 'index tracking'; for plain membership, use 'index funds'.",
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
			benchmarkValidation := "fund_unavailable"
			if fundErr == nil {
				if err := validateBenchmarkIdentity(schemeID, fd.BenchmarkText, indexName); err != nil {
					return usageErr(err)
				}
				benchmarkValidation = "matched"
				if strings.TrimSpace(fd.BenchmarkText) == "" {
					benchmarkValidation = "not_reported"
				}
			}

			niftyClient := newNiftyIndicesClient(flags)
			quotes, indexErr := niftyClient.LiveWatch(ctx)

			if fundErr != nil && indexErr != nil {
				return fmt.Errorf("fetching fund: %w; fetching index: %v", fundErr, indexErr)
			}

			out := map[string]any{"scheme_id": schemeID, "index": indexName, "benchmark_validation": benchmarkValidation}
			var fetchFailures []map[string]string

			if fundErr != nil {
				fetchFailures = append(fetchFailures, map[string]string{"source": "fund", "error": fundErr.Error()})
			} else {
				out["fund"] = fundDetailToView(fd)
			}

			if indexErr != nil {
				fetchFailures = append(fetchFailures, map[string]string{"source": "index", "error": indexErr.Error()})
			} else {
				matchedQuote := findLiveQuote(quotes, indexName)
				if matchedQuote != nil {
					out["index_quote"] = matchedQuote
				} else {
					fetchFailures = append(fetchFailures, map[string]string{"source": "index", "error": fmt.Sprintf("no live quote found for index name %q", indexName)})
				}

				slug := constituentSlug(indexName, matchedQuote)
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
	name = normalizeIndexName(name)
	for {
		before := name
		for _, suffix := range []string{" total return index", " total return", " tri", " index"} {
			name = strings.TrimSuffix(name, suffix)
		}
		if name == before {
			break
		}
	}
	return strings.TrimSpace(name)
}

func normalizeIndexName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.NewReplacer("-", " ", "_", " ", "(", " ", ")", " ").Replace(name)
	for {
		before := name
		name = indexIdentityBoundaryRE.ReplaceAllString(name, "$1$3 $2$4")
		if name == before {
			break
		}
	}
	return strings.Join(strings.Fields(name), " ")
}

func validateBenchmarkIdentity(schemeID, benchmark, requested string) error {
	if strings.TrimSpace(benchmark) == "" || canonicalIndexIdentity(benchmark) == canonicalIndexIdentity(requested) {
		return nil
	}
	return fmt.Errorf("fund %s declares benchmark %q, which does not match requested index %q", schemeID, benchmark, requested)
}

func findLiveQuote(quotes []niftyindices.LiveQuote, requested string) *niftyindices.LiveQuote {
	for i := range quotes {
		if strings.EqualFold(strings.TrimSpace(quotes[i].IndexName), strings.TrimSpace(requested)) {
			return &quotes[i]
		}
	}
	target := canonicalQuoteIdentity(requested)
	for i := range quotes {
		if canonicalQuoteIdentity(quotes[i].IndexName) == target {
			return &quotes[i]
		}
	}
	return nil
}

// Keep price and total-return quotes distinct even when their names use
// different spacing or abbreviations. Benchmark validation can accept either.
func canonicalQuoteIdentity(name string) string {
	normalized := normalizeIndexName(name)
	core := canonicalIndexIdentity(name)
	for _, suffix := range []string{" tri", " tri index", " total return", " total return index"} {
		if strings.HasSuffix(normalized, suffix) {
			return core + " tri"
		}
	}
	return core
}

func constituentSlug(requested string, matched *niftyindices.LiveQuote) string {
	if matched != nil {
		// A TRI reports total-return levels for the base index's basket. The
		// provider normally publishes that basket under the base-index slug.
		if strings.HasSuffix(canonicalQuoteIdentity(matched.IndexName), " tri") {
			return niftyindices.Slugify(canonicalIndexIdentity(matched.IndexName))
		}
		return niftyindices.Slugify(matched.IndexName)
	}
	return niftyindices.Slugify(requested)
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
