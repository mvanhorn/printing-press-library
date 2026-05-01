package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagType string

	cmd := &cobra.Command{
		Use:   "watch <title-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find where to stream, rent, or buy a movie or TV show",
		Long: `Look up streaming availability for a movie or TV show.
Accepts a TMDb ID (number) or a title (string) to search for.
Shows providers grouped by: Stream, Rent, Buy.`,
		Example: `  movie-goat-pp-cli watch "The Dark Knight"
  movie-goat-pp-cli watch 550
  movie-goat-pp-cli watch "Breaking Bad" --type tv
  movie-goat-pp-cli watch "Inception" --region GB`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			if flags.dryRun {
				query := strings.Join(args, " ")
				mt := flagType
				if mt == "" {
					mt = "movie"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "GET /search/%s?query=%s\nGET /%s/<id>/watch/providers\n", mt, query, mt)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			query := strings.Join(args, " ")
			mediaType := flagType

			var mediaID int
			var displayTitle string

			// Check if the argument is a numeric ID
			if id, err := strconv.Atoi(query); err == nil {
				mediaID = id
				if mediaType == "" {
					mediaType = "movie"
				}
			} else {
				// Search for it
				if mediaType == "tv" {
					// Search TV shows
					data, searchErr := c.Get("/search/tv", map[string]string{"query": query})
					if searchErr != nil {
						return classifyAPIError(searchErr)
					}
					var resp tmdbSearchResponse
					if err := json.Unmarshal(data, &resp); err != nil {
						return fmt.Errorf("parsing search results: %w", err)
					}
					if len(resp.Results) == 0 {
						return fmt.Errorf("no TV shows found for %q", query)
					}
					mediaID = resp.Results[0].ID
					displayTitle = resp.Results[0].DisplayTitle()
				} else {
					// Default: search movies first, fall back to TV
					id, title, searchErr := searchMovieByTitle(c, query)
					if searchErr != nil {
						// Try TV
						data, tvErr := c.Get("/search/tv", map[string]string{"query": query})
						if tvErr != nil {
							return classifyAPIError(searchErr)
						}
						var resp tmdbSearchResponse
						if err := json.Unmarshal(data, &resp); err != nil {
							return fmt.Errorf("parsing search results: %w", err)
						}
						if len(resp.Results) == 0 {
							return fmt.Errorf("no movies or TV shows found for %q", query)
						}
						mediaID = resp.Results[0].ID
						displayTitle = resp.Results[0].DisplayTitle()
						mediaType = "tv"
					} else {
						mediaID = id
						displayTitle = title
						if mediaType == "" {
							mediaType = "movie"
						}
					}
				}
			}

			// Fetch watch providers
			path := fmt.Sprintf("/%s/%d/watch/providers", mediaType, mediaID)
			data, err := c.Get(path, map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}

			var providers tmdbWatchProviders
			if err := json.Unmarshal(data, &providers); err != nil {
				return fmt.Errorf("parsing watch providers: %w", err)
			}

			region := strings.ToUpper(flagRegion)
			regionData, ok := providers.Results[region]

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				output := map[string]any{
					"media_type": mediaType,
					"media_id":   mediaID,
					"title":      displayTitle,
					"region":     region,
				}
				if ok {
					output["link"] = regionData.Link
					output["stream"] = regionData.Flatrate
					output["free"] = regionData.Free
					output["ads"] = regionData.Ads
					output["rent"] = regionData.Rent
					output["buy"] = regionData.Buy
				} else {
					output["available"] = false
				}
				if flags.compact {
					b, _ := json.Marshal(output)
					return printOutput(cmd.OutOrStdout(), compactFields(json.RawMessage(b)), true)
				}
				if flags.selectFields != "" {
					b, _ := json.Marshal(output)
					return printOutput(cmd.OutOrStdout(), filterFields(json.RawMessage(b), flags.selectFields), true)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			if displayTitle != "" {
				fmt.Fprintf(w, "Watch Providers for: %s (ID: %d)\n", displayTitle, mediaID)
			} else {
				fmt.Fprintf(w, "Watch Providers for ID: %d\n", mediaID)
			}
			fmt.Fprintf(w, "Region: %s\n\n", region)

			if !ok {
				fmt.Fprintf(w, "No watch providers found for region %s.\n", region)
				fmt.Fprintln(w, "Try a different region with --region (e.g. --region GB, --region DE)")
				return nil
			}

			if regionData.Link != "" {
				fmt.Fprintf(w, "TMDb Link: %s\n\n", regionData.Link)
			}

			printProviderGroup(w, "Stream", regionData.Flatrate)
			printProviderGroup(w, "Free", regionData.Free)
			printProviderGroup(w, "Ads", regionData.Ads)
			printProviderGroup(w, "Rent", regionData.Rent)
			printProviderGroup(w, "Buy", regionData.Buy)

			return nil
		},
	}

	cmd.Flags().StringVar(&flagRegion, "region", "US", "Region for watch providers (ISO 3166-1, e.g. US, GB, DE)")
	cmd.Flags().StringVar(&flagType, "type", "", "Media type: movie or tv (auto-detected if omitted)")

	return cmd
}

func printProviderGroup(w interface{ Write([]byte) (int, error) }, label string, providers []tmdbProvider) {
	if len(providers) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", label)
	for _, p := range providers {
		fmt.Fprintf(w, "  - %s\n", p.ProviderName)
	}
	fmt.Fprintln(w)
}
