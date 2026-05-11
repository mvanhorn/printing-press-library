package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"

	"github.com/spf13/cobra"
)

func newUsageHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show credit usage history with daily snapshots",
		Long:  "Snapshot current usage from the API, store it locally, then display usage history over time",
		Example: strings.Trim(`
  tavily-pp-cli usage history
  tavily-pp-cli usage history --days 7
  tavily-pp-cli usage history --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = store.DefaultDBPath()
			}

			// First: snapshot current usage from API
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/usage", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse the usage response to store it
			var usage map[string]any
			if err := json.Unmarshal(data, &usage); err == nil {
				db, err := store.Open(dbPath)
				if err != nil {
					return fmt.Errorf("opening db: %w", err)
				}
				defer db.Close()

				plan, _ := usage["plan"].(string)
				totalUsage := jsonInt(usage["total_usage"])
				planLimit := jsonInt(usage["plan_limit"])
				searchUsage := jsonInt(usage["search_usage"])
				extractUsage := jsonInt(usage["extract_usage"])
				crawlUsage := jsonInt(usage["crawl_usage"])
				mapUsage := jsonInt(usage["map_usage"])
				researchUsage := jsonInt(usage["research_usage"])

				_, _ = db.InsertUsageSnapshot(plan, totalUsage, planLimit, searchUsage, extractUsage, crawlUsage, mapUsage, researchUsage)

				// Now show history
				history, err := db.GetUsageHistory(flagDays)
				if err != nil {
					return fmt.Errorf("querying history: %w", err)
				}

				if flags.asJSON {
					return flags.printJSON(cmd, history)
				}

				if len(history) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No usage history found.")
					return nil
				}

				headers := []string{"DATE", "PLAN", "TOTAL", "SEARCH", "EXTRACT", "CRAWL", "MAP", "RESEARCH"}
				rows := make([][]string, 0, len(history))
				for _, h := range history {
					rows = append(rows, []string{
						h.SnapshotAt.Format("2006-01-02 15:04"),
						h.Plan,
						fmt.Sprintf("%d", h.TotalUsage),
						fmt.Sprintf("%d", h.SearchUsage),
						fmt.Sprintf("%d", h.ExtractUsage),
						fmt.Sprintf("%d", h.CrawlUsage),
						fmt.Sprintf("%d", h.MapUsage),
						fmt.Sprintf("%d", h.ResearchUsage),
					})
				}
				return flags.printTable(cmd, headers, rows)
			}

			// Fallback: just print the raw API response
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days of history to show")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to SQLite database (default ~/.tavily-pp-cli/tavily.db)")

	return cmd
}

// jsonInt extracts an integer from a JSON-decoded any value.
func jsonInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
