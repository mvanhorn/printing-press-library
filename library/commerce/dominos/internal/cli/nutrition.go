package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type nutritionTotals struct {
	Calories float64 `json:"calories"`
	FatG     float64 `json:"fat_g"`
	CarbsG   float64 `json:"carbs_g"`
	ProteinG float64 `json:"protein_g"`
}

type nutritionItem struct {
	Code     string  `json:"code"`
	Qty      int     `json:"qty"`
	Calories float64 `json:"calories"`
	FatG     float64 `json:"fat_g"`
	CarbsG   float64 `json:"carbs_g"`
	ProteinG float64 `json:"protein_g"`
	Source   string  `json:"source"`
	Name     string  `json:"-"`
}

func newNutritionCmd(flags *rootFlags) *cobra.Command {
	var cartPath, storeID string
	cmd := &cobra.Command{
		Use:   "nutrition",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Sum calories, fat, carbs, and protein for a cart",
		Example: "  dominos-pp-cli nutrition --cart cart.json --store 7094\n" +
			"  dominos-pp-cli nutrition --cart cart.json --json\n" +
			"  dominos-pp-cli nutrition --cart cart.json --store 7094 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			// When invoked with no flags at all, print help so users (and verify) get a useful response.
			if !flags.dryRun && strings.TrimSpace(cartPath) == "" && strings.TrimSpace(storeID) == "" {
				return cmd.Help()
			}
			if flags.dryRun {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				if storeID == "" {
					storeID = "DRYRUN"
				}
				_, err = c.Get("/power/store/"+storeID+"/menu", map[string]string{"lang": "en", "structured": "true"})
				if err != nil {
					return classifyAPIError(err)
				}
				return nil
			}
			if strings.TrimSpace(cartPath) == "" {
				return fmt.Errorf("required flag %q not set", "cart")
			}
			order, err := loadTemplateOrder(cartPath)
			if err != nil {
				return err
			}
			if storeID == "" {
				storeID = stringValue(order["StoreID"])
			}
			if storeID == "" {
				return fmt.Errorf("required flag %q not set", "store")
			}
			products, err := nutritionProducts(order["Products"])
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			menuRaw, err := c.Get("/power/store/"+storeID+"/menu", map[string]string{"lang": "en", "structured": "true"})
			if err != nil {
				return classifyAPIError(err)
			}
			menu, err := nutritionMenu(extractResponseData(menuRaw))
			if err != nil {
				return err
			}
			items := make([]nutritionItem, 0, len(products))
			unknown := make([]string, 0)
			totalQty := 0
			var totals nutritionTotals
			for _, product := range products {
				item, ok := nutritionResolve(menu, product)
				if !ok {
					item = nutritionItem{Code: product.Code, Qty: product.Qty, Source: "unknown"}
					unknown = append(unknown, product.Code)
				}
				items = append(items, item)
				totalQty += item.Qty
				totals.Calories += item.Calories
				totals.FatG += item.FatG
				totals.CarbsG += item.CarbsG
				totals.ProteinG += item.ProteinG
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"items":                   items,
					"totals":                  totals,
					"items_unknown_nutrition": unknown,
				})
			}
			if err := nutritionPrintTable(cmd, items, totals, totalQty); err != nil {
				return err
			}
			if len(unknown) > 0 {
				fmt.Fprintf(os.Stderr, "warning: unknown nutrition for %s\n", strings.Join(unknown, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cartPath, "cart", "", "Path to a cart JSON file")
	cmd.Flags().StringVar(&storeID, "store", "", "Store ID to use when cart lacks StoreID")
	return cmd
}

type nutritionProduct struct {
	Code string
	Qty  int
}

func nutritionProducts(raw any) ([]nutritionProduct, error) {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("cart requires Products")
	}
	out := make([]nutritionProduct, 0, len(items))
	for _, item := range items {
		product, ok := item.(map[string]any)
		if !ok {
			continue
		}
		code := comparePricesString(product["Code"])
		if code == "" {
			code = comparePricesString(product["code"])
		}
		if code == "" {
			continue
		}
		qty := 1
		if v, ok := comparePricesFloat(product["Qty"]); ok && v > 0 {
			qty = int(math.Round(v))
		} else if v, ok := comparePricesFloat(product["qty"]); ok && v > 0 {
			qty = int(math.Round(v))
		}
		out = append(out, nutritionProduct{Code: code, Qty: qty})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cart requires at least one product with Code")
	}
	return out, nil
}

