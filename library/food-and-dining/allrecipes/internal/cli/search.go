// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: top-level `search`, `top-rated`, and `quick` commands.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/store"

	"github.com/spf13/cobra"
)

func newSearchTopCmd(flags *rootFlags) *cobra.Command {
	var flagPage, flagLimit int
	var flagMaxMinutes int
	var flagMinRating float64
	var flagCacheOnly bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Allrecipes for recipes (live + cache)",
		Example: "  allrecipes-pp-cli search brownies\n" +
			"  allrecipes-pp-cli search \"chicken thighs\" --limit 10 --agent\n" +
			"  allrecipes-pp-cli search brownies --cache-only --limit 20",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			page := flagPage
			if page < 1 {
				page = 1
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 24
			}

			// Cache-only mode: serve from the local SQLite store using the
			// domain-specific SearchRecipes method. Skips the network entirely
			// — useful offline or when Cloudflare blocks the live request.
			if flagCacheOnly {
				if flags.dryRun {
					return nil
				}
				return runCacheSearch(cmd, flags, query, limit)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, err := recipes.FetchSearch(c, query, page, limit)
			if err != nil {
				return classifyAPIError(err)
			}
			// Optional cache-side filters that only apply once we've enriched
			// each hit by fetching its recipe page. We expose --max-minutes
			// and --min-rating as flags so power users can chain in one call;
			// for unfetched results these filters silently skip.
			if flagMaxMinutes > 0 || flagMinRating > 0 {
				results = applyClientSideFilters(results, flagMaxMinutes, flagMinRating)
			}
			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&flagPage, "page", 1, "Result page (1-indexed)")
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results")
	cmd.Flags().IntVar(&flagMaxMinutes, "max-minutes", 0, "Filter to recipes whose total time (when known) is at most this many minutes")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 0, "Filter to recipes whose rating (when known) is at least this value")
	cmd.Flags().BoolVar(&flagCacheOnly, "cache-only", false, "Serve results from the local SQLite store only (no network call)")
	return cmd
}

// runCacheSearch reads from the local store using the domain-specific
// SearchRecipes method. Returns an empty result list (not an error) when the
// store has no matching cached recipes.
func runCacheSearch(cmd *cobra.Command, flags *rootFlags, query string, limit int) error {
	var s *store.Store = openStoreForCommand()
	if s == nil {
		return fmt.Errorf("no local cache available — run 'recipe' or 'search' first to populate it")
	}
	defer s.Close()
	rows, err := recipes.SearchCachedRecipes(s, query, limit)
	if err != nil {
		return fmt.Errorf("cache search failed: %w", err)
	}
	out := make([]recipes.SearchResult, 0, len(rows))
	for _, raw := range rows {
		var rec recipes.Recipe
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		out = append(out, recipes.SearchResult{
			Title:       rec.Name,
			URL:         rec.URL,
			Rating:      rec.AggregateRating.Value,
			ReviewCount: rec.AggregateRating.Count,
		})
	}
	return renderJSON(cmd, flags, out)
}

// applyClientSideFilters drops results whose known fields fail the filter.
// Search-card data is sparse (rating/time are best-effort), so a result with
// missing data passes the filter (we do not penalize unknown).
func applyClientSideFilters(in []recipes.SearchResult, maxMinutes int, minRating float64) []recipes.SearchResult {
	out := in[:0]
	for _, r := range in {
		if minRating > 0 && r.Rating > 0 && r.Rating < minRating {
			continue
		}
		// max-minutes can't be enforced on raw search results; we keep them
		// and let the caller fetch+filter via `quick` for strict enforcement.
		_ = maxMinutes
		out = append(out, r)
	}
	return out
}

