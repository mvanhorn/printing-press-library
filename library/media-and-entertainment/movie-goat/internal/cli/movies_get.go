// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb"
)

func newMoviesGetCmd(flags *rootFlags) *cobra.Command {
	var flagAppendToResponse string

	cmd := &cobra.Command{
		Use:   "get <movieId>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get detailed info about a movie including cast, ratings, and streaming availability",
		Example: `  movie-goat-pp-cli movies get 550
  movie-goat-pp-cli movies get 155 --json
  movie-goat-pp-cli movies get 680 --append-to-response credits,videos`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/movie/{movieId}"
			path = replacePathParam(path, "movieId", args[0])
			params := map[string]string{}
			if flagAppendToResponse != "" {
				params["append_to_response"] = fmt.Sprintf("%v", flagAppendToResponse)
			}
			data, prov, err := resolveRead(c, flags, "movies", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// OMDb enrichment: if response has imdb_id and OMDB_API_KEY is set
			cfg, _ := config.Load(flags.configPath)
			omdbAPIKey := ""
			if cfg != nil {
				omdbAPIKey = cfg.OmdbApiKey
			}

			var movieData map[string]json.RawMessage
			_ = json.Unmarshal(data, &movieData)

			var imdbID string
			if raw, ok := movieData["imdb_id"]; ok {
				_ = json.Unmarshal(raw, &imdbID)
			}
			// Also check external_ids for imdb_id
			if imdbID == "" {
				if raw, ok := movieData["external_ids"]; ok {
					var extIDs map[string]json.RawMessage
					if json.Unmarshal(raw, &extIDs) == nil {
						if imdbRaw, ok := extIDs["imdb_id"]; ok {
							_ = json.Unmarshal(imdbRaw, &imdbID)
						}
					}
				}
			}

			var omdbResult *omdb.Result
			if imdbID != "" && omdbAPIKey != "" {
				omdbResult, _ = omdb.Fetch(imdbID, omdbAPIKey)
			}

			// Print provenance to stderr for human-facing output
			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}

			// For JSON output, include omdb data in envelope
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := data
				if flags.compact {
					filtered = compactFields(filtered)
				}
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				// Inject omdb data into JSON output
				if omdbResult != nil {
					filtered = injectOMDb(filtered, omdbResult)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			// Human table output — show OMDb ratings after main output
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			err = printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			if err != nil {
				return err
			}

			// Append OMDb ratings in table mode
			if omdbResult != nil {
				printOMDbRatings(cmd, omdbResult, movieData)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flagAppendToResponse, "append-to-response", "credits,watch/providers,videos,recommendations,external_ids", "Additional data to include (credits,watch/providers,videos,recommendations,similar,external_ids)")

	return cmd
}

// printOMDbRatings prints OMDb ratings in a human-readable format.
func printOMDbRatings(cmd *cobra.Command, result *omdb.Result, movieData map[string]json.RawMessage) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Ratings:")

	// TMDb rating from the original data
	var voteAverage float64
	if raw, ok := movieData["vote_average"]; ok {
		_ = json.Unmarshal(raw, &voteAverage)
	}
	if voteAverage > 0 {
		fmt.Fprintf(w, "  TMDb:             %.1f/10\n", voteAverage)
	}

	if result.ImdbRating != "" && result.ImdbRating != "N/A" {
		fmt.Fprintf(w, "  IMDb:             %s/10\n", result.ImdbRating)
	}

	rt := result.RatingBySource("Rotten Tomatoes")
	if rt != "" {
		fmt.Fprintf(w, "  Rotten Tomatoes:  %s\n", rt)
	}

	mc := result.RatingBySource("Metacritic")
	if mc != "" {
		fmt.Fprintf(w, "  Metacritic:       %s\n", mc)
	}

	if result.Awards != "" && result.Awards != "N/A" {
		fmt.Fprintf(w, "Awards: %s\n", result.Awards)
	}

	if result.BoxOffice != "" && result.BoxOffice != "N/A" {
		fmt.Fprintf(w, "Box Office: %s\n", result.BoxOffice)
	}
}

// injectOMDb adds an "omdb" field to JSON output.
func injectOMDb(data json.RawMessage, result *omdb.Result) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}
	omdbJSON, err := json.Marshal(result)
	if err != nil {
		return data
	}
	obj["omdb"] = json.RawMessage(omdbJSON)
	out, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return json.RawMessage(out)
}
