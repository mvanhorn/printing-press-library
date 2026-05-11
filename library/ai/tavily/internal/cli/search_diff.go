package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"

	"github.com/spf13/cobra"
)

func newSearchDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagMaxResults int

	cmd := &cobra.Command{
		Use:   "diff [query]",
		Short: "Re-run a search and compare results against the previous run",
		Long:  "Re-runs a search query, compares results by URL against the last stored search for the same query, and shows new, dropped, and rank-changed URLs",
		Example: strings.Trim(`
  tavily-pp-cli web-search diff "golang best practices"
  tavily-pp-cli web-search diff "golang best practices" --max-results 20
  tavily-pp-cli web-search diff "golang best practices" --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = strings.Join(args, " ")
			}
			if query == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = store.DefaultDBPath()
			}

			// Run search
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"query":       query,
				"max_results": flagMaxResults,
			}
			data, _, err := c.Post("/search", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse response
			var resp map[string]any
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing search response: %w", err)
			}

			type resultEntry struct {
				URL   string `json:"url"`
				Title string `json:"title"`
				Rank  int    `json:"rank"`
			}

			var currentResults []resultEntry
			if resultsRaw, ok := resp["results"]; ok {
				if arr, ok := resultsRaw.([]any); ok {
					for i, item := range arr {
						if obj, ok := item.(map[string]any); ok {
							u, _ := obj["url"].(string)
							t, _ := obj["title"].(string)
							currentResults = append(currentResults, resultEntry{URL: u, Title: t, Rank: i + 1})
						}
					}
				}
			}

			answer, _ := resp["answer"].(string)
			responseTime, _ := resp["response_time"].(float64)

			// Compute params hash for storage
			paramsHash := hashParams(query, flagMaxResults)

			// Open DB
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening db: %w", err)
			}
			defer db.Close()

			// Get previous results
			previous, err := db.GetSearchResultsByQuery(query)
			if err != nil {
				return fmt.Errorf("querying previous searches: %w", err)
			}

			// Store new result
			resultsJSON, _ := json.Marshal(currentResults)
			_, err = db.InsertSearchResult(query, paramsHash, string(resultsJSON), answer, responseTime, 1)
			if err != nil {
				return fmt.Errorf("storing search result: %w", err)
			}

			if len(previous) == 0 {
				result := map[string]any{
					"status":       "first_search",
					"query":        query,
					"result_count": len(currentResults),
					"results":      currentResults,
				}
				if flags.asJSON {
					return flags.printJSON(cmd, result)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "First search for %q: %d results (stored for future comparison)\n", query, len(currentResults))
				for _, r := range currentResults {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n     %s\n", r.Rank, r.Title, r.URL)
				}
				return nil
			}

			// Parse previous results
			var prevResults []resultEntry
			_ = json.Unmarshal([]byte(previous[0].ResultsJSON), &prevResults)

			// Build URL-to-rank maps
			prevRanks := make(map[string]int, len(prevResults))
			for _, r := range prevResults {
				prevRanks[r.URL] = r.Rank
			}
			currRanks := make(map[string]int, len(currentResults))
			for _, r := range currentResults {
				currRanks[r.URL] = r.Rank
			}

			type rankChange struct {
				URL     string `json:"url"`
				OldRank int    `json:"old_rank"`
				NewRank int    `json:"new_rank"`
				Delta   int    `json:"delta"`
			}

			var newURLs, droppedURLs []string
			var changes []rankChange

			for _, r := range currentResults {
				if _, existed := prevRanks[r.URL]; !existed {
					newURLs = append(newURLs, r.URL)
				} else if prevRanks[r.URL] != r.Rank {
					changes = append(changes, rankChange{
						URL:     r.URL,
						OldRank: prevRanks[r.URL],
						NewRank: r.Rank,
						Delta:   prevRanks[r.URL] - r.Rank,
					})
				}
			}
			for _, r := range prevResults {
				if _, exists := currRanks[r.URL]; !exists {
					droppedURLs = append(droppedURLs, r.URL)
				}
			}

			result := map[string]any{
				"query":          query,
				"previous_count": len(prevResults),
				"current_count":  len(currentResults),
				"new_urls":       newURLs,
				"dropped_urls":   droppedURLs,
				"rank_changes":   changes,
			}

			if flags.asJSON {
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Search diff for %q\n", query)
			fmt.Fprintf(cmd.OutOrStdout(), "  Previous: %d results  Current: %d results\n", len(prevResults), len(currentResults))

			if len(newURLs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nNew (%d):\n", len(newURLs))
				for _, u := range newURLs {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", u)
				}
			}
			if len(droppedURLs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDropped (%d):\n", len(droppedURLs))
				for _, u := range droppedURLs {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", u)
				}
			}
			if len(changes) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nRank changes (%d):\n", len(changes))
				for _, ch := range changes {
					arrow := "^"
					if ch.Delta < 0 {
						arrow = "v"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %+d  #%d -> #%d  %s\n", arrow, ch.Delta, ch.OldRank, ch.NewRank, ch.URL)
				}
			}
			if len(newURLs) == 0 && len(droppedURLs) == 0 && len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nNo changes detected.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "", "Path to SQLite database (default ~/.tavily-pp-cli/tavily.db)")
	cmd.Flags().IntVar(&flagMaxResults, "max-results", 10, "Number of search results to return")

	return cmd
}

func hashParams(query string, maxResults int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("q=%s&max=%d", query, maxResults)))
	return hex.EncodeToString(h[:8])
}
