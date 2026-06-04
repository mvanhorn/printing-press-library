// Copyright 2026 Amit and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type menuItemRow struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	PriceInt           int      `json:"price_int"`
	PriceFormatted     string   `json:"price,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	Category           string   `json:"category,omitempty"`
	CategorySlug       string   `json:"category_slug,omitempty"`
	Enabled            bool     `json:"enabled"`
	OutOfStock         bool     `json:"out_of_stock"`
	DietaryPreferences []string `json:"dietary_preferences,omitempty"`
	ImageURL           string   `json:"image_url,omitempty"`
}

type menuCategoryRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	ItemCount   int    `json:"item_count"`
}

func newMenuCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "menu",
		Short: "Browse a Wolt venue's menu (categories + items)",
		Long:  "Hits Wolt's consumer-assortment endpoint to fetch a venue's full menu — categories, items, prices, dietary preferences, availability.",
	}
	cmd.AddCommand(newMenuShowCmd(flags))
	cmd.AddCommand(newMenuItemsCmd(flags))
	cmd.AddCommand(newMenuCategoriesCmd(flags))
	cmd.AddCommand(newMenuSearchCmd(flags))
	return cmd
}

func newMenuShowCmd(flags *rootFlags) *cobra.Command {
	var lang string
	cmd := &cobra.Command{
		Use:         "show <slug>",
		Short:       "Show a venue's full menu (categories + items in one payload)",
		Example:     "  wolt-pp-cli menu show noodle-story-kamppi --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			data, err := fetchAssortment(cmd, flags, args[0], lang)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "en", "Menu language code (en, fi, sv, ...)")
	return cmd
}

func newMenuItemsCmd(flags *rootFlags) *cobra.Command {
	var lang, categorySlug string
	var limit int
	cmd := &cobra.Command{
		Use:         "items <slug>",
		Short:       "List menu items for a venue (id, name, price, category, availability)",
		Example:     "  wolt-pp-cli menu items noodle-story-kamppi --json --select count,items.name,items.price",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			data, err := fetchAssortment(cmd, flags, args[0], lang)
			if err != nil {
				return err
			}
			items := flattenMenuItems(data)
			if categorySlug != "" {
				// PATCH(menu-items-category-slug-or-name): match against either
				// the category slug or the display name (case-insensitive). The
				// flag was originally labelled "slug" but the filter only
				// compared display names, so passing a slug from `menu categories`
				// output returned zero results.
				want := strings.ToLower(categorySlug)
				filtered := items[:0]
				for _, it := range items {
					if strings.ToLower(it.Category) == want ||
						strings.ToLower(it.CategorySlug) == want {
						filtered = append(filtered, it)
					}
				}
				items = filtered
			}
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
			out := struct {
				Slug  string        `json:"slug"`
				Count int           `json:"count"`
				Items []menuItemRow `json:"items"`
			}{Slug: args[0], Count: len(items), Items: items}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "en", "Menu language code")
	cmd.Flags().StringVar(&categorySlug, "category", "", "Filter to items in this category (accepts slug or display name; case-insensitive)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap returned items (0 = no cap)")
	return cmd
}

func newMenuCategoriesCmd(flags *rootFlags) *cobra.Command {
	var lang string
	cmd := &cobra.Command{
		Use:         "categories <slug>",
		Short:       "List menu categories for a venue",
		Example:     "  wolt-pp-cli menu categories noodle-story-kamppi --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			data, err := fetchAssortment(cmd, flags, args[0], lang)
			if err != nil {
				return err
			}
			cats := flattenMenuCategories(data)
			out := struct {
				Slug       string            `json:"slug"`
				Count      int               `json:"count"`
				Categories []menuCategoryRow `json:"categories"`
			}{Slug: args[0], Count: len(cats), Categories: cats}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "en", "Menu language code")
	return cmd
}

func newMenuSearchCmd(flags *rootFlags) *cobra.Command {
	var lang, query string
	var maxPrice int
	cmd := &cobra.Command{
		Use:         "search <slug>",
		Short:       "Search within a venue's menu (substring match on name and description)",
		Example:     "  wolt-pp-cli menu search noodle-story-kamppi --q noodle --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			data, err := fetchAssortment(cmd, flags, args[0], lang)
			if err != nil {
				return err
			}
			items := flattenMenuItems(data)
			q := strings.ToLower(strings.TrimSpace(query))
			out := struct {
				Slug  string        `json:"slug"`
				Query string        `json:"query"`
				Count int           `json:"count"`
				Items []menuItemRow `json:"items"`
			}{Slug: args[0], Query: query}
			for _, it := range items {
				if maxPrice > 0 && it.PriceInt > maxPrice {
					continue
				}
				if q == "" || strings.Contains(strings.ToLower(it.Name), q) || strings.Contains(strings.ToLower(it.Description), q) {
					out.Items = append(out.Items, it)
				}
			}
			out.Count = len(out.Items)
			sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].PriceInt < out.Items[j].PriceInt })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "en", "Menu language code")
	cmd.Flags().StringVar(&query, "q", "", "Search query (substring on name/description)")
	cmd.Flags().IntVar(&maxPrice, "max-price", 0, "Cap price in cents (0 = no cap)")
	return cmd
}

func fetchAssortment(cmd *cobra.Command, flags *rootFlags, slug, lang string) (map[string]any, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, fmt.Errorf("venue slug is required")
	}
	if lang == "" {
		lang = "en"
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	fullURL := "https://consumer-api.wolt.com/consumer-api/consumer-assortment/v1/venues/slug/" +
		url.PathEscape(slug) + "/assortment?language=" + url.QueryEscape(lang)
	raw, err := c.Get(cmd.Context(), fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching menu for %s: %w", slug, err)
	}
	var d map[string]any
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parsing menu response: %w", err)
	}
	return d, nil
}

func flattenMenuItems(data map[string]any) []menuItemRow {
	itemsRaw, _ := data["items"].([]any)
	catsRaw, _ := data["categories"].([]any)

	// Build item_id -> {category name, category slug} maps. Tracking both
	// lets `menu items --category` accept either form (display name OR slug
	// copied from `menu categories` output).
	itemToCategory := map[string]string{}
	itemToCategorySlug := map[string]string{}
	for _, cRaw := range catsRaw {
		c, ok := cRaw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := c["name"].(string)
		slug, _ := c["slug"].(string)
		ids, _ := c["item_ids"].([]any)
		for _, idRaw := range ids {
			if id, ok := idRaw.(string); ok {
				itemToCategory[id] = name
				itemToCategorySlug[id] = slug
			}
		}
	}

	out := make([]menuItemRow, 0, len(itemsRaw))
	for _, iRaw := range itemsRaw {
		it, ok := iRaw.(map[string]any)
		if !ok {
			continue
		}
		row := menuItemRow{}
		row.ID, _ = it["id"].(string)
		row.Name, _ = it["name"].(string)
		row.Description, _ = it["description"].(string)
		row.Category = itemToCategory[row.ID]
		row.CategorySlug = itemToCategorySlug[row.ID]
		if e, ok := it["enabled"].(bool); ok {
			row.Enabled = e
		} else {
			row.Enabled = true
		}
		if oos, ok := it["is_out_of_stock"].(bool); ok {
			row.OutOfStock = oos
		}
		if dp, ok := it["dietary_preferences"].([]any); ok {
			for _, p := range dp {
				if ps, ok := p.(string); ok {
					row.DietaryPreferences = append(row.DietaryPreferences, ps)
				}
			}
		}
		if imgs, ok := it["images"].([]any); ok && len(imgs) > 0 {
			if img, ok := imgs[0].(map[string]any); ok {
				row.ImageURL, _ = img["url"].(string)
			}
		}
		// Pricing — Wolt's consumer-assortment returns `price` as a flat
		// integer in cents, plus optional `original_price`, `net_price`,
		// `unit_price`. Older shapes nested price as a map; fall through.
		if p, ok := it["price"].(float64); ok {
			row.PriceInt = int(p)
		} else if p, ok := it["price"].(map[string]any); ok {
			if amt, ok := p["amount"].(float64); ok {
				row.PriceInt = int(amt)
			}
			if cur, ok := p["currency"].(string); ok {
				row.Currency = cur
			}
		}
		if row.PriceInt == 0 {
			if bp, ok := it["base_price"].(float64); ok {
				row.PriceInt = int(bp)
			}
		}
		if row.Currency == "" {
			row.Currency = "EUR"
		}
		if row.PriceInt > 0 {
			row.PriceFormatted = formatMenuPrice(row.PriceInt, row.Currency)
		}
		out = append(out, row)
	}
	return out
}

func flattenMenuCategories(data map[string]any) []menuCategoryRow {
	catsRaw, _ := data["categories"].([]any)
	out := make([]menuCategoryRow, 0, len(catsRaw))
	for _, cRaw := range catsRaw {
		c, ok := cRaw.(map[string]any)
		if !ok {
			continue
		}
		row := menuCategoryRow{}
		row.ID, _ = c["id"].(string)
		row.Name, _ = c["name"].(string)
		row.Slug, _ = c["slug"].(string)
		row.Description, _ = c["description"].(string)
		if ids, ok := c["item_ids"].([]any); ok {
			row.ItemCount = len(ids)
		}
		out = append(out, row)
	}
	return out
}

func formatMenuPrice(cents int, currency string) string {
	whole := cents / 100
	frac := cents % 100
	sym := currency
	switch strings.ToUpper(currency) {
	case "EUR":
		sym = "€"
	case "USD":
		sym = "$"
	case "GBP":
		sym = "£"
	case "ILS":
		sym = "₪"
	}
	return fmt.Sprintf("%s%d.%02d", sym, whole, frac)
}
