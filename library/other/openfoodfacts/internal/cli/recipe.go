// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: aggregate nutrition across multiple products into a recipe.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type recipeIngredient struct {
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Grams      float64    `json:"grams"`
	Nutriments nutriments `json:"nutriments"`
}

type recipeReport struct {
	Servings    float64            `json:"servings"`
	Ingredients []recipeIngredient `json:"ingredients"`
	Total       nutriments         `json:"total"`
	PerServing  nutriments         `json:"per_serving"`
}

func newNovelRecipeCmd(flags *rootFlags) *cobra.Command {
	var flagServings float64

	cmd := &cobra.Command{
		Use:         "recipe <code1> [code2...]",
		Short:       "Sum per-serving nutriments across multiple products into one recipe block plus a per-serving block.",
		Long:        "Fetch each product by barcode, sum its contribution (the product's serving size, or 100g when none is declared), and report the recipe total plus per-serving macros for --servings portions.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli recipe 3017620422003 7622210449283 --servings 4\n  openfoodfacts-pp-cli recipe 3017620422003 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("at least one barcode is required\nUsage: %s <code1> [code2...] [--servings N]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagServings <= 0 {
				return usageErr(fmt.Errorf("--servings must be greater than zero"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report := recipeReport{Servings: flagServings, Ingredients: []recipeIngredient{}}
			for _, code := range args {
				prod, found, err := fetchProduct(cmd.Context(), c, code)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if !found {
					return notFoundErr(fmt.Errorf("no product found for barcode %s", code))
				}
				grams := 100.0
				if sg, ok := servingGrams(prod); ok {
					grams = sg
				}
				n := nutrFromObject(prod).scaled(grams / 100.0)
				report.Ingredients = append(report.Ingredients, recipeIngredient{
					Code:       code,
					Name:       prodString(prod, "product_name"),
					Grams:      grams,
					Nutriments: n,
				})
				report.Total = report.Total.add(n)
			}
			report.PerServing = report.Total.scaled(1.0 / flagServings)

			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "recipe of %d ingredients, %.0f servings\n", len(report.Ingredients), flagServings)
			for _, ing := range report.Ingredients {
				fmt.Fprintf(out, "  - %s (%.0fg): %.0f kcal\n", displayName(ing.Name, ing.Code), ing.Grams, ing.Nutriments.Kcal)
			}
			fmt.Fprintf(out, "total:       %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
				report.Total.Kcal, report.Total.Protein, report.Total.Fat, report.Total.Carbs)
			fmt.Fprintf(out, "per serving: %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
				report.PerServing.Kcal, report.PerServing.Protein, report.PerServing.Fat, report.PerServing.Carbs)
			return nil
		},
	}
	cmd.Flags().Float64Var(&flagServings, "servings", 1, "Number of servings the recipe yields")
	return cmd
}
