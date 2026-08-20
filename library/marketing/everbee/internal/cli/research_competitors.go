// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

func newNovelResearchCompetitorsCmd(flags *rootFlags) *cobra.Command {
	rf := &researchFlags{}

	cmd := &cobra.Command{
		Use:   "competitors [seed]",
		Short: "Get the market shape of a niche: result count, median price, review and sales density, listing-age quartiles.",
		Long: strings.Trim(`
Size up who already competes in a niche before committing any design work.

Runs EverBee's seeded product search and computes market-shape statistics over
the listings that are both relevant to the seed and of the requested product
type: how many listings exist, the price band and median, mean reviews and
estimated monthly sales per listing, listing-age quartiles, and the shops holding
the most revenue in the niche.

The statistics are computed locally from the rows, and the rows are returned
alongside them, so every number can be checked rather than taken on faith.

Use this command for market-shape stats about who you would compete with. For a
buy/skip verdict on the niche itself, use 'research niche' instead.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli research competitors "dad shirt" --agent
  everbee-pp-cli research competitors "dad shirt" --product apparel --limit 50 --json
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
				fmt.Fprintln(cmd.OutOrStdout(), "would sample competing listings and compute market-shape statistics")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a seed keyword is required, e.g. research competitors \"dad shirt\""))
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

			products, listingCount, prov, err := research.FetchProducts(ctx, c, seed, opts.PerPage)
			if err != nil {
				return wrapResearchErr(err)
			}

			sample := research.SampleCompetitors(seed, products, listingCount, opts)
			sample.Provenance = append(sample.Provenance, prov)

			return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
		},
	}
	cmd.Flags().StringVar(&rf.productType, "product", "", "Constrain evidence to a product type: physical, digital, or apparel (default: any)")
	cmd.Flags().BoolVar(&rf.excludeSVGPNG, "exclude-svg-png", false, "Exclude digital cut-file listings (SVG/PNG/Cricut) from the evidence count")
	cmd.Flags().Float64Var(&rf.minRelevance, "min-relevance", research.DefaultMinRelevance, "Relevance floor (0-1) a row must clear to count as evidence. Rows below it are still returned and annotated, never dropped.")
	cmd.Flags().IntVar(&rf.perPage, "limit", 20, "Number of listings to fetch as product evidence")
	return cmd
}
