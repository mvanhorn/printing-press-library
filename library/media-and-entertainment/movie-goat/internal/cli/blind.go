package cli

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb"
)

func newBlindCmd(flags *rootFlags) *cobra.Command {
	var flagGenre string
	var flagDecade string
	var flagMinRating float64
	var flagStreaming string
	var flagReroll bool

	cmd := &cobra.Command{
		Use:   "blind",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get a random high-quality movie pick — the surprise me experience",
		Long: `Can't decide what to watch? Let fate decide. Picks a random well-rated movie
using TMDb's discover API with filters for genre, decade, minimum rating,
and streaming availability.`,
		Example: `  movie-goat-pp-cli blind
  movie-goat-pp-cli blind --genre 878 --min-rating 7.5
  movie-goat-pp-cli blind --decade 1990s
  movie-goat-pp-cli blind --streaming 8 --min-rating 8.0
  movie-goat-pp-cli blind --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"sort_by":          "vote_average.desc",
				"vote_count.gte":   "200",
				"vote_average.gte": fmt.Sprintf("%.1f", flagMinRating),
			}

			if flagGenre != "" {
				params["with_genres"] = flagGenre
			}

			if flagDecade != "" {
				decade := strings.TrimSuffix(strings.ToLower(flagDecade), "s")
				if len(decade) == 4 {
					year, _ := fmt.Sscanf(decade, "%d")
					if year > 0 {
						// Parse succeeded, decade already has the value
					}
				}
				// Support formats: "1990", "1990s", "90s"
				var startYear int
				if len(decade) == 2 {
					fmt.Sscanf(decade, "%d", &startYear)
					if startYear >= 0 && startYear <= 30 {
						startYear += 2000
					} else {
						startYear += 1900
					}
				} else {
					fmt.Sscanf(decade, "%d", &startYear)
				}
				if startYear > 0 {
					params["primary_release_date.gte"] = fmt.Sprintf("%d-01-01", startYear)
					params["primary_release_date.lte"] = fmt.Sprintf("%d-12-31", startYear+9)
				}
			}

			if flagStreaming != "" {
				params["with_watch_providers"] = flagStreaming
				params["watch_region"] = "US"
			}

			// First, discover total pages available
			params["page"] = "1"
			data, err := c.Get("/discover/movie", params)
			if err != nil {
				return classifyAPIError(err)
			}

			var firstPage tmdbSearchResponse
			if err := json.Unmarshal(data, &firstPage); err != nil {
				return fmt.Errorf("parsing discover results: %w", err)
			}

			if firstPage.TotalResults == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No movies found matching your criteria. Try relaxing the filters.")
				return nil
			}

			// Pick a random page (TMDb caps at 500 pages)
			maxPage := firstPage.TotalPages
			if maxPage > 500 {
				maxPage = 500
			}

			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			// How many attempts to make (for reroll or initial pick)
			attempts := 1
			if flagReroll {
				attempts = 5
			}

			var picked *tmdbMovieDetail
			var pickedRaw json.RawMessage
			usedIndices := make(map[string]bool)

			for attempt := 0; attempt < attempts; attempt++ {
				randomPage := rng.Intn(maxPage) + 1
				params["page"] = fmt.Sprintf("%d", randomPage)

				pageData, err := c.Get("/discover/movie", params)
				if err != nil {
					continue
				}

				var pageResp tmdbSearchResponse
				if json.Unmarshal(pageData, &pageResp) != nil || len(pageResp.Results) == 0 {
					continue
				}

				// Pick a random movie from this page
				idx := rng.Intn(len(pageResp.Results))
				key := fmt.Sprintf("%d-%d", randomPage, idx)
				if usedIndices[key] {
					continue
				}
				usedIndices[key] = true

				movieID := pageResp.Results[idx].ID
				detail, raw, err := getMovieDetail(c, movieID, "credits,watch/providers,external_ids")
				if err != nil {
					continue
				}
				picked = detail
				pickedRaw = raw

				if !flagReroll {
					break
				}
				// For reroll, show options and keep the last one
				fmt.Fprintf(cmd.ErrOrStderr(), "Roll %d: %s (%s) - %.1f/10\n",
					attempt+1, detail.Title, detail.ReleaseDate, detail.VoteAverage)
			}

			if picked == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Could not find a movie. Try different filters.")
				return nil
			}

			// OMDb enrichment
			cfg, _ := config.Load(flags.configPath)
			omdbKey := ""
			if cfg != nil {
				omdbKey = cfg.OmdbApiKey
			}
			var omdbResult *omdb.Result
			if omdbKey != "" && picked.ImdbID != "" {
				omdbResult, _ = omdb.Fetch(picked.ImdbID, omdbKey)
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if omdbResult != nil {
					pickedRaw = injectOMDb(pickedRaw, omdbResult)
				}
				if flags.compact {
					pickedRaw = compactFields(pickedRaw)
				}
				if flags.selectFields != "" {
					pickedRaw = filterFields(pickedRaw, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), pickedRaw, true)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Your Blind Pick")
			fmt.Fprintln(w, "===============")
			fmt.Fprintln(w)
			fmt.Fprintf(w, "Title:    %s\n", picked.Title)
			if picked.Tagline != "" {
				fmt.Fprintf(w, "Tagline:  %q\n", picked.Tagline)
			}
			year := ""
			if len(picked.ReleaseDate) >= 4 {
				year = picked.ReleaseDate[:4]
			}
			fmt.Fprintf(w, "Year:     %s\n", year)
			fmt.Fprintf(w, "Runtime:  %s\n", fmtRuntime(picked.Runtime))
			fmt.Fprintf(w, "Genres:   %s\n", genreNames(picked))
			fmt.Fprintln(w)

			// Ratings
			fmt.Fprintln(w, "Ratings:")
			fmt.Fprintf(w, "  TMDb:             %.1f/10\n", picked.VoteAverage)
			if omdbResult != nil {
				if omdbResult.ImdbRating != "" && omdbResult.ImdbRating != "N/A" {
					fmt.Fprintf(w, "  IMDb:             %s/10\n", omdbResult.ImdbRating)
				}
				if rt := omdbResult.RatingBySource("Rotten Tomatoes"); rt != "" {
					fmt.Fprintf(w, "  Rotten Tomatoes:  %s\n", rt)
				}
				if mc := omdbResult.RatingBySource("Metacritic"); mc != "" {
					fmt.Fprintf(w, "  Metacritic:       %s\n", mc)
				}
			}
			fmt.Fprintln(w)

			// Overview
			if picked.Overview != "" {
				fmt.Fprintf(w, "Overview:\n  %s\n", picked.Overview)
				fmt.Fprintln(w)
			}

			// Top cast
			if picked.Credits != nil && len(picked.Credits.Cast) > 0 {
				fmt.Fprintln(w, "Top Cast:")
				limit := 5
				if len(picked.Credits.Cast) < limit {
					limit = len(picked.Credits.Cast)
				}
				for _, m := range picked.Credits.Cast[:limit] {
					fmt.Fprintf(w, "  - %s as %s\n", m.Name, m.Character)
				}
				fmt.Fprintln(w)
			}

			// Director
			if picked.Credits != nil {
				for _, m := range picked.Credits.Crew {
					if m.Job == "Director" {
						fmt.Fprintf(w, "Director: %s\n", m.Name)
						break
					}
				}
			}

			if omdbResult != nil && omdbResult.Awards != "" && omdbResult.Awards != "N/A" {
				fmt.Fprintf(w, "Awards:   %s\n", omdbResult.Awards)
			}

			fmt.Fprintln(w)
			fmt.Fprintf(w, "TMDb ID: %d", picked.ID)
			if picked.ImdbID != "" {
				fmt.Fprintf(w, " | IMDb: https://www.imdb.com/title/%s/", picked.ImdbID)
			}
			fmt.Fprintln(w)

			return nil
		},
	}

	cmd.Flags().StringVar(&flagGenre, "genre", "", "Genre ID to filter by (e.g. 28=Action, 35=Comedy, 878=Sci-Fi)")
	cmd.Flags().StringVar(&flagDecade, "decade", "", "Decade filter (e.g. 1990s, 80s, 2010)")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 7.0, "Minimum TMDb rating (0-10)")
	cmd.Flags().StringVar(&flagStreaming, "streaming", "", "Watch provider ID to filter by (e.g. 8=Netflix, 337=Disney+)")
	cmd.Flags().BoolVar(&flagReroll, "reroll", false, "Roll 5 times and show all options")

	return cmd
}
