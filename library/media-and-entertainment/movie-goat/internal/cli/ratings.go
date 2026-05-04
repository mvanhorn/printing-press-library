package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb"
)

// ratingsOutput is the typed JSON shape produced by `movie ratings`. Keeping
// every field tagged (rather than building a map[string]any) means --select
// and --compact work reliably against the documented contract.
type ratingsOutput struct {
	TMDBID    int            `json:"tmdb_id"`
	Kind      string         `json:"kind"`
	Title     string         `json:"title"`
	Year      string         `json:"year"`
	IMDbID    string         `json:"imdb_id"`
	Ratings   ratingsValues  `json:"ratings"`
	TMDBVotes int            `json:"tmdb_votes"`
	IMDbVotes string         `json:"imdb_votes"`
	Awards    string         `json:"awards"`
	Sources   ratingsSources `json:"sources"`
}

// ratingsValues normalizes the four supported rating sources into stable
// field names. Empty string indicates "no value available" — matching the
// documented JSON contract; the human renderer prints "N/A" for empties.
type ratingsValues struct {
	TMDB           string `json:"tmdb"`
	IMDb           string `json:"imdb"`
	RottenTomatoes string `json:"rotten_tomatoes"`
	Metacritic     string `json:"metacritic"`
}

type ratingsSources struct {
	TMDB bool `json:"tmdb"`
	OMDb bool `json:"omdb"`
}

func newRatingsCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagRegion string
	cmd := &cobra.Command{
		Use:         "ratings <id-or-title>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Show TMDb + IMDb + Rotten Tomatoes + Metacritic ratings for a title",
		Long: `Resolve a title or ID, fetch TMDb's full detail (with append_to_response=external_ids
for the IMDb id), and overlay OMDb's IMDb / Rotten Tomatoes / Metacritic ratings
into a single rating card.

OMDb enrichment is optional: set OMDB_API_KEY in the environment to enable it.
Without OMDB_API_KEY the IMDb / RT / Metacritic rows render "N/A" but TMDb
ratings still work.`,
		Example: `  movie-goat-pp-cli ratings 550
  movie-goat-pp-cli ratings "Fight Club" --json
  movie-goat-pp-cli ratings 1396 --type tv
  movie-goat-pp-cli ratings 550 --json --select title,ratings`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			kind := strings.ToLower(strings.TrimSpace(flagType))
			if kind == "" {
				kind = "movie"
			}
			if kind != "movie" && kind != "tv" {
				return usageErr(fmt.Errorf("--type must be \"movie\" or \"tv\", got %q", flagType))
			}
			query := strings.Join(args, " ")

			out := ratingsOutput{Kind: kind}
			out.Sources.TMDB = true

			switch kind {
			case "movie":
				id, _, err := resolveMovieID(c, query)
				if err != nil {
					return classifyAPIError(err)
				}
				detail, _, err := getMovieDetail(c, id, "external_ids")
				if err != nil {
					return classifyAPIError(err)
				}
				out.TMDBID = detail.ID
				out.Title = detail.Title
				if len(detail.ReleaseDate) >= 4 {
					out.Year = detail.ReleaseDate[:4]
				}
				out.IMDbID = detail.ImdbID
				if detail.ExternalIDs != nil && out.IMDbID == "" {
					out.IMDbID = detail.ExternalIDs.IMDbID
				}
				out.TMDBVotes = detail.VoteCount
				if detail.VoteAverage > 0 {
					out.Ratings.TMDB = fmt.Sprintf("%.1f", detail.VoteAverage)
				}
			case "tv":
				id, _, err := resolveTVID(c, query)
				if err != nil {
					return classifyAPIError(err)
				}
				detail, _, err := getTVDetail(c, id, "external_ids")
				if err != nil {
					return classifyAPIError(err)
				}
				out.TMDBID = detail.ID
				out.Title = detail.Name
				if len(detail.FirstAirDate) >= 4 {
					out.Year = detail.FirstAirDate[:4]
				}
				if detail.ExternalIDs != nil {
					out.IMDbID = detail.ExternalIDs.IMDbID
				}
				out.TMDBVotes = detail.VoteCount
				if detail.VoteAverage > 0 {
					out.Ratings.TMDB = fmt.Sprintf("%.1f", detail.VoteAverage)
				}
			}

			// OMDb enrichment — env-only key, graceful degradation when unset.
			omdbKey := strings.TrimSpace(os.Getenv("OMDB_API_KEY"))
			if omdbKey != "" && out.IMDbID != "" {
				res, oerr := omdb.Fetch(out.IMDbID, omdbKey)
				if oerr != nil && !omdb.IsRateLimit(oerr) {
					// Log but continue: enrichment is best-effort. Rate-limit
					// errors are intentionally surfaced (printed-CLI rule).
					fmt.Fprintf(os.Stderr, "warn: omdb fetch failed: %v\n", oerr)
				}
				if oerr != nil && omdb.IsRateLimit(oerr) {
					return rateLimitErr(oerr)
				}
				if res != nil {
					out.Sources.OMDb = true
					if v := res.ImdbRating; v != "" && v != "N/A" {
						out.Ratings.IMDb = v
					}
					if v := res.RatingBySource("Rotten Tomatoes"); v != "" {
						out.Ratings.RottenTomatoes = v
					}
					if v := res.RatingBySource("Metacritic"); v != "" {
						out.Ratings.Metacritic = v
					} else if res.Metascore != "" && res.Metascore != "N/A" {
						out.Ratings.Metacritic = res.Metascore + "/100"
					}
					if v := res.ImdbVotes; v != "" && v != "N/A" {
						out.IMDbVotes = v
					}
					if v := res.Awards; v != "" && v != "N/A" {
						out.Awards = v
					}
				}
			}

			// JSON / pipe path: route through the standard pipeline.
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Human-readable card.
			w := cmd.OutOrStdout()
			titleLine := out.Title
			if out.Year != "" {
				titleLine += " (" + out.Year + ")"
			}
			fmt.Fprintln(w, titleLine)
			fmt.Fprintln(w, strings.Repeat("=", len(titleLine)))
			rows := [][2]string{
				{"TMDb", naIfEmpty(out.Ratings.TMDB)},
				{"IMDb", naIfEmpty(out.Ratings.IMDb)},
				{"Rotten Tomatoes", naIfEmpty(out.Ratings.RottenTomatoes)},
				{"Metacritic", naIfEmpty(out.Ratings.Metacritic)},
			}
			tw := newTabWriter(w)
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
			}
			tw.Flush()
			if out.IMDbVotes != "" {
				fmt.Fprintf(w, "IMDb votes: %s\n", out.IMDbVotes)
			}
			if out.TMDBVotes > 0 {
				fmt.Fprintf(w, "TMDb votes: %d\n", out.TMDBVotes)
			}
			if out.Awards != "" {
				fmt.Fprintf(w, "Awards: %s\n", out.Awards)
			}
			if !out.Sources.OMDb && omdbKey == "" {
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Tip: set OMDB_API_KEY to enable IMDb / Rotten Tomatoes / Metacritic ratings.")
			}
			_ = flagRegion // reserved for future region-specific renderings
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "movie", "Media type: movie or tv")
	cmd.Flags().StringVar(&flagRegion, "region", "US", "Region code (reserved for future use)")
	return cmd
}

func naIfEmpty(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}
