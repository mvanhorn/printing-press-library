// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

// researchFlags are shared by every research command so the evidence contract
// is expressed identically everywhere.
type researchFlags struct {
	productType   string
	excludeSVGPNG bool
	minRelevance  float64
	perPage       int
}

func (rf *researchFlags) options(seed string) research.Options {
	return research.Options{
		Seed:          seed,
		ProductType:   rf.productType,
		ExcludeSVGPNG: rf.excludeSVGPNG,
		MinRelevance:  rf.minRelevance,
		PerPage:       rf.perPage,
	}
}

// validateProductType rejects an unrecognized --product value up front rather
// than silently returning zero evidence, which would read as "no opportunity".
func (rf *researchFlags) validate() error {
	switch strings.ToLower(strings.TrimSpace(rf.productType)) {
	case "", "any", "physical", "digital", "apparel":
	default:
		return fmt.Errorf("--product %q is not recognized; use physical, digital, or apparel", rf.productType)
	}
	if rf.minRelevance < 0 || rf.minRelevance > 1 {
		return fmt.Errorf("--min-relevance must be between 0 and 1, got %v", rf.minRelevance)
	}
	return nil
}

func newNovelResearchNicheCmd(flags *rootFlags) *cobra.Command {
	rf := &researchFlags{}

	cmd := &cobra.Command{
		Use:   "niche [seed]",
		Short: "Score an Etsy niche from a seed keyword and get the evidence behind the score, not just the number.",
		Long: strings.Trim(`
Score an Etsy niche from a seed keyword and return the evidence behind the score.

Runs EverBee's seeded keyword search and seeded product search for the seed, then
scores demand, competition, saturation, price band, and an opportunity score
locally. Every returned row is annotated with its relevance to the seed and every
metric carries its provenance, so a "low competition" claim can be audited rather
than trusted. Confidence tracks evidence coverage: it is capped when there is no
keyword evidence and is 0 when nothing relevant came back.

Rows are never dropped. Use --min-relevance to set the floor a row must clear to
count as evidence; rows below it are still returned, marked with their score.

Use this command for a single-seed niche verdict with evidence. Do NOT use it to
enumerate child niches; use 'research subniches' instead. For market-shape stats
alone, use 'research competitors'.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli research niche "dad shirt" --agent
  everbee-pp-cli research niche "dad shirt" --product apparel --exclude-svg-png --json
  everbee-pp-cli research niche "wedding sign" --min-relevance 0.75 --limit 50 --json
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
				fmt.Fprintln(cmd.OutOrStdout(), "would run seeded keyword + product search and score the niche")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a seed keyword is required, e.g. research niche \"dad shirt\""))
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
			verdict, err := research.Niche(ctx, c, rf.options(seed))
			if err != nil {
				return wrapResearchErr(err)
			}
			return emitVerdict(cmd, flags, verdict)
		},
	}
	cmd.Flags().StringVar(&rf.productType, "product", "", "Constrain evidence to a product type: physical, digital, or apparel (default: any)")
	cmd.Flags().BoolVar(&rf.excludeSVGPNG, "exclude-svg-png", false, "Exclude digital cut-file listings (SVG/PNG/Cricut) from the evidence count")
	cmd.Flags().Float64Var(&rf.minRelevance, "min-relevance", research.DefaultMinRelevance, "Relevance floor (0-1) a row must clear to count as evidence. Rows below it are still returned and annotated, never dropped.")
	cmd.Flags().IntVar(&rf.perPage, "limit", 20, "Number of listings to fetch as product evidence")
	return cmd
}

// wrapResearchErr turns a plan-cap refusal into an actionable message. A quota
// refusal and an empty result are different facts and must never look alike.
func wrapResearchErr(err error) error {
	if err == nil {
		return nil
	}
	if isPlanCapErr(err) {
		return fmt.Errorf("%w\nCheck remaining quota with: everbee-pp-cli account --json", err)
	}
	return err
}

func isPlanCapErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), research.ErrPlanCapReached.Error())
}

func emitVerdict(cmd *cobra.Command, flags *rootFlags, v *research.Verdict) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printVerdictHuman(cmd, v)
	}
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

func printVerdictHuman(cmd *cobra.Command, v *research.Verdict) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Niche: %s\n", v.Seed)
	if v.ProductType != "" {
		fmt.Fprintf(w, "Product type: %s\n", v.ProductType)
	}
	fmt.Fprintf(w, "Opportunity: %.0f/100   Confidence: %.2f\n", v.Opportunity, v.Confidence)
	if v.SeedMetrics.Present {
		sat := "undefined (no demand)"
		if v.Saturation != nil {
			sat = fmt.Sprintf("%.1f", *v.Saturation)
		}
		fmt.Fprintf(w, "Demand: %d searches   Competition: %d listings   Saturation: %s\n",
			v.Demand, v.Competition, sat)
	} else {
		fmt.Fprintln(w, "Demand/Competition: not reported by EverBee for this seed")
	}
	if v.PriceBand.Count > 0 {
		fmt.Fprintf(w, "Price band: %.2f - %.2f (median %.2f from %d listings)\n",
			v.PriceBand.Min, v.PriceBand.Max, v.PriceBand.Median, v.PriceBand.Count)
	}
	fmt.Fprintf(w, "Evidence: %d/%d keywords, %d/%d listings relevant",
		v.Evidence.KeywordsRelevant, v.Evidence.KeywordsReturned,
		v.Evidence.ProductsRelevant, v.Evidence.ProductsReturned)
	if v.ProductType != "" {
		fmt.Fprintf(w, " (%d in type)", v.Evidence.ProductsInType)
	}
	fmt.Fprintln(w)
	for _, warn := range v.Warnings {
		fmt.Fprintf(w, "  - %s\n", warn)
	}
	return nil
}
