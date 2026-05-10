package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// gqueryDBCount represents a single database count from EGQuery.
type gqueryDBCount struct {
	DBName string `json:"db_name"`
	Count  int    `json:"count"`
}

func ensureGqueryTrendTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS gquery_snapshots (
			query TEXT NOT NULL,
			db_name TEXT NOT NULL,
			count INTEGER NOT NULL,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (query, db_name, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gquery_query ON gquery_snapshots(query)`,
		`CREATE INDEX IF NOT EXISTS idx_gquery_snapshot_at ON gquery_snapshots(snapshot_at)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating gquery_trend tables: %w", err)
		}
	}
	return nil
}

func newGqueryTrendCmd(flags *rootFlags) *cobra.Command {
	var flagStore bool
	var flagTopMovers bool

	cmd := &cobra.Command{
		Use:   "gquery-trend <query>",
		Short: "Track EGQuery counts across databases over time",
		Long: strings.TrimSpace(`
Multi-DB Count Heatmap / Trend -- calls EGQuery for a query and shows
counts per database. With --store, snapshots are saved to SQLite for
trend analysis. Use --top-movers to see databases with the biggest
changes since the last snapshot.`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli gquery-trend "CRISPR base editing"
  ncbi-entrez-pp-cli gquery-trend "CRISPR base editing" --store
  ncbi-entrez-pp-cli gquery-trend "CRISPR base editing" --top-movers
  ncbi-entrez-pp-cli gquery-trend list`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureGqueryTrendTables(db); err != nil {
				return err
			}

			// Call EGQuery
			params := map[string]string{
				"term":    query,
				"retmode": "json",
			}
			data, err := c.Get("/egquery.fcgi", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			counts := parseEGQueryCounts(data)

			// Store snapshot if requested
			if flagStore && len(counts) > 0 {
				tx, err := db.DB().Begin()
				if err != nil {
					return err
				}
				for _, dc := range counts {
					_, _ = tx.Exec(
						`INSERT INTO gquery_snapshots (query, db_name, count, snapshot_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
						query, dc.DBName, dc.Count,
					)
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("storing snapshot: %w", err)
				}
			}

			// Top movers analysis
			if flagTopMovers {
				return showTopMovers(cmd, db, flags, query, counts)
			}

			// Default: show current counts
			result := map[string]any{
				"query":     query,
				"counts":    counts,
				"stored":    flagStore,
				"total_dbs": len(counts),
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().BoolVar(&flagStore, "store", false, "Store the current counts as a snapshot")
	cmd.Flags().BoolVar(&flagTopMovers, "top-movers", false, "Show databases with biggest count changes")

	cmd.AddCommand(newGqueryTrendListCmd(flags))

	return cmd
}

// parseEGQueryCounts extracts per-database counts from an EGQuery JSON response.
func parseEGQueryCounts(data json.RawMessage) []gqueryDBCount {
	// Try the standard EGQuery format
	var resp struct {
		EGQueryResult struct {
			ResultItem []struct {
				DBName string `json:"DbName"`
				Count  string `json:"Count"`
			} `json:"ResultItem"`
		} `json:"egqueryresult"`
	}

	var counts []gqueryDBCount

	if json.Unmarshal(data, &resp) == nil && len(resp.EGQueryResult.ResultItem) > 0 {
		for _, item := range resp.EGQueryResult.ResultItem {
			var count int
			fmt.Sscanf(item.Count, "%d", &count)
			if count > 0 {
				counts = append(counts, gqueryDBCount{
					DBName: item.DBName,
					Count:  count,
				})
			}
		}
		return counts
	}

	// Fallback: try alternative format
	var alt struct {
		Result []struct {
			DB    string `json:"db"`
			Count int    `json:"count"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &alt) == nil {
		for _, r := range alt.Result {
			if r.Count > 0 {
				counts = append(counts, gqueryDBCount{
					DBName: r.DB,
					Count:  r.Count,
				})
			}
		}
	}

	return counts
}

func showTopMovers(cmd *cobra.Command, db *store.Store, flags *rootFlags, query string, current []gqueryDBCount) error {
	// Get the most recent snapshot timestamp for this query (before the current one)
	var lastSnapshot string
	err := db.DB().QueryRow(
		`SELECT MAX(snapshot_at) FROM gquery_snapshots WHERE query = ?`,
		query,
	).Scan(&lastSnapshot)

	if err != nil || lastSnapshot == "" {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status":  "no_previous_snapshot",
			"message": "No previous snapshot found. Use --store first to save a baseline.",
			"current": current,
		}, flags)
	}

	// Load previous counts
	rows, err := db.DB().Query(
		`SELECT db_name, count FROM gquery_snapshots WHERE query = ? AND snapshot_at = ?`,
		query, lastSnapshot,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	prevCounts := make(map[string]int)
	for rows.Next() {
		var dbName string
		var count int
		if rows.Scan(&dbName, &count) == nil {
			prevCounts[dbName] = count
		}
	}

	// Compute deltas
	type mover struct {
		DBName   string  `json:"db_name"`
		Current  int     `json:"current"`
		Previous int     `json:"previous"`
		Delta    int     `json:"delta"`
		DeltaPct float64 `json:"delta_pct"`
	}

	var movers []mover
	for _, dc := range current {
		prev := prevCounts[dc.DBName]
		delta := dc.Count - prev
		deltaPct := 0.0
		if prev > 0 {
			deltaPct = float64(delta) / float64(prev) * 100
		}
		movers = append(movers, mover{
			DBName:   dc.DBName,
			Current:  dc.Count,
			Previous: prev,
			Delta:    delta,
			DeltaPct: deltaPct,
		})
	}

	// Sort by absolute delta descending
	for i := 0; i < len(movers); i++ {
		for j := i + 1; j < len(movers); j++ {
			if math.Abs(float64(movers[j].Delta)) > math.Abs(float64(movers[i].Delta)) {
				movers[i], movers[j] = movers[j], movers[i]
			}
		}
	}

	// Take top 20
	if len(movers) > 20 {
		movers = movers[:20]
	}

	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"query":         query,
		"since":         lastSnapshot,
		"top_movers":    movers,
		"total_checked": len(current),
	}, flags)
}

func newGqueryTrendListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List all tracked queries",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureGqueryTrendTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(
				`SELECT query, COUNT(DISTINCT snapshot_at) as snapshots, MIN(snapshot_at) as first, MAX(snapshot_at) as latest
				 FROM gquery_snapshots
				 GROUP BY query
				 ORDER BY latest DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var queries []map[string]any
			for rows.Next() {
				var query, first, latest string
				var snapshots int
				if err := rows.Scan(&query, &snapshots, &first, &latest); err != nil {
					return err
				}
				queries = append(queries, map[string]any{
					"query":     query,
					"snapshots": snapshots,
					"first":     first,
					"latest":    latest,
				})
			}
			if queries == nil {
				queries = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), queries, flags)
		},
	}
}
