// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel qna command — search with include_answer=advanced, compact output.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"
)

func newQnACmd(flags *rootFlags) *cobra.Command {
	var query string
	var session string

	cmd := &cobra.Command{
		Use:   "qna",
		Short: "Get a direct answer to a question",
		Long: `Get a direct answer to a question.

Equivalent to 'web --include-answer=advanced' but prints just the answer
and top 3 source URLs — no extra flags needed.`,
		Example: `  tavily-pp-cli qna --query "What is the Tavily API rate limit?"
  tavily-pp-cli qna --query "Latest LLM benchmarks" --session my-agent`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" && len(args) > 0 {
				query = args[0]
			}
			if query == "" {
				return fmt.Errorf("required: --query or positional argument")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"query":          query,
				"include_answer": "advanced",
				"max_results":    5,
			}
			data, _, err := c.Post("/search", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Persist to store for offline features
			if session != "" || true {
				if st, serr := store.Open(); serr == nil {
					bodyJSON, _ := json.Marshal(body)
					st.InsertSearch(query, string(bodyJSON), string(data), session)
					st.InsertCredit("search", 1.0, session)
					st.Close()
				}
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			// Human-friendly: print just the answer + sources
			var resp struct {
				Answer  string `json:"answer"`
				Results []struct {
					Title string `json:"title"`
					URL   string `json:"url"`
				} `json:"results"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			if resp.Answer != "" {
				fmt.Fprintln(cmd.OutOrStdout(), resp.Answer)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "(no direct answer; see sources below)")
			}
			if len(resp.Results) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nSources:")
				max := 3
				if len(resp.Results) < max {
					max = len(resp.Results)
				}
				for i := 0; i < max; i++ {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s\n      %s\n", i+1, resp.Results[i].Title, resp.Results[i].URL)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Question to answer")
	cmd.Flags().StringVar(&session, "session", "", "Session label for replay and cost tracking")
	return cmd
}
