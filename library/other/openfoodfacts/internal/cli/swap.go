// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: find healthier alternatives in the same category.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type swapAlt struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	NutriScore    string  `json:"nutriscore_grade"`
	NutriScoreNum float64 `json:"nutriscore_score"`
	Nova          float64 `json:"nova_group"`
	Category      string  `json:"category"`
}

func newNovelSwapCmd(flags *rootFlags) *cobra.Command {
	var flagMaxNova int
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "swap <code>",
		Short:       "Given a product, find better-scoring items in the same category, ranked by Nutri-Score/NOVA.",
		Long:        "Look up a product's primary category, search that category sorted by Nutri-Score, and return products that score better (lower Nutri-Score) than the original — the healthier-alternative answer no other tool gives.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli swap 3017620422003 --max-nova 3\n  openfoodfacts-pp-cli swap 3017620422003 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("barcode is required\nUsage: %s <code> [--max-nova N]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			code := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			prod, found, err := fetchProduct(cmd.Context(), c, code)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !found {
				return notFoundErr(fmt.Errorf("no product found for barcode %s", code))
			}
			category := prodFirstRawTag(prod, "categories_tags")
			if category == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "no category known for %s — cannot find alternatives\n", code)
				return printJSONOrEmpty(cmd, flags, []swapAlt{})
			}
			srcScore := prodFloat(prod, "nutriscore_score")
			_, hasSrcScore := prod["nutriscore_score"]

			products, err := offSearch(cmd.Context(), c, map[string]string{
				"categories_tags": category,
				"sort_by":         "nutriscore_score",
				"page_size":       "50",
				"fields":          "code,product_name,nutriscore_grade,nutriscore_score,nova_group,categories_tags",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			alts := []swapAlt{}
			for _, p := range products {
				pc := prodString(p, "code")
				if pc == "" || pc == code {
					continue
				}
				if _, ok := p["nutriscore_score"]; !ok {
					continue
				}
				score := prodFloat(p, "nutriscore_score")
				if hasSrcScore && score >= srcScore {
					continue
				}
				nv := prodFloat(p, "nova_group")
				// NOVA 0 means unclassified in OFF. When the user sets a NOVA
				// ceiling (to avoid ultra-processed food), products with an
				// unknown processing level must NOT slip through the filter.
				if flagMaxNova > 0 && (nv <= 0 || nv > float64(flagMaxNova)) {
					continue
				}
				alts = append(alts, swapAlt{
					Code:          pc,
					Name:          prodString(p, "product_name"),
					NutriScore:    prodString(p, "nutriscore_grade"),
					NutriScoreNum: score,
					Nova:          nv,
					Category:      category,
				})
			}
			sort.SliceStable(alts, func(i, j int) bool { return alts[i].NutriScoreNum < alts[j].NutriScoreNum })
			if flagLimit > 0 && len(alts) > flagLimit {
				alts = alts[:flagLimit]
			}

			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), alts, flags)
			}
			if len(alts) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no healthier alternatives found in %s\n", strings.TrimPrefix(category, "en:"))
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tNAME\tNUTRI\tSCORE\tNOVA")
			for _, a := range alts {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.0f\t%s\n", a.Code, truncate(a.Name, 32), strings.ToUpper(grade(a.NutriScore)), a.NutriScoreNum, nova(a.Nova))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagMaxNova, "max-nova", 0, "Only return alternatives at or below this NOVA group (1-4)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of alternatives to return")
	return cmd
}

// prodFirstRawTag returns the most specific *canonical* member of a *_tags
// array (a "lang:slug" entry whose slug is lowercase ascii + hyphens), prefix
// intact. OFF category lists end with localized/display variants ("en:Pâtes à
// tartiner") that are not valid search keys; this skips them. Falls back to the
// last non-empty entry only if no canonical tag exists.
func prodFirstRawTag(prod map[string]any, key string) string {
	raw, ok := prod[key].([]any)
	if !ok || len(raw) == 0 {
		return ""
	}
	var fallback string
	for i := len(raw) - 1; i >= 0; i-- {
		s, ok := raw[i].(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		if fallback == "" {
			fallback = s
		}
		if isCanonicalTag(s) {
			return s
		}
	}
	return fallback
}

// isCanonicalTag reports whether a tag is a canonical "lang:slug" key whose
// slug is lowercase ascii letters, digits, and hyphens — the form OFF accepts
// as a search filter.
func isCanonicalTag(tag string) bool {
	i := strings.IndexByte(tag, ':')
	if i < 1 || i+1 >= len(tag) {
		return false
	}
	for _, r := range tag[i+1:] {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

// printJSONOrEmpty prints v as JSON in machine mode; human mode is a no-op
// because the caller already printed an explanatory line.
func printJSONOrEmpty(cmd *cobra.Command, flags *rootFlags, v any) error {
	if wantsJSONOut(cmd, flags) {
		return printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	return nil
}
