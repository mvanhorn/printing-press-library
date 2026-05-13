// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel context command — build ready-to-paste LLM context from search results.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"
)

func newContextCmd(flags *rootFlags) *cobra.Command {
	var query string
	var maxResults int
	var session string
	var rawContent bool

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Build a ready-to-paste LLM context string from search results",
		Long: `Search the web and format the results as a numbered passage block
suitable for pasting into an LLM prompt. Each passage includes the source
URL and page content (or snippet if raw content not requested).`,
		Example: `  tavily-pp-cli context --query "Tavily API authentication"
  tavily-pp-cli context --query "LLM evals 2025" --max-results 10 --raw-content`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" && len(args) > 0 {
				query = args[0]
			}
			if query == "" {
				return fmt.Errorf("required: --query or positional argument")
			}
			if maxResults <= 0 {
				maxResults = 5
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"query":       query,
				"max_results": maxResults,
			}
			if rawContent {
				body["include_raw_content"] = "markdown"
			}

			data, _, err := c.Post("/search", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if st, serr := store.Open(); serr == nil {
				bodyJSON, _ := json.Marshal(body)
				st.InsertSearch(query, string(bodyJSON), string(data), session)
				st.InsertCredit("search", 1.0, session)
				st.Close()
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			var resp struct {
				Results []struct {
					Title      string `json:"title"`
					URL        string `json:"url"`
					Content    string `json:"content"`
					RawContent string `json:"raw_content"`
				} `json:"results"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "# Context for: %s\n\n", query)
			for i, r := range resp.Results {
				content := r.Content
				if rawContent && r.RawContent != "" {
					content = r.RawContent
				}
				fmt.Fprintf(&sb, "## [%d] %s\nSource: %s\n\n%s\n\n", i+1, r.Title, r.URL, content)
			}
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Search query to build context for")
	cmd.Flags().IntVar(&maxResults, "max-results", 5, "Number of search results to include")
	cmd.Flags().BoolVar(&rawContent, "raw-content", false, "Include full page markdown in each passage")
	cmd.Flags().StringVar(&session, "session", "", "Session label")
	return cmd
}
