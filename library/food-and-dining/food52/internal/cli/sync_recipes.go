// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

// _ keeps the database/sql import live; storeRecipe* take *sql.DB.
var _ = sql.ErrNoRows

func newSyncRecipesCmd(flags *rootFlags) *cobra.Command {
	var (
		concurrency int
		summaryOnly bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "recipes <tag> [<tag>...]",
		Short: "Pull Food52 recipes for one or more tags into the local store (FTS-indexed)",
		Long: strings.TrimSpace(`
Walks one or more tag pages, fetches each recipe's full structured detail
(ingredients, steps, ratings, kitchen notes), and writes them into the local
SQLite store. After sync the recipes are searchable offline via 'search' and
matchable against your pantry via 'pantry match'.

Use --summary-only when you just want the tag listings (no per-recipe detail
fetch) — much faster but pantry match won't work because ingredients are
absent from the summary shape.
`),
		Example: strings.Trim(`
  food52-pp-cli sync recipes chicken
  food52-pp-cli sync recipes chicken vegetarian dessert --concurrency 6
  food52-pp-cli sync recipes pasta --summary-only
`, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openStoreOrErr()
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			summary := struct {
				Tags          []string `json:"tags"`
				RecipesPulled int      `json:"recipes_pulled"`
				DetailsPulled int      `json:"details_pulled"`
				Errors        []string `json:"errors,omitempty"`
			}{Tags: args}

			for _, tag := range args {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				html, err := fetchHTML(c, "/recipes/"+tag, nil)
				if err != nil {
					summary.Errors = append(summary.Errors, fmt.Sprintf("tag %s: %v", tag, err))
					continue
				}
				summaries, _, err := food52.ExtractRecipesByTag(html)
				if err != nil {
					summary.Errors = append(summary.Errors, fmt.Sprintf("tag %s extract: %v", tag, err))
					continue
				}
				if limit > 0 && len(summaries) > limit {
					summaries = summaries[:limit]
				}

				// Store each summary first (always), with the tag column set.
				for _, rs := range summaries {
					if err := storeRecipeSummary(db.DB(), rs, tag); err != nil {
						summary.Errors = append(summary.Errors, fmt.Sprintf("store summary %s: %v", rs.Slug, err))
						continue
					}
					summary.RecipesPulled++
				}

				if summaryOnly {
					continue
				}

				// Fetch full details with bounded concurrency.
				det := fetchDetailsConcurrent(c, summaries, concurrency)
				for _, fd := range det {
					if fd.err != nil {
						summary.Errors = append(summary.Errors, fmt.Sprintf("detail %s: %v", fd.slug, fd.err))
						continue
					}
					if err := storeRecipeDetail(db.DB(), fd.recipe, tag); err != nil {
						summary.Errors = append(summary.Errors, fmt.Sprintf("store detail %s: %v", fd.slug, err))
						continue
					}
					summary.DetailsPulled++
				}
			}

			return emitFromFlags(flags, summary, func() {
				fmt.Printf("Synced recipes for %d tag(s)\n", len(args))
				fmt.Printf("  summaries:  %d\n", summary.RecipesPulled)
				fmt.Printf("  details:    %d\n", summary.DetailsPulled)
				if n := len(summary.Errors); n > 0 {
					fmt.Printf("  errors:     %d (run with --json for details)\n", n)
				}
			})
		},
	}
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Concurrent recipe-detail fetches (1-16)")
	cmd.Flags().BoolVar(&summaryOnly, "summary-only", false, "Skip per-recipe detail fetch (pantry match won't work after a summary-only sync)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of recipes pulled per tag (0 = all on the first page)")
	return cmd
}

type detailFetch struct {
	slug   string
	recipe *food52.Recipe
	err    error
}

func fetchDetailsConcurrent(c *client.Client, summaries []food52.RecipeSummary, n int) []detailFetch {
	if n <= 0 {
		n = 4
	}
	if n > 16 {
		n = 16
	}
	out := make([]detailFetch, len(summaries))
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	for i, rs := range summaries {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rs food52.RecipeSummary) {
			defer wg.Done()
			defer func() { <-sem }()
			html, err := fetchHTML(c, "/recipes/"+rs.Slug, nil)
			if err != nil {
				out[i] = detailFetch{slug: rs.Slug, err: err}
				return
			}
			r, err := food52.ExtractRecipe(html, canonicalRecipeURL(rs.Slug))
			out[i] = detailFetch{slug: rs.Slug, recipe: r, err: err}
		}(i, rs)
	}
	wg.Wait()
	return out
}

// storeRecipeSummary writes a RecipeSummary row keyed by ID with the tag column.
func storeRecipeSummary(db *sql.DB, rs food52.RecipeSummary, tag string) error {
	return execStoreRecipe(db, rs.ID, rs.Slug, tag, mustJSON(rs))
}

// storeRecipeDetail writes a Recipe row keyed by ID with the tag column. Overwrites any prior summary row for the same ID.
func storeRecipeDetail(db *sql.DB, r *food52.Recipe, tag string) error {
	if r == nil {
		return nil
	}
	return execStoreRecipe(db, r.ID, r.Slug, tag, mustJSON(r))
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
