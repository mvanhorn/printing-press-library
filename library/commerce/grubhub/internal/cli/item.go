// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
	"github.com/spf13/cobra"
)

func newItemCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "item <restaurant-id> <item-id>",
		Short: "Show a menu item's price and modifier options",
		Long: "Show full detail for a single menu item, including its modifier/choice categories (sizes, add-ons) and prices.\n\n" +
			"Get the restaurant id and item id from 'menu <restaurant-id>'.",
		Example:     "  grubhub-pp-cli item 1414955 278441811233",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the menu item detail")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a restaurant id and an item id are required, e.g. item 1414955 278441811233"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := grubhubClient(ctx, flags)
			if err != nil {
				return err
			}
			raw, err := c.Get(ctx, "/restaurants/"+args[0]+"/menu_items/"+args[1], map[string]string{
				"version":   "4",
				"orderType": "standard",
			})
			if err != nil {
				return err
			}
			item, err := grubhub.ParseItem(raw)
			if err != nil {
				return err
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, itemView(item))
			}
			return renderItem(cmd, item)
		},
	}
	return cmd
}

func itemView(item grubhub.ItemDetail) map[string]any {
	cats := make([]map[string]any, 0, len(item.ChoiceCategories))
	for _, cc := range item.ChoiceCategories {
		opts := make([]map[string]any, 0, len(cc.Options))
		for _, o := range cc.Options {
			opts = append(opts, map[string]any{
				"name":        cliutil.CleanText(o.Description),
				"price":       grubhub.Dollars(o.Price.Amount),
				"price_cents": o.Price.Amount,
			})
		}
		cats = append(cats, map[string]any{
			"name":    cc.Name,
			"min":     cc.Min,
			"max":     cc.Max,
			"options": opts,
		})
	}
	return map[string]any{
		"id":          item.ID,
		"name":        cliutil.CleanText(item.Name),
		"category":    item.Category,
		"description": cliutil.CleanText(item.Description),
		"price":       grubhub.Dollars(item.Price.Amount),
		"price_cents": item.Price.Amount,
		"choices":     cats,
	}
}

func renderItem(cmd *cobra.Command, item grubhub.ItemDetail) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s — %s\n", cliutil.CleanText(item.Name), grubhub.Dollars(item.Price.Amount))
	if d := cliutil.CleanText(item.Description); d != "" {
		fmt.Fprintf(out, "%s\n", d)
	}
	for _, cc := range item.ChoiceCategories {
		fmt.Fprintf(out, "\n%s (choose %d-%d):\n", cc.Name, cc.Min, cc.Max)
		tw := newTabWriter(out)
		for _, o := range cc.Options {
			fmt.Fprintf(tw, "  %s\t%s\n", truncate(cliutil.CleanText(o.Description), 40), grubhub.Dollars(o.Price.Amount))
		}
		_ = tw.Flush()
	}
	return nil
}
