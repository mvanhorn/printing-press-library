package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newMarathonCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "marathon <collection-name>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Plan a franchise marathon with watch order and total runtime",
		Long: `Search for a TMDb collection (e.g. "Star Wars", "The Dark Knight") and display
the full franchise lineup in release order. Shows runtime for each entry,
calculates total watch time, and suggests break points after every 2-3 movies.`,
		Example: `  movie-goat-pp-cli marathon "Star Wars"
  movie-goat-pp-cli marathon "The Dark Knight"
  movie-goat-pp-cli marathon "Lord of the Rings" --json
  movie-goat-pp-cli marathon "Harry Potter" --json --select title,runtime`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "GET /search/collection?query=%s\nGET /collection/<id>\nGET /movie/<id> (per movie in collection)\n", strings.Join(args, " "))
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			query := strings.Join(args, " ")

			// Search for the collection
			searchData, err := c.Get("/search/collection", map[string]string{
				"query": query,
			})
			if err != nil {
				return classifyAPIError(err)
			}

			var searchResp struct {
				Results []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"results"`
			}
			if err := json.Unmarshal(searchData, &searchResp); err != nil {
				return fmt.Errorf("parsing collection search: %w", err)
			}

			if len(searchResp.Results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No collections found for %q. Try a different name.\n", query)
				return nil
			}

			collectionID := searchResp.Results[0].ID
			collectionName := searchResp.Results[0].Name

			// Fetch collection details
			collectionPath := fmt.Sprintf("/collection/%d", collectionID)
			collectionData, err := c.Get(collectionPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}

			var collection struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Overview string `json:"overview"`
				Parts    []struct {
					ID          int     `json:"id"`
					Title       string  `json:"title"`
					ReleaseDate string  `json:"release_date"`
					VoteAverage float64 `json:"vote_average"`
					Overview    string  `json:"overview"`
				} `json:"parts"`
			}
			if err := json.Unmarshal(collectionData, &collection); err != nil {
				return fmt.Errorf("parsing collection details: %w", err)
			}

			if len(collection.Parts) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Collection %q has no movies.\n", collectionName)
				return nil
			}

			// Sort by release date for watch order
			sort.Slice(collection.Parts, func(i, j int) bool {
				return collection.Parts[i].ReleaseDate < collection.Parts[j].ReleaseDate
			})

			// Fetch runtime for each movie
			type marathonEntry struct {
				ID          int     `json:"id"`
				Title       string  `json:"title"`
				Year        string  `json:"year"`
				Runtime     int     `json:"runtime"`
				VoteAverage float64 `json:"vote_average"`
				ReleaseDate string  `json:"release_date"`
				Order       int     `json:"order"`
			}

			var entries []marathonEntry
			totalRuntime := 0

			for i, part := range collection.Parts {
				entry := marathonEntry{
					ID:          part.ID,
					Title:       part.Title,
					VoteAverage: part.VoteAverage,
					ReleaseDate: part.ReleaseDate,
					Order:       i + 1,
				}
				if len(part.ReleaseDate) >= 4 {
					entry.Year = part.ReleaseDate[:4]
				}

				// Fetch movie details for runtime
				detail, _, detailErr := getMovieDetail(c, part.ID, "")
				if detailErr == nil && detail != nil {
					entry.Runtime = detail.Runtime
					totalRuntime += detail.Runtime
				}

				entries = append(entries, entry)
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				output := map[string]any{
					"collection_id":   collectionID,
					"collection_name": collectionName,
					"movie_count":     len(entries),
					"total_runtime":   totalRuntime,
					"total_formatted": fmtRuntime(totalRuntime),
					"movies":          entries,
				}
				data, _ := json.Marshal(output)
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
			fmt.Fprintf(w, "%s Marathon\n", collectionName)
			fmt.Fprintf(w, "%s\n", strings.Repeat("=", len(collectionName)+9))
			fmt.Fprintf(w, "%d movies  |  Total runtime: %s\n\n", len(entries), fmtRuntime(totalRuntime))

			// Watch order table
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "#\tTitle\tYear\tRuntime\tRating")
			for _, e := range entries {
				runtime := "N/A"
				if e.Runtime > 0 {
					runtime = fmtRuntime(e.Runtime)
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%.1f\n",
					e.Order, truncate(e.Title, 40), e.Year, runtime, e.VoteAverage)
			}
			tw.Flush()

			// Suggest break points
			if len(entries) > 2 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Suggested Breaks:")
				breakInterval := 2
				if len(entries) > 6 {
					breakInterval = 3
				}
				cumulativeRuntime := 0
				for i, e := range entries {
					cumulativeRuntime += e.Runtime
					if (i+1)%breakInterval == 0 && i+1 < len(entries) {
						fmt.Fprintf(w, "  Break after #%d %q — %s watched so far\n",
							i+1, truncate(e.Title, 30), fmtRuntime(cumulativeRuntime))
					}
				}
			}

			fmt.Fprintln(w)
			return nil
		},
	}

	return cmd
}
