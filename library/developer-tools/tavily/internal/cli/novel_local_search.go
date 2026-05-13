// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel local-search command — offline FTS over SQLite-cached content.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"
)

func newLocalSearchCmd(flags *rootFlags) *cobra.Command {
	var term string
	var limit int

	cmd := &cobra.Command{
		Use:     "local-search",
		Aliases: []string{"ls"},
		Short:   "Full-text search over locally cached content without API credits",
		Long: `Search all previously fetched search results and extracted content
stored in the local SQLite database. Returns ranked snippets with source
URLs. Zero API credits consumed.`,
		Example: `  tavily-pp-cli local-search --term "vector embeddings"
  tavily-pp-cli local-search --term "rate limit" --limit 5
  tavily-pp-cli local-search "openai api" --json`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if term == "" && len(args) > 0 {
				term = args[0]
			}
			if term == "" {
				return fmt.Errorf("required: --term or positional argument")
			}
			if limit <= 0 {
				limit = 20
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			searches, err := st.FTSSearch(term, limit)
			if err != nil {
				return fmt.Errorf("FTS search: %w", err)
			}
			extracts, err := st.FTSExtract(term, limit)
			if err != nil {
				return fmt.Errorf("FTS extract: %w", err)
			}

			if flags.asJSON {
				out := map[string]any{
					"term":     term,
					"searches": searches,
					"extracts": extracts,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			total := len(searches) + len(extracts)
			if total == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No results for %q in local cache. Run some searches or extracts first.\n", term)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Local results for %q (%d matches):\n\n", term, total)

			if len(searches) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "From searches:")
				for _, r := range searches {
					var resp struct {
						Results []struct {
							Title string `json:"title"`
							URL   string `json:"url"`
						} `json:"results"`
					}
					_ = json.Unmarshal([]byte(r.Response), &resp)
					fmt.Fprintf(cmd.OutOrStdout(), "  Query: %s  (%s)\n", r.Query, r.CreatedAt.Format("2006-01-02 15:04"))
					for _, res := range resp.Results {
						fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n      %s\n", res.Title, res.URL)
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}

			if len(extracts) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "From extracts:")
				for _, r := range extracts {
					var urls []string
					_ = json.Unmarshal([]byte(r.URLs), &urls)
					fmt.Fprintf(cmd.OutOrStdout(), "  Session: %s  (%s)\n", r.Session, r.CreatedAt.Format("2006-01-02 15:04"))
					for _, u := range urls {
						fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", u)
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&term, "term", "", "Search term (FTS5 syntax supported)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")
	return cmd
}