func nutritionMenu(data json.RawMessage) (map[string]any, error) {
	var menu map[string]any
	if err := json.Unmarshal(data, &menu); err != nil {
		return nil, fmt.Errorf("parse menu response: %w", err)
	}
	return menu, nil
}
func nutritionResolve(menu map[string]any, product nutritionProduct) (nutritionItem, bool) {
	if variants, ok := nutritionMap(menu["Variants"]); ok {
		if item, ok := nutritionFromNode(product, variants[product.Code], "Variants.Tags.NutritionInfo"); ok {
			return item, true
		}
	}
	if products, ok := nutritionMap(menu["Products"]); ok {
		if item, ok := nutritionFromNode(product, products[product.Code], "Products.Nutrition"); ok {
			return item, true
		}
		for _, raw := range products {
			obj, ok := nutritionMap(raw)
			if !ok {
				continue
			}
			code := comparePricesString(obj["Code"])
			if code == "" {
				code = comparePricesString(obj["code"])
			}
			if code != product.Code {
				continue
			}
			if item, ok := nutritionFromNode(product, obj, "Products.Nutrition"); ok {
				return item, true
			}
		}
	}
	return nutritionItem{}, false
}

func nutritionFromNode(product nutritionProduct, raw any, fallbackSource string) (nutritionItem, bool) {
	node, ok := nutritionMap(raw)
	if !ok {
		return nutritionItem{}, false
	}
	name := comparePricesString(node["Name"])
	if name == "" {
		name = comparePricesString(node["name"])
	}
	if tags, ok := nutritionMap(node["Tags"]); ok {
		if info, ok := nutritionMap(tags["NutritionInfo"]); ok {
			item, ok := nutritionItemFromInfo(product, name, info, tags, "Variants.Tags.NutritionInfo")
			if ok {
				return item, true
			}
		}
	}
	if info, ok := nutritionMap(node["Nutrition"]); ok {
		return nutritionItemFromInfo(product, name, info, node, fallbackSource)
	}
	return nutritionItem{}, false
}

func nutritionItemFromInfo(product nutritionProduct, name string, info, context map[string]any, source string) (nutritionItem, bool) {
	calories, okCalories := nutritionNumber(info, "Calories", "calories")
	fat, okFat := nutritionNumber(info, "Fat", "fat")
	carbs, okCarbs := nutritionNumber(info, "Carbs", "carbs", "Carbohydrates", "carbohydrates")
	protein, okProtein := nutritionNumber(info, "Protein", "protein")
	if !okCalories && !okFat && !okCarbs && !okProtein {
		return nutritionItem{}, false
	}
	multiplier := float64(product.Qty)
	if nutritionTruthy(context["PerServing"]) || nutritionTruthy(info["PerServing"]) {
		if servings, ok := nutritionNumber(context, "ServingCount", "servingCount", "Servings", "servings"); ok && servings > 0 {
			multiplier *= servings
		} else if servings, ok := nutritionNumber(info, "ServingCount", "servingCount", "Servings", "servings"); ok && servings > 0 {
			multiplier *= servings
		}
	}
	return nutritionItem{
		Code:     product.Code,
		Qty:      product.Qty,
		Calories: calories * multiplier,
		FatG:     fat * multiplier,
		CarbsG:   carbs * multiplier,
		ProteinG: protein * multiplier,
		Source:   source,
		Name:     name,
	}, true
}

func nutritionMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func nutritionNumber(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if v, ok := comparePricesFloat(m[key]); ok {
			return v, true
		}
	}
	return 0, false
}

func nutritionTruthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "true")
	default:
		f, ok := comparePricesFloat(v)
		return ok && f != 0
	}
}

func nutritionPrintTable(cmd *cobra.Command, items []nutritionItem, totals nutritionTotals, totalQty int) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "Item\tQty\tCal\tFat\tCarbs\tProtein")
	for _, item := range items {
		label := item.Code
		if item.Name != "" {
			label += " (" + item.Name + ")"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", label, item.Qty, nutritionFmt(item.Calories), nutritionFmt(item.FatG), nutritionFmt(item.CarbsG), nutritionFmt(item.ProteinG))
	}
	fmt.Fprintf(tw, "%s\n", strings.Repeat("-", 56))
	fmt.Fprintf(tw, "Total\t%d\t%s\t%s\t%s\t%s\n", totalQty, nutritionFmt(totals.Calories), nutritionFmt(totals.FatG), nutritionFmt(totals.CarbsG), nutritionFmt(totals.ProteinG))
	return tw.Flush()
}

func nutritionFmt(v float64) string {
	if math.Abs(v-math.Round(v)) < 0.05 {
		return fmt.Sprintf("%.0f", math.Round(v))
	}
	return fmt.Sprintf("%.1f", v)
}
