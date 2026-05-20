// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newRecipesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipes",
		Short: "List and describe AI recipes (panel templates)",
	}
	cmd.AddCommand(newRecipesListCmd(flags))
	cmd.AddCommand(newRecipesDescribeCmd(flags))
	cmd.AddCommand(newRecipeCoverageCmd(flags))
	return cmd
}

func newRecipesListCmd(flags *rootFlags) *cobra.Command {
	var source, category, tag string
	var topUsage bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recipes (public/user/shared)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			recipes := c.RecipesAll()
			var filtered []granola.Recipe
			for _, r := range recipes {
				if source != "" && r.Source != source {
					continue
				}
				if category != "" && r.Category != category {
					continue
				}
				if tag != "" {
					t := strings.ToLower(tag)
					if !strings.Contains(strings.ToLower(r.Slug), t) && !strings.Contains(strings.ToLower(r.Config.Description), t) {
						continue
					}
				}
				filtered = append(filtered, r)
			}
			if topUsage {
				type ru struct {
					Recipe granola.Recipe
					Count  int64
				}
				items := make([]ru, 0, len(filtered))
				for _, r := range filtered {
					u := c.RecipesUsage[r.ID]
					var n int64
					fmt.Sscanf(u.TotalCount, "%d", &n)
					items = append(items, ru{Recipe: r, Count: n})
				}
				// Simple in-place insertion sort (small lists, ~60).
				for i := 1; i < len(items); i++ {
					for j := i; j > 0 && items[j-1].Count < items[j].Count; j-- {
						items[j-1], items[j] = items[j], items[j-1]
					}
				}
				out := make([]map[string]any, 0, len(items))
				for _, it := range items {
					out = append(out, map[string]any{
						"id":          it.Recipe.ID,
						"slug":        it.Recipe.Slug,
						"name":        it.Recipe.Name,
						"description": it.Recipe.Config.Description,
						"source":      it.Recipe.Source,
						"usage_count": it.Count,
					})
				}
				return emitJSON(cmd, flags, out)
			}
			out := make([]map[string]any, 0, len(filtered))
			for _, r := range filtered {
				out = append(out, map[string]any{
					"id":          r.ID,
					"slug":        r.Slug,
					"name":        r.Name,
					"description": r.Config.Description,
					"source":      r.Source,
				})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "public | user | shared")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	cmd.Flags().StringVar(&tag, "tag", "", "Substring match on slug or description")
	cmd.Flags().BoolVar(&topUsage, "top-usage", false, "Sort by usage_count desc")
	return cmd
}

func newRecipesDescribeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <slug-or-id>",
		Short: "Show one recipe and the meetings that used it",
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
			key := args[0]
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			var hit *granola.Recipe
			for i := range c.RecipesAll() {
				r := c.RecipesAll()[i]
				if r.Slug == key || r.ID == key {
					hit = &r
					break
				}
			}
			if hit == nil {
				return notFoundErr(fmt.Errorf("recipe %q not found", key))
			}
			usage := c.RecipesUsage[hit.ID]
			out := map[string]any{
				"id":           hit.ID,
				"slug":         hit.Slug,
				"name":         hit.Name,
				"description":  hit.Config.Description,
				"instructions": hit.Config.Instructions,
				"source":       hit.Source,
				"usage": map[string]any{
					"count":        usage.TotalCount,
					"last_used_at": usage.LastUsedAt,
				},
			}
			return emitJSON(cmd, flags, out)
		},
	}
	return cmd
}
