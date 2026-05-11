package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"

	"github.com/spf13/cobra"
)

func newMapDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagURL string

	cmd := &cobra.Command{
		Use:   "diff [url]",
		Short: "Compare current sitemap against the last stored map for the same URL",
		Long:  "Calls the /map API for the current sitemap, compares against the last stored result for the same base URL, and shows added/removed URLs",
		Example: strings.Trim(`
  tavily-pp-cli map diff https://example.com
  tavily-pp-cli map diff --url https://example.com
  tavily-pp-cli map diff https://example.com --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := flagURL
			if len(args) > 0 {
				url = args[0]
			}
			if url == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = store.DefaultDBPath()
			}

			// Call the /map API
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{"url": url}
			data, _, err := c.Post("/map", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse response to extract URLs
			var resp map[string]any
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing map response: %w", err)
			}

			var currentURLs []string
			if urlsRaw, ok := resp["urls"]; ok {
				if arr, ok := urlsRaw.([]any); ok {
					for _, u := range arr {
						if s, ok := u.(string); ok {
							currentURLs = append(currentURLs, s)
						}
					}
				}
			}

			// Open DB and get previous result
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening db: %w", err)
			}
			defer db.Close()

			previous, err := db.GetMapResultsByBaseURL(url)
			if err != nil {
				return fmt.Errorf("querying previous maps: %w", err)
			}

			// Store new result
			_, err = db.InsertMapResult(url, currentURLs, len(currentURLs))
			if err != nil {
				return fmt.Errorf("storing map result: %w", err)
			}

			if len(previous) == 0 {
				// No previous result — just show current
				result := map[string]any{
					"status":    "first_scan",
					"base_url":  url,
					"url_count": len(currentURLs),
					"urls":      currentURLs,
				}
				if flags.asJSON {
					return flags.printJSON(cmd, result)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "First scan for %s: %d URLs found (stored for future comparison)\n", url, len(currentURLs))
				for _, u := range currentURLs {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", u)
				}
				return nil
			}

			// Compare against last stored result
			prevURLSet := make(map[string]bool, len(previous[0].URLs))
			for _, u := range previous[0].URLs {
				prevURLSet[u] = true
			}
			currURLSet := make(map[string]bool, len(currentURLs))
			for _, u := range currentURLs {
				currURLSet[u] = true
			}

			var added, removed []string
			for _, u := range currentURLs {
				if !prevURLSet[u] {
					added = append(added, u)
				}
			}
			for _, u := range previous[0].URLs {
				if !currURLSet[u] {
					removed = append(removed, u)
				}
			}

			result := map[string]any{
				"base_url":       url,
				"previous_count": len(previous[0].URLs),
				"current_count":  len(currentURLs),
				"added_count":    len(added),
				"removed_count":  len(removed),
				"added":          added,
				"removed":        removed,
			}

			if flags.asJSON {
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Map diff for %s\n", url)
			fmt.Fprintf(cmd.OutOrStdout(), "  Previous: %d URLs  Current: %d URLs\n", len(previous[0].URLs), len(currentURLs))
			if len(added) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nAdded (%d):\n", len(added))
				for _, u := range added {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", u)
				}
			}
			if len(removed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nRemoved (%d):\n", len(removed))
				for _, u := range removed {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", u)
				}
			}
			if len(added) == 0 && len(removed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nNo changes detected.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "", "Path to SQLite database (default ~/.tavily-pp-cli/tavily.db)")
	cmd.Flags().StringVar(&flagURL, "url", "", "URL to map (alternative to positional arg)")

	return cmd
}
