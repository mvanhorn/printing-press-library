package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newTonightCmd(flags *rootFlags) *cobra.Command {
	var flagGenre string
	var flagType string

	cmd := &cobra.Command{
		Use:   "tonight",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Smart \"what should I watch tonight?\" recommendation",
		Long: `Combines trending and popular movies to surface the best picks for tonight.
Fetches today's trending movies and currently popular top-rated movies,
deduplicates them, and ranks by a weighted score (vote_average * popularity).
Shows the top 5 picks with title, year, rating, genre IDs, and overview.`,
		Example: `  movie-goat-pp-cli tonight
  movie-goat-pp-cli tonight --genre 28
  movie-goat-pp-cli tonight --type tv
  movie-goat-pp-cli tonight --json
  movie-goat-pp-cli tonight --json --select title,vote_average`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			mediaType := flagType
			if mediaType == "" {
				mediaType = "movie"
			}

			// Fetch trending for today
			trendingPath := fmt.Sprintf("/trending/%s/day", mediaType)
			trendingData, err := c.Get(trendingPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}

			var trendingResp tmdbSearchResponse
			if err := json.Unmarshal(trendingData, &trendingResp); err != nil {
				return fmt.Errorf("parsing trending results: %w", err)
			}

			// Fetch popular
			popularPath := fmt.Sprintf("/%s/popular", mediaType)
			popularData, err := c.Get(popularPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}

			var popularResp tmdbSearchResponse
			if err := json.Unmarshal(popularData, &popularResp); err != nil {
				return fmt.Errorf("parsing popular results: %w", err)
			}

			// Combine and deduplicate by ID
			type tonightPick struct {
				ID          int     `json:"id"`
				Title       string  `json:"title"`
				Year        string  `json:"year"`
				VoteAverage float64 `json:"vote_average"`
				Popularity  float64 `json:"popularity"`
				Overview    string  `json:"overview"`
				GenreIDs    []int   `json:"genre_ids"`
				MediaType   string  `json:"media_type"`
				Score       float64 `json:"weighted_score"`
			}

			seen := make(map[int]bool)
			var picks []tonightPick

			addResults := func(results []tmdbSearchResult) {
				for _, r := range results {
					if seen[r.ID] {
						continue
					}
					seen[r.ID] = true

					// Parse genre_ids from the raw JSON since the struct doesn't carry them
					pick := tonightPick{
						ID:          r.ID,
						Title:       r.DisplayTitle(),
						Year:        r.Year(),
						VoteAverage: r.VoteAverage,
						Popularity:  r.Popularity,
						Overview:    r.Overview,
						MediaType:   mediaType,
						Score:       r.VoteAverage * r.Popularity,
					}
					picks = append(picks, pick)
				}
			}

			addResults(trendingResp.Results)
			addResults(popularResp.Results)

			// Parse genre_ids from raw JSON for filtering
			// We need to re-parse both responses to get genre_ids
			type rawResult struct {
				ID       int   `json:"id"`
				GenreIDs []int `json:"genre_ids"`
			}
			genreMap := make(map[int][]int)

			var trendingRaw struct {
				Results []rawResult `json:"results"`
			}
			if json.Unmarshal(trendingData, &trendingRaw) == nil {
				for _, r := range trendingRaw.Results {
					genreMap[r.ID] = r.GenreIDs
				}
			}

			var popularRaw struct {
				Results []rawResult `json:"results"`
			}
			if json.Unmarshal(popularData, &popularRaw) == nil {
				for _, r := range popularRaw.Results {
					if _, ok := genreMap[r.ID]; !ok {
						genreMap[r.ID] = r.GenreIDs
					}
				}
			}

			// Attach genre IDs to picks
			for i := range picks {
				picks[i].GenreIDs = genreMap[picks[i].ID]
			}

			// Filter by genre if specified
			if flagGenre != "" {
				var filtered []tonightPick
				for _, p := range picks {
					for _, gid := range p.GenreIDs {
						if fmt.Sprintf("%d", gid) == flagGenre {
							filtered = append(filtered, p)
							break
						}
					}
				}
				picks = filtered
			}

			// Sort by weighted score descending
			sort.Slice(picks, func(i, j int) bool {
				return picks[i].Score > picks[j].Score
			})

			// Top 5
			if len(picks) > 5 {
				picks = picks[:5]
			}

			if len(picks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No picks found matching your criteria. Try removing the --genre filter.")
				return nil
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(picks)
				if flags.compact {
					data = compactFields(json.RawMessage(data))
				}
				if flags.selectFields != "" {
					data = filterFields(json.RawMessage(data), flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(data), true)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Tonight's Top Picks")
			fmt.Fprintln(w, "===================")
			fmt.Fprintln(w)

			for i, p := range picks {
				fmt.Fprintf(w, "%d. %s", i+1, p.Title)
				if p.Year != "" {
					fmt.Fprintf(w, " (%s)", p.Year)
				}
				fmt.Fprintln(w)
				fmt.Fprintf(w, "   Rating: %.1f/10  |  Popularity: %.0f  |  Score: %.0f\n", p.VoteAverage, p.Popularity, p.Score)

				if len(p.GenreIDs) > 0 {
					genreStrs := make([]string, len(p.GenreIDs))
					for j, gid := range p.GenreIDs {
						genreStrs[j] = fmt.Sprintf("%d", gid)
					}
					fmt.Fprintf(w, "   Genres: %s\n", strings.Join(genreStrs, ", "))
				}

				if p.Overview != "" {
					overview := p.Overview
					if len(overview) > 200 {
						overview = overview[:197] + "..."
					}
					fmt.Fprintf(w, "   %s\n", overview)
				}
				fmt.Fprintln(w)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flagGenre, "genre", "", "Filter by genre ID (e.g. 28=Action, 35=Comedy, 878=Sci-Fi)")
	cmd.Flags().StringVar(&flagType, "type", "movie", "Media type: movie or tv")

	return cmd
}
