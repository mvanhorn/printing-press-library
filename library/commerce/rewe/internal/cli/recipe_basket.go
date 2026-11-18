// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

type recipeLine struct {
	Ingredient string `json:"ingredient"`
	Basic      bool   `json:"basic"`
	ProductID  string `json:"product_id,omitempty"`
	ListingID  string `json:"listing_id,omitempty"`
	Title      string `json:"title,omitempty"`
	PriceCents int    `json:"price_cents,omitempty"`
	Grammage   string `json:"grammage,omitempty"`
	Options    int    `json:"options"`
}

func newNovelRecipeBasketCmd(flags *rootFlags) *cobra.Command {
	var servings int
	var add, skipBasic bool
	cmd := &cobra.Command{
		Use:   "basket [slug-or-url]",
		Short: "Map a REWE recipe's ingredients to shop products for N servings and add the recommended ones.",
		Long: "Sends the recipe to REWE's ingredient mapper (the 'Zutaten online bestellen' button), waits\n" +
			"for the result and lists the first recommended product per ingredient. --add puts them into\n" +
			"the basket; --skip-basic leaves out pantry basics such as salt, pepper and oil.",
		Example:     "  rewe-pp-cli recipe basket gemuese-lasagne --servings 4 --json\n  rewe-pp-cli recipe basket gemuese-lasagne --servings 2 --skip-basic --add",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "slug-or-url=gemuese-lasagne;--servings=2", "pp:preview-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "recipe basket"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if servings <= 0 || servings > 20 {
				return usageErr(fmt.Errorf("--servings must be between 1 and 20"))
			}
			if dryRunOK(flags) && !add {
				return writeDryRun(cmd.OutOrStdout(), flags, "recipe basket")
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			r, err := sh.Recipe(ctx, args[0])
			if err != nil {
				return reweErr(err)
			}
			m, err := sh.MapRecipe(ctx, r, servings)
			if err != nil {
				return reweErr(err)
			}
			lines := []recipeLine{}
			items := []rewe.LineItem{}
			total := 0
			for _, ing := range m.Ingredients {
				l := recipeLine{Ingredient: ing.DisplayName, Basic: ing.Basic, Options: len(ing.ProductRecommendations)}
				if l.Ingredient == "" {
					l.Ingredient = ing.NormalizedSearchTerm
				}
				if len(ing.ProductRecommendations) > 0 && !(skipBasic && ing.Basic) {
					p := ing.ProductRecommendations[0]
					l.ProductID, l.ListingID, l.Title, l.PriceCents, l.Grammage = p.ProductID, p.ListingID, p.Title, p.Pricing.CurrentRetailPrice, p.Pricing.Grammage
					items = append(items, rewe.LineItem{ListingID: p.ListingID, Quantity: 1})
					total += p.Pricing.CurrentRetailPrice
				}
				lines = append(lines, l)
			}
			out := map[string]any{"recipe": r.Name, "servings": servings, "mapping_id": m.MappingID, "lines": lines, "total_cents": total, "total": rewe.FormatCents(total), "added": false}
			if add {
				if flags.dryRun {
					out["would_add"] = len(items)
				} else if len(items) > 0 {
					res, err := sh.AddMany(ctx, items, "ingredient-mapper")
					if err != nil {
						return reweErr(err)
					}
					out["added"] = true
					out["add_result"] = res
					_ = sh.Persist()
				}
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "%s for %d servings:\n", r.Name, servings)
				for _, l := range lines {
					if l.Title == "" {
						fmt.Fprintf(w, "  - %-30s (no product%s)\n", trunc(l.Ingredient, 30), map[bool]string{true: ", basic", false: ""}[l.Basic])
						continue
					}
					fmt.Fprintf(w, "  - %-30s -> %-40s %s\n", trunc(l.Ingredient, 30), trunc(l.Title, 40), rewe.FormatCents(l.PriceCents))
				}
				fmt.Fprintf(w, "Total of recommended products: %s. Added: %v\n", rewe.FormatCents(total), out["added"])
			})
		},
	}
	cmd.Flags().IntVar(&servings, "servings", 4, "Number of servings")
	cmd.Flags().BoolVar(&add, "add", false, "Add the recommended products to the basket")
	cmd.Flags().BoolVar(&skipBasic, "skip-basic", false, "Leave out pantry basics (salt, pepper, oil ...)")
	return cmd
}
