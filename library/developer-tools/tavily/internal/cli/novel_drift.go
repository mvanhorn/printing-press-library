// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel drift-detect command — diff current search results vs stored baseline.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/tavily/internal/store"
)

func newDriftDetectCmd(flags *rootFlags) *cobra.Command {
	var query string
	var session string

	cmd := &cobra.Command{
		Use:   "drift-detect",
		Short: "Compare current search results against a stored baseline",
		Long: `Run a fresh search for a query and compare the results against the
most recently cached result set for the same query. Reports which URLs
appeared, disappeared, or changed relevance score.

The first time you run drift-detect for a query, the current results become
the baseline. Subsequent runs compare against that baseline.`,
		Example: `  tavily-pp-cli drift-detect --query "Tavily Python SDK"
  tavily-pp-cli drift-detect --query "LLM benchmarks 2025" --json`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" && len(args) > 0 {
				query = args[0]
			}
			if query == "" {
				return fmt.Errorf("required: --query or positional argument")
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			// Load baseline
			baseline, err := st.BaselineSearch(query)
			if err != nil {
				return fmt.Errorf("reading baseline: %w", err)
			}

			// Run fresh search
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"query":       query,
				"max_results": 10,
			}
			data, _, serr := c.Post("/search", body)
			if serr != nil {
				return classifyAPIError(serr, flags)
			}

			bodyJSON, _ := json.Marshal(body)
			st.InsertSearch(query, string(bodyJSON), string(data), session)
			st.InsertCredit("search", 1.0, session)

			if baseline == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "No baseline found for %q — this run is now the baseline.\n", query)
				return nil
			}

			type Result struct {
				URL   string  `json:"url"`
				Title string  `json:"title"`
				Score float64 `json:"score"`
			}
			parseResults := func(raw string) map[string]Result {
				var resp struct {
					Results []Result `json:"results"`
				}
				json.Unmarshal([]byte(raw), &resp)
				m := make(map[string]Result)
				for _, r := range resp.Results {
					m[r.URL] = r
				}
				return m
			}

			baselineURLs := parseResults(baseline.Response)
			currentURLs := parseResults(string(data))

			type DriftEntry struct {
				URL    string  `json:"url"`
				Change string  `json:"change"`
				Delta  float64 `json:"score_delta,omitempty"`
			}
			var drift []DriftEntry

			for url, cur := range currentURLs {
				if prev, ok := baselineURLs[url]; !ok {
					drift = append(drift, DriftEntry{URL: url, Change: "appeared"})
				} else {
					delta := cur.Score - prev.Score
					if delta > 0.05 || delta < -0.05 {
						drift = append(drift, DriftEntry{URL: url, Change: "score-changed", Delta: delta})
					}
				}
			}
			for url := range baselineURLs {
				if _, ok := currentURLs[url]; !ok {
					drift = append(drift, DriftEntry{URL: url, Change: "disappeared"})
				}
			}

			if flags.asJSON {
				out := map[string]any{
					"query":        query,
					"baseline_at":  baseline.CreatedAt,
					"drift_count":  len(drift),
					"drift":        drift,
				}
				outData, _ := json.MarshalIndent(out, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), outData, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Drift for %q (baseline: %s)\n\n",
				query, baseline.CreatedAt.Format("2006-01-02 15:04"))
			if len(drift) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift detected — results are stable.")
				return nil
			}
			for _, d := range drift {
				switch d.Change {
				case "appeared":
					fmt.Fprintf(cmd.OutOrStdout(), "  + appeared:    %s\n", d.URL)
				case "disappeared":
					fmt.Fprintf(cmd.OutOrStdout(), "  - disappeared: %s\n", d.URL)
				case "score-changed":
					sign := "+"
					if d.Delta < 0 {
						sign = ""
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  ~ score %s%.3f: %s\n", sign, d.Delta, d.URL)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Query to drift-check")
	cmd.Flags().StringVar(&session, "session", "", "Session label")
	return cmd
}
