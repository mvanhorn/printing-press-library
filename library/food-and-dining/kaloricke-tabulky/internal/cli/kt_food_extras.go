package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/kaloricke-tabulky/internal/jsonld"
)

// food get <slug>           — HTML scrape + JSON-LD nutrition parser
// food allergens <slug>     — derived from JSON-LD keywords (transcendence #6)
// food substitutes <slug>   — Euclidean distance over typed nutrition (transcendence #5)
// All three are added under the existing "food" promoted parent via the
// kt_root_wiring registration.

func newKTFoodGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [slug]",
		Short: "Get full Czech-language nutrition for a foodstuff from the JSON-LD on its detail page",
		Long: `Fetches /potraviny/<slug> and parses the embedded
<script type=\"application/ld+json\"> block. Returns typed nutrition
(per 100 g): energy (kJ + kcal), protein, fat, carb, fiber, sugar,
saturated/mono/poly fats, calcium.`,
		Example: `  kaloricke-tabulky-pp-cli food get jablko
  kaloricke-tabulky-pp-cli food get tvaroh-mekky --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			c, _, err := ktNewAuthenticatedClient(flags)
			cookieHeader := ""
			if err == nil {
				cookieHeader = c.Config.AuthHeader()
			}
			d, err := jsonld.FetchDetail(nil, "https://www.kaloricketabulky.cz/potraviny/"+slug, cookieHeader)
			if err != nil {
				return err
			}
			return ktEmit(cmd.OutOrStdout(), flags, d)
		},
	}
	return cmd
}

func newKTFoodAllergensCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "allergens [slug]",
		Short: "Mine Czech allergen tokens (gluten/lactose/egg/nuts/soy/fish/...) from a food's JSON-LD",
		Long: `Extracts Czech allergen tokens from the JSON-LD keywords block on
/potraviny/<slug>. Returns the canonical English allergen names found.

Recognized: gluten (lepek), lactose (laktóza), egg (vejce), nuts (ořechy),
soy (sója), fish (ryby), honey (med), celery (celer), mustard (hořčice),
sesame (sezam), crustaceans (korýši), molluscs (měkkýši).`,
		Example: `  kaloricke-tabulky-pp-cli food allergens jablko
  kaloricke-tabulky-pp-cli food allergens tvaroh-mekky --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			d, err := jsonld.FetchDetail(nil, "https://www.kaloricketabulky.cz/potraviny/"+slug, "")
			if err != nil {
				return err
			}
			allergens := jsonld.ExtractAllergens(d.Nutrition)
			sort.Strings(allergens)
			return ktEmit(cmd.OutOrStdout(), flags, map[string]any{
				"slug":      slug,
				"title":     d.Title,
				"allergens": allergens,
				"count":     len(allergens),
			})
		},
	}
	return cmd
}

