// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newSearchBatchCmd(flags *rootFlags) *cobra.Command {
	var file string
	var parallel int
	var maxTotalCost float64
	var searchType string
	var maxResults int

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run multiple searches from a query file and emit JSONL.",
		Long: `Read queries from a file (one per line) and run each as a semantic search.
Emits one JSON object per line (JSONL) for downstream processing.

Tracks cumulative cost and aborts with partial results if --max-total-cost is exceeded.
Use --parallel N to run N searches concurrently.`,
		Example: `  echo -e "CRISPR therapy\narXiv RNA folding" > queries.txt
  valyu-pp-cli semantic-search batch --file queries.txt --type paper
  valyu-pp-cli semantic-search batch -f queries.txt --parallel 3 --max-total-cost 1.00 | jq '.result.results[].url'`,
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("required flag \"file\" not set")
			}

			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("opening query file: %w", err)
			}
			defer f.Close()

			var queries []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					queries = append(queries, line)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading query file: %w", err)
			}
			if len(queries) == 0 {
				return fmt.Errorf("no queries found in %s", file)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, _ := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if db != nil {
				defer db.Close()
			}

			var mu sync.Mutex
			var totalCost float64
			aborted := false

			type batchResult struct {
				index int
				query string
				data  json.RawMessage
				cost  float64
				err   error
			}

			runOne := func(idx int, q string) batchResult {
				body := map[string]any{
					"query":           q,
					"search_type":     searchType,
					"max_num_results": maxResults,
				}
				data, _, apiErr := c.PostQueryWithParams(cmd.Context(), "/v1/search", map[string]string{}, body)
				cost := extractCostFromResponse(data)
				if apiErr != nil {
					return batchResult{index: idx, query: q, err: classifyAPIError(apiErr, flags)}
				}
				return batchResult{index: idx, query: q, data: data, cost: cost}
			}

			emit := func(r batchResult) {
				if r.err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "query %d error: %v\n", r.index, r.err)
					return
				}
				out, _ := json.Marshal(map[string]any{
					"query":  r.query,
					"index":  r.index,
					"cost":   r.cost,
					"result": json.RawMessage(r.data),
				})
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			}

			if parallel <= 1 {
				for i, q := range queries {
					select {
					case <-cmd.Context().Done():
						return cmd.Context().Err()
					default:
					}
					mu.Lock()
					stop := aborted
					mu.Unlock()
					if stop {
						break
					}
					r := runOne(i, q)
					mu.Lock()
					totalCost += r.cost
					if maxTotalCost > 0 && totalCost >= maxTotalCost {
						aborted = true
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: --max-total-cost $%.4f reached after query %d/%d (total $%.4f). Stopping.\n", maxTotalCost, i+1, len(queries), totalCost)
					}
					mu.Unlock()
					emit(r)
				}
			} else {
				sem := make(chan struct{}, parallel)
				results := make([]batchResult, len(queries))
				var wg sync.WaitGroup
				for i, q := range queries {
					wg.Add(1)
					go func(idx int, query string) {
						defer wg.Done()
						sem <- struct{}{}
						defer func() { <-sem }()
						mu.Lock()
						stop := aborted
						mu.Unlock()
						if stop {
							results[idx] = batchResult{index: idx, query: query, err: fmt.Errorf("aborted")}
							return
						}
						r := runOne(idx, query)
						mu.Lock()
						totalCost += r.cost
						if maxTotalCost > 0 && totalCost >= maxTotalCost {
							aborted = true
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: --max-total-cost $%.4f reached (total $%.4f)\n", maxTotalCost, totalCost)
						}
						mu.Unlock()
						results[idx] = r
					}(i, q)
				}
				wg.Wait()
				for _, r := range results {
					emit(r)
				}
			}

			if db != nil {
				recordCost(db, "semantic-search batch", totalCost)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "total cost: $%.4f for %d queries\n", totalCost, len(queries))
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to query file (one query per line)")
	cmd.Flags().IntVar(&parallel, "parallel", 1, "Number of concurrent searches (default 1 = sequential)")
	cmd.Flags().Float64Var(&maxTotalCost, "max-total-cost", 0, "Abort if cumulative cost exceeds this amount (USD)")
	cmd.Flags().StringVar(&searchType, "type", "all", "Search type for all queries (web, paper, finance, bio, patent, sec, economics, news, all)")
	cmd.Flags().IntVar(&maxResults, "max", 5, "Max results per query")
	return cmd
}
