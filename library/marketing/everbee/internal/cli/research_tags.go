// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

func newNovelResearchTagsCmd(flags *rootFlags) *cobra.Command {
	rf := &researchFlags{}

	cmd := &cobra.Command{
		Use:   "tags [seed]",
		Short: "See which tags and title tokens the winning listings in a niche agree on, and whether demand is seasonal or evergreen.",
		Long: strings.Trim(`
Report the consensus vocabulary of a niche and whether its demand survives the holiday.

Aggregates the tags and title words across the listings that are relevant to the
seed and of the requested product type, ranked by how many of those listings use
each one. This is the same derivation EverBee's own Tag Analytics tab performs —
tags ride on every product row, and the tab fires no network request — made
scriptable here.

Seasonality is judged from the variance of the seed's search-volume trend against
a documented threshold. When EverBee returns too few trend points to judge, the
verdict is "unknown" rather than a guess.

Use this command for tag/title consensus and seasonality of a seed's listings.
For the overall opportunity verdict, use 'research niche' instead.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli research tags "dad shirt" --agent
  everbee-pp-cli research tags "dad shirt" --product apparel --exclude-svg-png --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "seed=dad shirt",
			// A nonsense seed is a valid search that finds nothing, not an error:
			// EverBee answers 200 and the command returns an honest zero-evidence
			// result with exit 0. Telling bad input apart from a valid empty result
			// would mean inventing API semantics EverBee does not have.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate tag/title consensus and classify seasonality")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a seed keyword is required, e.g. research tags \"dad shirt\""))
			}
			if err := rf.validate(); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			seed := strings.Join(args, " ")
			opts := rf.options(seed)

			products, _, prProv, err := research.FetchProducts(ctx, c, seed, opts.PerPage)
			if err != nil {
				return wrapResearchErr(err)
			}

			// Seasonality needs the seed's trend series, which rides on the
			// keyword response. A keyword failure degrades the seasonality call
			// to "unknown" rather than aborting the tag consensus.
			var trend []float64
			kwRows, _, kwProv, kwErr := research.FetchKeywords(ctx, c, seed, 1)
			if kwErr != nil {
				if isPlanCapErr(kwErr) {
					return wrapResearchErr(kwErr)
				}
				kwProv.Fallback = "keyword search failed; seasonality is unknown"
			} else {
				trend = research.SeedTrend(kwRows, seed)
			}

			tc := research.ConsensusTags(seed, products, opts, trend)
			tc.Provenance = append(tc.Provenance, prProv, kwProv)
			if kwErr != nil {
				tc.Warnings = append(tc.Warnings, fmt.Sprintf("seasonality unavailable: %v", kwErr))
			}

			return printJSONFiltered(cmd.OutOrStdout(), tc, flags)
		},
	}
	cmd.Flags().StringVar(&rf.productType, "product", "", "Constrain evidence to a product type: physical, digital, or apparel (default: any)")
	cmd.Flags().BoolVar(&rf.excludeSVGPNG, "exclude-svg-png", false, "Exclude digital cut-file listings (SVG/PNG/Cricut) from the evidence count")
	cmd.Flags().Float64Var(&rf.minRelevance, "min-relevance", research.DefaultMinRelevance, "Relevance floor (0-1) a row must clear to count as evidence. Rows below it are still returned and annotated, never dropped.")
	cmd.Flags().IntVar(&rf.perPage, "limit", 20, "Number of listings to fetch as product evidence")
	return cmd
}