// food substitutes <slug> [--by macro] [--limit N]
// Approach: fetch the JSON-LD nutrition for the seed food, then run
// /autocomplete/foodstuff with a broad query (the slug's first word),
// fetch the JSON-LD for each candidate, compute Euclidean distance over
// the chosen macro dimension, return the top N.
func newKTFoodSubstitutesCmd(flags *rootFlags) *cobra.Command {
	var byMacro string
	var limit int
	var poolQuery string

	cmd := &cobra.Command{
		Use:   "substitutes [slug]",
		Short: "Find foods with a macro profile similar to the seed food (Euclidean distance per 100 g)",
		Long: `Fetches the seed food's nutrition (JSON-LD), then searches the
autocomplete for a candidate pool, fetches each candidate's nutrition,
and ranks by distance over the chosen macro dimension.

Default --by=protein favors substitutes with similar protein but
possibly different energy/carb/fat (useful for shopping a leaner option
at the same protein target). --by=energy ranks by total kJ.

Pool is built from the autocomplete results for --query (default: the
seed slug's first word). The pool is bounded to keep latency low; pass
--limit-pool to control it.`,
		Example: `  kaloricke-tabulky-pp-cli food substitutes jablko --by protein --limit 5
  kaloricke-tabulky-pp-cli food substitutes tvaroh-mekky --by energy --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			// Seed
			seed, err := jsonld.FetchDetail(nil, "https://www.kaloricketabulky.cz/potraviny/"+slug, "")
			if err != nil {
				return fmt.Errorf("fetching seed food: %w", err)
			}

			// Candidate pool via autocomplete
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				// Substitutes works without auth too; rebuild a no-auth client via direct HTTP.
				return fmt.Errorf("(substitutes can run without auth in a future revision; for now please run `auth password-login` or `auth login --chrome`) %w", err)
			}
			query := poolQuery
			if query == "" {
				query = strings.SplitN(slug, "-", 2)[0]
				if len(query) < 3 {
					query = slug
				}
			}
			raw, err := c.GetNoCache("/autocomplete/foodstuff", map[string]string{
				"query":  query,
				"format": "json",
			})
			if err != nil {
				return fmt.Errorf("fetching pool: %w", err)
			}
			var hits []ktAutocompleteHit
			if err := jsonUnmarshalSlice(raw, &hits); err != nil {
				return fmt.Errorf("parsing pool: %w", err)
			}
			// Bound the pool size (fetching JSON-LD per candidate is expensive).
			poolCap := 12
			if len(hits) > poolCap {
				hits = hits[:poolCap]
			}

			type candidate struct {
				Slug      string           `json:"slug"`
				Title     string           `json:"title"`
				Distance  float64          `json:"distance"`
				Nutrition jsonld.Nutrition `json:"nutrition"`
			}
			cands := make([]candidate, 0, len(hits))
			for _, h := range hits {
				if h.URL == slug {
					continue // skip seed
				}
				d, err := jsonld.FetchDetail(nil, "https://www.kaloricketabulky.cz/potraviny/"+h.URL, "")
				if err != nil {
					continue
				}
				d.Nutrition.Raw = nil // bulky
				dist := ktNutritionDistance(seed.Nutrition, d.Nutrition, byMacro)
				cands = append(cands, candidate{Slug: h.URL, Title: d.Title, Distance: dist, Nutrition: d.Nutrition})
			}
			sort.Slice(cands, func(i, j int) bool { return cands[i].Distance < cands[j].Distance })
			if limit > 0 && len(cands) > limit {
				cands = cands[:limit]
			}
			result := map[string]any{
				"seed_slug":       slug,
				"seed_title":      seed.Title,
				"seed_nutrition":  seed.Nutrition,
				"distance_metric": byMacro,
				"candidates":      cands,
			}
			return ktEmit(cmd.OutOrStdout(), flags, result)
		},
	}
	cmd.Flags().StringVar(&byMacro, "by", "protein", "Distance metric: protein|carb|fat|energy|all")
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of substitutes to return")
	cmd.Flags().StringVar(&poolQuery, "pool-query", "", "Autocomplete query to build the candidate pool (default: seed slug's first word)")
	return cmd
}

func ktNutritionDistance(a, b jsonld.Nutrition, by string) float64 {
	switch by {
	case "protein":
		return math.Abs(a.ProteinG - b.ProteinG)
	case "carb", "carbohydrate":
		return math.Abs(a.CarbG - b.CarbG)
	case "fat":
		return math.Abs(a.FatG - b.FatG)
	case "energy", "kj":
		return math.Abs(a.EnergyKJ - b.EnergyKJ)
	case "all":
		// Multi-dim Euclidean with per-axis weight 1
		dp := a.ProteinG - b.ProteinG
		dc := a.CarbG - b.CarbG
		df := a.FatG - b.FatG
		de := (a.EnergyKJ - b.EnergyKJ) / 100 // scale-down kJ
		return math.Sqrt(dp*dp + dc*dc + df*df + de*de)
	}
	return math.Abs(a.ProteinG - b.ProteinG)
}

// jsonUnmarshalSlice is a tiny indirection so we don't need encoding/json
// in callers that already pull from helpers.
func jsonUnmarshalSlice(raw []byte, dst any) error {
	return jsonUnmarshalImpl(raw, dst)
}
