// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newScaleCmd(flags *rootFlags) *cobra.Command {
	var servings int
	cmd := &cobra.Command{
		Use:   "scale <slug-or-url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Scale a Food52 recipe's ingredients to a different number of servings",
		Long: strings.TrimSpace(`
Fetches a recipe, parses its yield from the JSON-LD recipeYield field, and
rewrites every ingredient quantity proportionally. Falls back to the SSR
servings field when JSON-LD doesn't carry a numeric yield.

Quantities are detected by parsing the leading number in each ingredient
line. Mixed numbers ("1 1/2 cups") and ASCII fractions ("1/2 cup") are
both supported. Lines without a leading quantity are passed through
unchanged.
`),
		Example: strings.Trim(`
  food52-pp-cli scale sarah-fennel-s-best-lunch-lady-brownie-recipe --servings 8 --json
  food52-pp-cli scale https://food52.com/recipes/mom-s-japanese-curry-chicken-with-radish-and-cauliflower --servings 6
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || servings <= 0 {
				return cmd.Help()
			}
			slug := recipeSlugFromArg(args[0])
			if slug == "" {
				return fmt.Errorf("recipe slug or URL is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			html, err := fetchHTML(c, "/recipes/"+slug, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			if c.DryRun {
				return emitJSON(map[string]any{
					"slug":            slug,
					"target_servings": servings,
					"dry_run":         true,
					"url":             canonicalRecipeURL(slug),
				})
			}
			r, err := food52.ExtractRecipe(html, canonicalRecipeURL(slug))
			if err != nil {
				return err
			}
			origServings, ok := parseYieldServings(r.Yield)
			if !ok {
				return fmt.Errorf("could not parse a numeric serving count from yield %q (the recipe doesn't expose one)", r.Yield)
			}
			factor := float64(servings) / float64(origServings)
			scaled := make([]string, len(r.Ingredients))
			for i, line := range r.Ingredients {
				scaled[i] = scaleIngredientLine(line, factor)
			}
			out := map[string]any{
				"slug":              r.Slug,
				"title":             r.Title,
				"url":               r.URL,
				"original_servings": origServings,
				"target_servings":   servings,
				"factor":            factor,
				"ingredients":       scaled,
				"instructions":      r.Instructions,
				"yield":             r.Yield,
			}
			return emitFromFlags(flags, out, func() {
				fmt.Printf("%s\n%s\n", r.Title, strings.Repeat("=", min(len(r.Title), 78)))
				fmt.Printf("Scaled from %d → %d servings (factor %.2f)\n\n", origServings, servings, factor)
				fmt.Println("Ingredients (scaled)")
				fmt.Println("--------------------")
				for _, line := range scaled {
					fmt.Printf("- %s\n", line)
				}
				fmt.Println()
				fmt.Println("Steps (unchanged)")
				fmt.Println("-----------------")
				for i, s := range r.Instructions {
					fmt.Printf("%d. %s\n", i+1, s)
				}
			})
		},
	}
	cmd.Flags().IntVar(&servings, "servings", 0, "Target serving count (must be > 0)")
	return cmd
}

// parseYieldServings extracts an integer servings count from a recipeYield
// string. Tolerates: "Serves: 4", "Serves: 4-5", "4 servings", "Makes 12",
// "Yields 8 cookies". Returns the lower bound for ranges.
func parseYieldServings(yield string) (int, bool) {
	if yield == "" {
		return 0, false
	}
	digits := regexp.MustCompile(`(\d+)`).FindString(yield)
	if digits == "" {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// scaleIngredientLine rewrites the leading quantity in an ingredient line by
// the given factor. Supports mixed numbers ("1 1/2"), simple fractions
// ("1/2"), and decimals ("1.5"). Non-quantity-prefixed lines are returned
// unchanged.
//
// We deliberately don't try to convert units or simplify fractions: keeping
// the scaled value as a decimal with one digit of precision (or two for
// values < 1) keeps the math obvious to the cook.
var (
	// "1 1/2 cups flour"
	mixedRe = regexp.MustCompile(`^\s*(\d+)\s+(\d+)/(\d+)\s+(.*)$`)
	// "1/2 cup butter"
	fracRe = regexp.MustCompile(`^\s*(\d+)/(\d+)\s+(.*)$`)
	// "1.5 cups sugar" or "2 cups flour"
	decRe = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s+(.*)$`)
)

func scaleIngredientLine(line string, factor float64) string {
	if m := mixedRe.FindStringSubmatch(line); m != nil {
		whole, _ := strconv.Atoi(m[1])
		num, _ := strconv.Atoi(m[2])
		den, _ := strconv.Atoi(m[3])
		if den == 0 {
			return line
		}
		v := (float64(whole) + float64(num)/float64(den)) * factor
		return formatScaled(v) + " " + m[4]
	}
	if m := fracRe.FindStringSubmatch(line); m != nil {
		num, _ := strconv.Atoi(m[1])
		den, _ := strconv.Atoi(m[2])
		if den == 0 {
			return line
		}
		v := (float64(num) / float64(den)) * factor
		return formatScaled(v) + " " + m[3]
	}
	if m := decRe.FindStringSubmatch(line); m != nil {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return line
		}
		v = v * factor
		return formatScaled(v) + " " + m[2]
	}
	return line
}

func formatScaled(v float64) string {
	if v == float64(int(v)) {
		return strconv.Itoa(int(v))
	}
	if v < 1 {
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}
