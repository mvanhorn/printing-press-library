// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

// subNicheResult is the batch envelope. fetch_failures is always present (as an
// empty list when nothing failed) so a caller can tell "nothing failed" from
// "the field is missing".
type subNicheResult struct {
	Parent        string                `json:"parent"`
	ProductWord   string                `json:"product_word,omitempty"`
	ProductType   string                `json:"product_type,omitempty"`
	ChildrenFound int                   `json:"children_found"`
	Ranked        []research.SubNiche   `json:"ranked"`
	FetchFailures []fetchFailure        `json:"fetch_failures"`
	Provenance    []research.Provenance `json:"provenance"`
	Warnings      []string              `json:"warnings"`
}

type fetchFailure struct {
	Seed  string `json:"seed"`
	Error string `json:"error"`
}

func newNovelResearchSubnichesCmd(flags *rootFlags) *cobra.Command {
	rf := &researchFlags{}
	var parent string
	var productWord string
	var maxChildren int

	cmd := &cobra.Command{
		Use:   "subniches",
		Short: "Expand a parent niche into child niches and rank them on comparable, normalized scores.",
		Long: strings.Trim(`
Expand a parent niche into child niches and rank them against each other.

Child seeds are drawn from EverBee's own keyword suggestions for the parent, so
they are terms Etsy shoppers actually search rather than invented permutations.
Each child runs through the same evidence-aware pipeline as 'research niche',
then scores are normalized across the batch so the children are genuinely
comparable to one another.

Product-type constraints matter most here: --product apparel --exclude-svg-png
keeps digital cut-file listings from polluting an apparel batch.

Use this command to rank many child niches under one parent. For a deep verdict
on a single niche, use 'research niche' instead.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli research subniches --parent dad --product apparel --exclude-svg-png --agent
  everbee-pp-cli research subniches --parent dad --product-word shirt --max-children 15 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--parent=dad",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would expand the parent seed into child niches and score each")
				return nil
			}
			if strings.TrimSpace(parent) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--parent is required, e.g. --parent dad"))
			}
			if err := rf.validate(); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if maxChildren < 1 {
				maxChildren = 10
			}
			// A live-dogfood pass must fit the matrix timeout; a full batch of
			// children is a real network fan-out.
			if cliutil.IsDogfoodEnv() && maxChildren > 2 {
				maxChildren = 2
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			out := subNicheResult{
				Parent:        parent,
				ProductWord:   productWord,
				ProductType:   rf.productType,
				FetchFailures: make([]fetchFailure, 0),
				Ranked:        make([]research.SubNiche, 0),
				Warnings:      make([]string, 0),
			}

			// Step 1: ask EverBee which child keywords actually exist under the parent.
			//
			// When a product word is given, discovery is seeded with "<parent>
			// <product>" rather than the bare parent. EverBee's suggestions for
			// "dad" are overwhelmingly not about shirts, so seeding with "dad"
			// and then filtering for "shirt" discards nearly everything. Seeding
			// with "dad shirt" is what a user types, and it is what makes the
			// suggestion engine return shirt sub-niches in the first place.
			discoverySeed := strings.TrimSpace(parent + " " + productWord)

			rows, _, prov, err := research.FetchKeywords(ctx, c, discoverySeed, 1)
			if err != nil {
				return wrapResearchErr(err)
			}
			out.Provenance = append(out.Provenance, prov)

			// Children must be relevant to the parent concept; the product word
			// is additionally required when one was given.
			children := research.ChildSeeds(rows, parent, productWord, rf.minRelevance, maxChildren)
			out.ChildrenFound = len(children)
			if len(children) == 0 {
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"EverBee returned no child keywords for %q that clear the relevance floor%s; nothing to rank.",
					parent, productWordNote(productWord)))
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Step 2: score each child. A failure travels with its child and is
			// excluded from the ranking rather than becoming a zero-scored row
			// that would read as a real (bad) result.
			type item struct {
				idx int
				sub research.SubNiche
			}
			results := make(chan item, len(children))
			var wg sync.WaitGroup
			for i, child := range children {
				wg.Add(1)
				go func(i int, seed string) {
					defer wg.Done()
					v, err := research.Niche(ctx, c, rf.options(seed))
					if err != nil {
						results <- item{idx: i, sub: research.SubNiche{Seed: seed, Error: err.Error()}}
						return
					}
					results <- item{idx: i, sub: research.SubNiche{
						Seed:        seed,
						Demand:      v.Demand,
						Competition: v.Competition,
						Saturation:  v.Saturation,
						Opportunity: v.Opportunity,
						Confidence:  v.Confidence,
						Evidence:    v.Evidence,
						PriceBand:   v.PriceBand,
						Warnings:    v.Warnings,
					}}
				}(i, child)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			ordered := make([]research.SubNiche, len(children))
			for r := range results {
				ordered[r.idx] = r.sub
			}

			scored := make([]research.SubNiche, 0, len(ordered))
			for _, s := range ordered {
				if s.Error != "" {
					out.FetchFailures = append(out.FetchFailures, fetchFailure{Seed: s.Seed, Error: s.Error})
					continue
				}
				scored = append(scored, s)
			}
			research.Normalize(scored)
			out.Ranked = scored

			// Every child failed: the research path is broken, not the market empty.
			if len(scored) == 0 && len(out.FetchFailures) > 0 {
				_ = printJSONFiltered(cmd.OutOrStdout(), out, flags)
				return fmt.Errorf("all %d child niches failed to fetch; the research path is not usable (see fetch_failures)", len(out.FetchFailures))
			}
			if n := len(out.FetchFailures); n > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d child niches failed to fetch; ranking computed over the remaining %d\n",
					n, len(children), len(scored))
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"%d of %d children failed to fetch and are excluded from the ranking (see fetch_failures).",
					n, len(children)))
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&rf.productType, "product", "", "Constrain evidence to a product type: physical, digital, or apparel (default: any)")
	cmd.Flags().BoolVar(&rf.excludeSVGPNG, "exclude-svg-png", false, "Exclude digital cut-file listings (SVG/PNG/Cricut) from the evidence count")
	cmd.Flags().Float64Var(&rf.minRelevance, "min-relevance", research.DefaultMinRelevance, "Relevance floor (0-1) a row must clear to count as evidence. Rows below it are still returned and annotated, never dropped.")
	cmd.Flags().IntVar(&rf.perPage, "limit", 20, "Number of listings to fetch as product evidence")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent niche to expand, e.g. dad")
	cmd.Flags().StringVar(&productWord, "product-word", "", "Require child keywords to mention this word, e.g. shirt")
	cmd.Flags().IntVar(&maxChildren, "max-children", 10, "Maximum child niches to score")
	return cmd
}

func productWordNote(w string) string {
	if w == "" {
		return ""
	}
	return fmt.Sprintf(" and mention %q", w)
}
