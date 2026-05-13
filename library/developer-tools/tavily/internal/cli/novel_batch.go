// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel batch-plan command — pre-flight query list against cache.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"
)

func newBatchPlanCmd(flags *rootFlags) *cobra.Command {
	var queryFile string
	var budgetLimit float64
	var session string
	var execute bool

	cmd := &cobra.Command{
		Use:   "batch-plan",
		Short: "Pre-flight a query list against cache, estimate cost, and run net-new within budget",
		Long: `Read a list of queries (one per line) from a file or stdin, check each
against the local cache, estimate credit cost for uncached queries, and
optionally execute only the net-new queries that fit within the budget.`,
		Example: `  tavily-pp-cli batch-plan --query-file queries.txt
  tavily-pp-cli batch-plan --query-file queries.txt --budget 50 --execute
  echo "quantum computing\nmachine learning" | tavily-pp-cli batch-plan --execute`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if budgetLimit <= 0 {
				budgetLimit = 1e9 // effectively unlimited if not set
			}

			// Read queries
			var queries []string
			if queryFile == "" || queryFile == "-" {
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					q := strings.TrimSpace(scanner.Text())
					if q != "" && !strings.HasPrefix(q, "#") {
						queries = append(queries, q)
					}
				}
			} else {
				f, err := os.Open(queryFile)
				if err != nil {
					return fmt.Errorf("opening query file: %w", err)
				}
				defer f.Close()
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					q := strings.TrimSpace(scanner.Text())
					if q != "" && !strings.HasPrefix(q, "#") {
						queries = append(queries, q)
					}
				}
			}
			if len(queries) == 0 {
				return fmt.Errorf("no queries found; provide a --query-file or pipe queries via stdin")
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			var plan []batchPlanEntry
			var totalCost float64

			for _, q := range queries {
				rows, _ := st.SearchesByQuery(q, 1)
				cached := len(rows) > 0
				cost := 0.0
				if !cached {
					cost = 1.0 // 1 credit per basic search
				}
				plan = append(plan, batchPlanEntry{Query: q, Cached: cached, Cost: cost})
				if !cached {
					totalCost += cost
				}
			}

			if flags.asJSON {
				out := map[string]any{
					"queries":      len(queries),
					"cached":       countCached(plan),
					"net_new":      len(queries) - countCached(plan),
					"estimated_cost": totalCost,
					"budget":       budgetLimit,
					"within_budget": totalCost <= budgetLimit,
					"plan":         plan,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Batch plan: %d queries, %d cached, %d net-new, ~%.0f credits\n\n",
				len(queries), countCached(plan), len(queries)-countCached(plan), totalCost)

			for _, p := range plan {
				status := "CACHE"
				if !p.Cached {
					status = fmt.Sprintf("LIVE  ~%.0f cr", p.Cost)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%-11s] %s\n", status, p.Query)
			}

			if totalCost > budgetLimit {
				fmt.Fprintf(cmd.OutOrStdout(), "\nEstimated cost (%.0f cr) exceeds budget (%.0f cr). Use --budget to increase or remove queries.\n",
					totalCost, budgetLimit)
				return nil
			}

			if !execute {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDry-run complete. Add --execute to run net-new queries.\n")
				return nil
			}

			// Execute net-new queries
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			for _, p := range plan {
				if p.Cached {
					continue
				}
				body := map[string]any{"query": p.Query, "max_results": 5}
				data, _, serr := c.Post("/search", body)
				if serr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: search %q failed: %v\n", p.Query, serr)
					continue
				}
				bodyJSON, _ := json.Marshal(body)
				st.InsertSearch(p.Query, string(bodyJSON), string(data), session)
				st.InsertCredit("search", 1.0, session)
				fmt.Fprintf(cmd.OutOrStdout(), "  Fetched: %s\n", p.Query)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&queryFile, "query-file", "-", "Query file path (- for stdin)")
	cmd.Flags().Float64Var(&budgetLimit, "budget", 0, "Credit budget limit (0=unlimited)")
	cmd.Flags().StringVar(&session, "session", "", "Session label for executed calls")
	cmd.Flags().BoolVar(&execute, "execute", false, "Execute net-new queries (default: dry-run only)")
	return cmd
}

type batchPlanEntry struct {
	Query  string  `json:"query"`
	Cached bool    `json:"cached"`
	Cost   float64 `json:"estimated_cost"`
}

func countCached(plan []batchPlanEntry) int {
	n := 0
	for _, p := range plan {
		if p.Cached {
			n++
		}
	}
	return n
}
