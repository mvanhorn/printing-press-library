package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb"
)

func newVersusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versus <title1> <title2>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compare two movies side-by-side: ratings, box office, cast, streaming",
		Long: `Compare two movies head-to-head across all dimensions: ratings (TMDb, IMDb,
Rotten Tomatoes, Metacritic), runtime, budget, revenue, genres, streaming
availability, and cast overlap.`,
		Example: `  movie-goat-pp-cli versus "The Dark Knight" "Inception"
  movie-goat-pp-cli versus "Alien" "Aliens"
  movie-goat-pp-cli versus "Barbie" "Oppenheimer" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				if len(args) >= 2 {
					fmt.Fprintf(cmd.OutOrStdout(), "GET /search/movie?query=%s\nGET /search/movie?query=%s\nGET /movie/<id1>?append_to_response=credits,watch/providers,external_ids\nGET /movie/<id2>?append_to_response=credits,watch/providers,external_ids\n", args[0], args[1])
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "GET /search/movie (resolve both titles)\nGET /movie/<id1>?append_to_response=credits,watch/providers,external_ids\nGET /movie/<id2>?append_to_response=credits,watch/providers,external_ids")
				}
				return nil
			}
			if len(args) < 2 {
				return fmt.Errorf("requires exactly 2 movie titles or IDs\n\nUsage: movie-goat-pp-cli versus <title1> <title2>")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve both movies
			id1, title1, err := resolveMovieID(c, args[0])
			if err != nil {
				return classifyAPIError(err)
			}
			id2, title2, err := resolveMovieID(c, args[1])
			if err != nil {
				return classifyAPIError(err)
			}

			// Fetch full details for both
			appendFields := "credits,watch/providers,external_ids"
			detail1, raw1, err := getMovieDetail(c, id1, appendFields)
			if err != nil {
				return classifyAPIError(err)
			}
			detail2, raw2, err := getMovieDetail(c, id2, appendFields)
			if err != nil {
				return classifyAPIError(err)
			}

			if title1 == "" {
				title1 = detail1.Title
			}
			if title2 == "" {
				title2 = detail2.Title
			}

			// OMDb enrichment
			cfg, _ := config.Load(flags.configPath)
			omdbKey := ""
			if cfg != nil {
				omdbKey = cfg.OmdbApiKey
			}

			var omdb1, omdb2 *omdb.Result
			if omdbKey != "" {
				if detail1.ImdbID != "" {
					omdb1, _ = omdb.Fetch(detail1.ImdbID, omdbKey)
				}
				if detail2.ImdbID != "" {
					omdb2, _ = omdb.Fetch(detail2.ImdbID, omdbKey)
				}
			}

			// Find cast overlap
			var castOverlap []string
			if detail1.Credits != nil && detail2.Credits != nil {
				cast1 := make(map[int]string)
				for _, m := range detail1.Credits.Cast {
					cast1[m.ID] = m.Name
				}
				for _, m := range detail2.Credits.Cast {
					if name, ok := cast1[m.ID]; ok {
						castOverlap = append(castOverlap, name)
					}
				}
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				output := map[string]any{
					"movie_1":      buildComparisonJSON(detail1, raw1, omdb1),
					"movie_2":      buildComparisonJSON(detail2, raw2, omdb2),
					"cast_overlap": castOverlap,
				}
				outData, _ := json.Marshal(output)
				if flags.compact {
					outData = compactFields(json.RawMessage(outData))
				}
				if flags.selectFields != "" {
					outData = filterFields(json.RawMessage(outData), flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(outData), true)
			}

			// Human-readable side-by-side comparison
			w := cmd.OutOrStdout()

			// Header
			col1 := truncate(title1, 30)
			col2 := truncate(title2, 30)
			fmt.Fprintf(w, "%-20s  %-30s  vs  %-30s\n", "", col1, col2)
			fmt.Fprintf(w, "%s\n", strings.Repeat("-", 86))

			// Basic info
			printVersusRow(w, "Release Date", detail1.ReleaseDate, detail2.ReleaseDate)
			printVersusRow(w, "Runtime", fmtRuntime(detail1.Runtime), fmtRuntime(detail2.Runtime))
			printVersusRow(w, "Genres", genreNames(detail1), genreNames(detail2))
			printVersusRow(w, "Status", detail1.Status, detail2.Status)

			// Ratings
			fmt.Fprintf(w, "\n%-20s  %-30s  vs  %-30s\n", "RATINGS", "", "")
			printVersusRow(w, "TMDb", fmt.Sprintf("%.1f/10 (%d votes)", detail1.VoteAverage, detail1.VoteCount),
				fmt.Sprintf("%.1f/10 (%d votes)", detail2.VoteAverage, detail2.VoteCount))

			if omdb1 != nil || omdb2 != nil {
				imdb1, imdb2 := "N/A", "N/A"
				rt1, rt2 := "N/A", "N/A"
				mc1, mc2 := "N/A", "N/A"
				if omdb1 != nil {
					if omdb1.ImdbRating != "" && omdb1.ImdbRating != "N/A" {
						imdb1 = omdb1.ImdbRating + "/10"
					}
					if v := omdb1.RatingBySource("Rotten Tomatoes"); v != "" {
						rt1 = v
					}
					if v := omdb1.RatingBySource("Metacritic"); v != "" {
						mc1 = v
					}
				}
				if omdb2 != nil {
					if omdb2.ImdbRating != "" && omdb2.ImdbRating != "N/A" {
						imdb2 = omdb2.ImdbRating + "/10"
					}
					if v := omdb2.RatingBySource("Rotten Tomatoes"); v != "" {
						rt2 = v
					}
					if v := omdb2.RatingBySource("Metacritic"); v != "" {
						mc2 = v
					}
				}
				printVersusRow(w, "IMDb", imdb1, imdb2)
				printVersusRow(w, "Rotten Tomatoes", rt1, rt2)
				printVersusRow(w, "Metacritic", mc1, mc2)
			}

			// Financials
			fmt.Fprintf(w, "\n%-20s  %-30s  vs  %-30s\n", "FINANCIALS", "", "")
			printVersusRow(w, "Budget", formatMoney(detail1.Budget), formatMoney(detail2.Budget))
			printVersusRow(w, "Revenue", formatMoney(detail1.Revenue), formatMoney(detail2.Revenue))

			if omdb1 != nil || omdb2 != nil {
				bo1, bo2 := "N/A", "N/A"
				if omdb1 != nil && omdb1.BoxOffice != "" && omdb1.BoxOffice != "N/A" {
					bo1 = omdb1.BoxOffice
				}
				if omdb2 != nil && omdb2.BoxOffice != "" && omdb2.BoxOffice != "N/A" {
					bo2 = omdb2.BoxOffice
				}
				printVersusRow(w, "Box Office", bo1, bo2)
			}

			// Awards
			if omdb1 != nil || omdb2 != nil {
				a1, a2 := "N/A", "N/A"
				if omdb1 != nil && omdb1.Awards != "" && omdb1.Awards != "N/A" {
					a1 = truncate(omdb1.Awards, 30)
				}
				if omdb2 != nil && omdb2.Awards != "" && omdb2.Awards != "N/A" {
					a2 = truncate(omdb2.Awards, 30)
				}
				printVersusRow(w, "Awards", a1, a2)
			}

			// Cast overlap
			if len(castOverlap) > 0 {
				fmt.Fprintf(w, "\nCast Overlap: %s\n", strings.Join(castOverlap, ", "))
			}

			return nil
		},
	}

	return cmd
}

func printVersusRow(w interface{ Write([]byte) (int, error) }, label, val1, val2 string) {
	fmt.Fprintf(w, "%-20s  %-30s  vs  %-30s\n", label, truncate(val1, 30), truncate(val2, 30))
}

func fmtRuntime(minutes int) string {
	if minutes == 0 {
		return "N/A"
	}
	h := minutes / 60
	m := minutes % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm (%d min)", h, m, minutes)
	}
	return fmt.Sprintf("%d min", minutes)
}

func buildComparisonJSON(detail *tmdbMovieDetail, raw json.RawMessage, omdbData *omdb.Result) map[string]any {
	result := map[string]any{
		"id":           detail.ID,
		"title":        detail.Title,
		"release_date": detail.ReleaseDate,
		"runtime":      detail.Runtime,
		"vote_average": detail.VoteAverage,
		"vote_count":   detail.VoteCount,
		"budget":       detail.Budget,
		"revenue":      detail.Revenue,
		"genres":       genreNames(detail),
		"tagline":      detail.Tagline,
		"imdb_id":      detail.ImdbID,
	}
	if omdbData != nil {
		result["omdb"] = omdbData
	}
	return result
}
