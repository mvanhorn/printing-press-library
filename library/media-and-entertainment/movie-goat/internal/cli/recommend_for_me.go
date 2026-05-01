package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newRecommendForMeCmd(flags *rootFlags) *cobra.Command {
	var flagStreaming string

	cmd := &cobra.Command{
		Use:   "recommend-for-me <title1> [title2] [title3...]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get movie recommendations based on movies you love",
		Long: `Provide 2 or more movies you enjoy. The algorithm fetches recommendations
and similar movies for each, then scores candidates by how well they bridge
your taste — movies matching genres from multiple inputs rank highest.

Works best with diverse inputs: mixing genres helps find cross-taste gems.`,
		Example: `  movie-goat-pp-cli recommend-for-me "The Matrix" "Saving Private Ryan" "Sisterhood of the Traveling Pants"
  movie-goat-pp-cli recommend-for-me "The Dark Knight" "Inception" "Interstellar"
  movie-goat-pp-cli recommend-for-me "Parasite" "Oldboy" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Provide at least 2 movie titles for meaningful recommendations.\n")
				if len(args) == 0 {
					return cmd.Help()
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type recEntry struct {
				ID             int      `json:"id"`
				Title          string   `json:"title"`
				VoteAverage    float64  `json:"vote_average"`
				ReleaseDate    string   `json:"release_date"`
				Overview       string   `json:"overview"`
				Popularity     float64  `json:"popularity"`
				GenreIDs       []int    `json:"genre_ids"`
				Count          int      `json:"match_count"`
				Sources        []string `json:"recommended_by"`
				GenreScore     int      `json:"genre_score"`
				CompositeScore float64  `json:"composite_score"`
			}

			recMap := make(map[int]*recEntry)
			inputIDs := make(map[int]bool)

			// Collect all genres from input movies for cross-taste scoring
			inputGenres := make(map[int]int) // genre ID → count of input movies with this genre

			for _, title := range args {
				movieID, resolvedTitle, err := resolveMovieID(c, title)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %q: %v\n", title, err)
					continue
				}
				inputIDs[movieID] = true
				if resolvedTitle == "" {
					resolvedTitle = title
				}

				// Fetch movie details to get genre IDs
				detailPath := fmt.Sprintf("/movie/%d", movieID)
				detailData, detailErr := c.Get(detailPath, map[string]string{})
				if detailErr == nil {
					var detail struct {
						Genres []struct {
							ID int `json:"id"`
						} `json:"genres"`
						GenreIDs []int `json:"genre_ids"`
					}
					if json.Unmarshal(detailData, &detail) == nil {
						for _, g := range detail.Genres {
							inputGenres[g.ID]++
						}
						for _, gid := range detail.GenreIDs {
							inputGenres[gid]++
						}
					}
				}

				// Fetch both recommendations AND similar movies
				candidateCount := 0
				for _, endpoint := range []string{"recommendations", "similar"} {
					path := fmt.Sprintf("/movie/%d/%s", movieID, endpoint)
					data, getErr := c.Get(path, map[string]string{})
					if getErr != nil {
						continue
					}
					var resp tmdbSearchResponse
					if json.Unmarshal(data, &resp) != nil {
						continue
					}
					candidateCount += len(resp.Results)
					for _, r := range resp.Results {
						if existing, ok := recMap[r.ID]; ok {
							existing.Count++
							found := false
							for _, s := range existing.Sources {
								if s == resolvedTitle {
									found = true
									break
								}
							}
							if !found {
								existing.Sources = append(existing.Sources, resolvedTitle)
							}
						} else {
							recMap[r.ID] = &recEntry{
								ID:          r.ID,
								Title:       r.DisplayTitle(),
								VoteAverage: r.VoteAverage,
								ReleaseDate: r.ReleaseDate,
								Overview:    r.Overview,
								Popularity:  r.Popularity,
								GenreIDs:    r.GenreIDs,
								Count:       1,
								Sources:     []string{resolvedTitle},
							}
						}
					}
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Found %d candidates for %q\n", candidateCount, resolvedTitle)
			}

			// Score each candidate by genre bridge — how many input genres it covers
			for id, entry := range recMap {
				if inputIDs[id] {
					continue
				}
				genreScore := 0
				for _, gid := range entry.GenreIDs {
					if count, ok := inputGenres[gid]; ok {
						genreScore += count
					}
				}
				entry.GenreScore = genreScore
				// Composite: genre bridge × list appearances × rating
				entry.CompositeScore = float64(genreScore) * float64(entry.Count) * entry.VoteAverage
			}

			// Collect results, excluding input movies
			var results []*recEntry
			for id, entry := range recMap {
				if inputIDs[id] {
					continue
				}
				results = append(results, entry)
			}

			// Sort by composite score descending
			sort.Slice(results, func(i, j int) bool {
				return results[i].CompositeScore > results[j].CompositeScore
			})

			// Limit to top 10
			if len(results) > 10 {
				results = results[:10]
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recommendations found.")
				return nil
			}

			// Filter by streaming provider if requested
			if flagStreaming != "" {
				var filtered []*recEntry
				for _, r := range results {
					path := fmt.Sprintf("/movie/%d/watch/providers", r.ID)
					data, getErr := c.Get(path, map[string]string{})
					if getErr != nil {
						continue
					}
					var providers tmdbWatchProviders
					if json.Unmarshal(data, &providers) != nil {
						continue
					}
					if us, ok := providers.Results["US"]; ok {
						for _, p := range us.Flatrate {
							if strings.EqualFold(p.ProviderName, flagStreaming) ||
								fmt.Sprintf("%d", p.ProviderID) == flagStreaming {
								filtered = append(filtered, r)
								break
							}
						}
					}
				}
				if len(filtered) > 0 {
					results = filtered
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "No results available on %q. Showing all recommendations.\n", flagStreaming)
				}
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(results)
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
			fmt.Fprintf(w, "Recommendations Based On Your Taste\n")
			fmt.Fprintf(w, "===================================\n")
			fmt.Fprintf(w, "Your genres: ")
			genreNames := []string{}
			for gid := range inputGenres {
				genreNames = append(genreNames, fmt.Sprintf("%d", gid))
			}
			fmt.Fprintf(w, "(scoring by genre bridge across %d input genres)\n\n", len(inputGenres))

			tw := newTabWriter(w)
			fmt.Fprintln(tw, "#\tTitle\tRating\tYear\tGenre Bridge\tFrom")
			for i, r := range results {
				year := ""
				if len(r.ReleaseDate) >= 4 {
					year = r.ReleaseDate[:4]
				}
				sources := strings.Join(r.Sources, ", ")
				if len(sources) > 35 {
					sources = sources[:32] + "..."
				}
				bridgeLabel := fmt.Sprintf("%d genres", r.GenreScore)
				fmt.Fprintf(tw, "%d\t%s\t%.1f\t%s\t%s\t%s\n",
					i+1, truncate(r.Title, 30), r.VoteAverage, year, bridgeLabel, sources)
			}
			tw.Flush()

			return nil
		},
	}

	cmd.Flags().StringVar(&flagStreaming, "streaming", "", "Filter results by streaming provider name or ID")

	return cmd
}