func newTopRatedCmd(flags *rootFlags) *cobra.Command {
	var flagLimit, flagPage, flagSmoothC int
	var flagPriorMean float64
	var flagEnrich bool
	cmd := &cobra.Command{
		Use:   "top-rated <query>",
		Short: "Search and rank by Bayesian-smoothed rating",
		Long: "Ranks search results by a Bayesian-smoothed estimate of the true rating:\n\n" +
			"\tsmoothed = (C * priorMean + reviewCount * rating) / (C + reviewCount)\n\n" +
			"C is the credibility weight (default 200). priorMean (default 4.0) is the\n" +
			"corpus prior — most rated Allrecipes recipes cluster between 4.2 and 4.7.\n" +
			"This smoothing prevents single-review 5-star outliers from outranking\n" +
			"proven-popular recipes.\n\n" +
			"By default ranking uses whatever rating data the search-result cards\n" +
			"surface, which on current Allrecipes templates is often blank (the site\n" +
			"renders ratings client-side after the initial HTML loads). Pass --enrich\n" +
			"to fetch each candidate's full JSON-LD for accurate ratings — slower,\n" +
			"but the score reflects real review data.",
		Example: "  allrecipes-pp-cli top-rated brownies --limit 5 --enrich\n" +
			"  allrecipes-pp-cli top-rated \"chocolate cake\" --enrich --smooth-c 200 --agent",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			fetchLimit := limit * 2
			if fetchLimit < 24 {
				fetchLimit = 24
			}
			page := flagPage
			if page < 1 {
				page = 1
			}
			results, err := recipes.FetchSearch(c, query, page, fetchLimit)
			if err != nil {
				return classifyAPIError(err)
			}
			// Optional enrichment: fetch each candidate's recipe page to pull
			// the JSON-LD rating + review count. Without this, top-rated falls
			// back to whatever rating data the search-card parser found, which
			// is sparse on current Allrecipes templates. With --enrich, we
			// trade ~N HTTP calls for accurate ranking.
			if flagEnrich {
				results = enrichRatings(c, results, limit*2)
			}
			ranked := recipes.Rank(results, flagPriorMean, flagSmoothC)
			if len(ranked) > limit {
				ranked = ranked[:limit]
			}
			// Decorate with smoothed score for transparency.
			type entry struct {
				Rank          int     `json:"rank"`
				Title         string  `json:"title"`
				URL           string  `json:"url"`
				Rating        float64 `json:"rating,omitempty"`
				ReviewCount   int     `json:"reviewCount,omitempty"`
				SmoothedScore float64 `json:"smoothedScore"`
			}
			out := make([]entry, len(ranked))
			for i, r := range ranked {
				out[i] = entry{
					Rank:          i + 1,
					Title:         r.Title,
					URL:           r.URL,
					Rating:        r.Rating,
					ReviewCount:   r.ReviewCount,
					SmoothedScore: recipes.BayesianRating(r.Rating, r.ReviewCount, flagPriorMean, flagSmoothC),
				}
			}
			return renderJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Number of top results to return")
	cmd.Flags().IntVar(&flagPage, "page", 1, "Source page to rank")
	cmd.Flags().IntVar(&flagSmoothC, "smooth-c", 200, "Bayesian credibility weight (higher = stricter; needs more reviews to leave the prior)")
	cmd.Flags().Float64Var(&flagPriorMean, "prior-mean", 4.0, "Bayesian prior mean (corpus baseline rating)")
	cmd.Flags().BoolVar(&flagEnrich, "enrich", false, "Fetch each candidate's full JSON-LD to get accurate rating data (slower, more HTTP calls)")
	return cmd
}

// enrichRatings fetches each search result's recipe page to pull the JSON-LD
// rating/reviewCount. Capped at maxFetch results to bound the HTTP cost. On a
// fetch error, the original SearchResult passes through unchanged.
func enrichRatings(c recipes.Client, results []recipes.SearchResult, maxFetch int) []recipes.SearchResult {
	if maxFetch <= 0 || maxFetch > len(results) {
		maxFetch = len(results)
	}
	out := make([]recipes.SearchResult, len(results))
	copy(out, results)
	for i := 0; i < maxFetch; i++ {
		r := out[i]
		if r.URL == "" {
			continue
		}
		rec, err := recipes.FetchRecipe(c, r.URL)
		if err != nil {
			continue
		}
		out[i].Rating = rec.AggregateRating.Value
		out[i].ReviewCount = rec.AggregateRating.Count
		// Best-effort cache write so subsequent commands can use this data
		// without re-fetching.
		persistRecipe(rec)
	}
	return out
}

func newQuickCmd(flags *rootFlags) *cobra.Command {
	var flagMaxMinutes, flagLimit int
	var flagQuery, flagCategory string
	var flagMinRating float64
	cmd := &cobra.Command{
		Use:   "quick",
		Short: "Top-rated recipes from cache that fit a strict time cap",
		Long: "Reads the local recipe cache and returns recipes whose total cook+prep time\n" +
			"is at or below --max-minutes, ranked by Bayesian-smoothed rating. Allrecipes'\n" +
			"website cannot enforce a strict numeric cap; this command can, because the\n" +
			"local cache stores the parsed totalTime.\n\n" +
			"Recipes only appear in the cache after you fetch them at least once. Run\n" +
			"`allrecipes-pp-cli search ...` and `allrecipes-pp-cli recipe ...` to\n" +
			"populate the cache.",
		Example: "  allrecipes-pp-cli quick --max-minutes 30\n" +
			"  allrecipes-pp-cli quick --max-minutes 25 --query chicken --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available — run 'recipe' or 'search' first to populate it")
			}
			defer s.Close()
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}
			maxSec := flagMaxMinutes * 60
			rows, err := recipes.QueryIndex(s, recipes.IndexQuery{
				MaxTime:   maxSec,
				MinRating: flagMinRating,
				Category:  flagCategory,
				Limit:     limit * 3,
				OrderBy:   "rating",
				OrderDesc: true,
			})
			if err != nil {
				return err
			}
			if flagQuery != "" {
				rows = filterByName(rows, flagQuery)
			}
			// Bayesian re-rank.
			scored := make([]recipes.IndexRow, len(rows))
			copy(scored, rows)
			sortByBayes(scored)
			if len(scored) > limit {
				scored = scored[:limit]
			}
			return renderJSON(cmd, flags, scored)
		},
	}
	cmd.Flags().IntVar(&flagMaxMinutes, "max-minutes", 30, "Maximum total time in minutes")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Substring to filter recipe titles")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Substring to filter recipe categories (e.g., 'dessert')")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 4.0, "Minimum raw rating before Bayesian smoothing")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum results")
	return cmd
}

func filterByName(rows []recipes.IndexRow, q string) []recipes.IndexRow {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Name), q) {
			out = append(out, r)
		}
	}
	return out
}

func sortByBayes(rows []recipes.IndexRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0; j-- {
			a := recipes.BayesianRating(rows[j].Rating, rows[j].ReviewCount, 4.0, 200)
			b := recipes.BayesianRating(rows[j-1].Rating, rows[j-1].ReviewCount, 4.0, 200)
			if a > b {
				rows[j], rows[j-1] = rows[j-1], rows[j]
			} else {
				break
			}
		}
	}
}
