// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
	"github.com/spf13/cobra"
)

type menuItemRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Price       string `json:"price"`
	PriceCents  int    `json:"price_cents"`
	Popular     bool   `json:"popular"`
	HasCoupon   bool   `json:"has_coupon"`
	Description string `json:"description,omitempty"`
}

func newMenuCmd(flags *rootFlags) *cobra.Command {
	var category string
	var popularOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:   "menu <restaurant-id>",
		Short: "Browse a restaurant's full menu by id",
		Long: "Browse the full menu for a Grubhub restaurant id (get ids from 'near' or 'compare').\n\n" +
			"Use this to read one known restaurant's menu. To find which nearby restaurants carry a specific dish, use 'dish' instead.",
		Example:     "  grubhub-pp-cli menu 1414955 --popular",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the restaurant menu")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a restaurant id is required, e.g. menu 1414955"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := grubhubClient(ctx, flags)
			if err != nil {
				return err
			}
			raw, err := c.Get(ctx, "/restaurants/"+args[0], map[string]string{
				"version":                  "4",
				"orderType":                "standard",
				"showMenuItemCoupons":      "true",
				"includePromos":            "true",
				"hideUnavailableMenuItems": "true",
			})
			if err != nil {
				return err
			}
			name, items, err := grubhub.ParseMenu(raw)
			if err != nil {
				return err
			}
			rows := menuRowsFromItems(items, category, popularOnly)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"restaurant_id":   args[0],
					"restaurant_name": name,
					"count":           len(rows),
					"items":           rows,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d items\n\n", name, len(rows))
			return renderMenuTable(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Only show items in categories matching this text")
	cmd.Flags().BoolVar(&popularOnly, "popular", false, "Only show popular-flagged items")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum items to return (0 = all)")
	return cmd
}

func menuRowsFromItems(items []grubhub.MenuItem, category string, popularOnly bool) []menuItemRow {
	category = strings.ToLower(strings.TrimSpace(category))
	rows := make([]menuItemRow, 0, len(items))
	for _, it := range items {
		if popularOnly && !it.Popular {
			continue
		}
		if category != "" && !strings.Contains(strings.ToLower(it.Category), category) {
			continue
		}
		rows = append(rows, menuItemRow{
			ID:          it.ID,
			Name:        cliutil.CleanText(it.Name),
			Category:    it.Category,
			Price:       grubhub.Dollars(it.PriceCents()),
			PriceCents:  it.PriceCents(),
			Popular:     it.Popular,
			HasCoupon:   it.ItemCoupon,
			Description: cliutil.CleanText(it.Description),
		})
	}
	return rows
}

func renderMenuTable(cmd *cobra.Command, rows []menuItemRow) error {
	out := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(out, "No menu items found.")
		return nil
	}
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "ITEM\tPRICE\tCATEGORY\tPOPULAR\tID")
	for _, r := range rows {
		pop := ""
		if r.Popular {
			pop = "★"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", truncate(r.Name, 36), r.Price, truncate(r.Category, 20), pop, r.ID)
	}
	return tw.Flush()
}
