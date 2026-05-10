package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// watchQuery represents a registered query to track over time.
type watchQuery struct {
	Name      string `json:"name"`
	Query     string `json:"query"`
	DB        string `json:"db"`
	Interval  string `json:"interval"`
	CreatedAt string `json:"created_at"`
}

// watchCount represents a single point in the time series of publication counts.
type watchCount struct {
	Name       string `json:"name"`
	Count      int    `json:"count"`
	RecordedAt string `json:"recorded_at"`
}

// watchTrend summarises the velocity of a watched query.
type watchTrend struct {
	Name           string  `json:"name"`
	Query          string  `json:"query"`
	LatestCount    int     `json:"latest_count"`
	PreviousCount  int     `json:"previous_count"`
	Delta          int     `json:"delta"`
	VelocityChange float64 `json:"velocity_change_pct"`
	DataPoints     int     `json:"data_points"`
}

func ensureWatchTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watch_queries (
			name TEXT PRIMARY KEY,
			query TEXT NOT NULL,
			db_name TEXT NOT NULL DEFAULT 'pubmed',
			interval TEXT NOT NULL DEFAULT 'weekly',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watch_counts (
			name TEXT NOT NULL,
			count INTEGER NOT NULL,
			recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (name) REFERENCES watch_queries(name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_watch_counts_name ON watch_counts(name)`,
		`CREATE INDEX IF NOT EXISTS idx_watch_counts_recorded ON watch_counts(recorded_at)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating watch tables: %w", err)
		}
	}
	return nil
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track publication velocity for saved queries",
		Long: `Publication Velocity Monitor -- register PubMed queries and track
how their result counts change over time. Run 'watch run' periodically
to record new data points. No background daemon; counts are stored when run.`,
		Example: `  ncbi-entrez-pp-cli watch add "GLP-1 safety" --name glp1-safety
  ncbi-entrez-pp-cli watch run
  ncbi-entrez-pp-cli watch list --trending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))

	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	var flagDB string
	var flagInterval string

	cmd := &cobra.Command{
		Use:   "add <query>",
		Short: "Register a query to track",
		Example: `  ncbi-entrez-pp-cli watch add "GLP-1 safety" --name glp1-safety
  ncbi-entrez-pp-cli watch add "chimeric antigen receptor" --name car-t --interval monthly`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("search query required as argument")
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			if flagName == "" {
				// Auto-generate name from query
				flagName = strings.ReplaceAll(strings.ToLower(query), " ", "-")
				if len(flagName) > 40 {
					flagName = flagName[:40]
				}
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchTables(db); err != nil {
				return err
			}

			_, err = db.DB().Exec(
				`INSERT INTO watch_queries (name, query, db_name, interval, created_at)
				 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
				 ON CONFLICT(name) DO UPDATE SET query = excluded.query, db_name = excluded.db_name, interval = excluded.interval`,
				flagName, query, flagDB, flagInterval,
			)
			if err != nil {
				return fmt.Errorf("saving watch query: %w", err)
			}

			result := map[string]any{
				"status":   "added",
				"name":     flagName,
				"query":    query,
				"db":       flagDB,
				"interval": flagInterval,
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagName, "name", "", "Friendly name for the watch (auto-generated if omitted)")
	cmd.Flags().StringVar(&flagDB, "db", "pubmed", "Target database")
	cmd.Flags().StringVar(&flagInterval, "interval", "weekly", "Intended check interval (daily, weekly, monthly)")

	return cmd
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Check all watched queries and record counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

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

			if err := ensureWatchTables(db); err != nil {
				return err
			}

			// Load all watched queries
			rows, err := db.DB().Query(`SELECT name, query, db_name FROM watch_queries ORDER BY name`)
			if err != nil {
				return err
			}

			type queryInfo struct {
				name, query, dbName string
			}
			var queries []queryInfo
			for rows.Next() {
				var q queryInfo
				if err := rows.Scan(&q.name, &q.query, &q.dbName); err != nil {
					rows.Close()
					return err
				}
				queries = append(queries, q)
			}
			rows.Close()

			if len(queries) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status":  "no_queries",
					"message": "No watched queries. Use 'watch add' first.",
				}, flags)
			}

			var results []map[string]any
			for _, q := range queries {
				fmt.Fprintf(os.Stderr, "checking %s...\n", q.name)

				params := map[string]string{
					"db":      q.dbName,
					"term":    q.query,
					"retmax":  "0",
					"retmode": "json",
				}

				data, err := c.Get("/esearch.fcgi", params)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: query %s failed: %v\n", q.name, err)
					results = append(results, map[string]any{
						"name":   q.name,
						"status": "error",
						"error":  err.Error(),
					})
					continue
				}

				count := extractCountFromEsearch(data)

				// Store the count
				_, err = db.DB().Exec(
					`INSERT INTO watch_counts (name, count, recorded_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
					q.name, count,
				)
				if err != nil {
					return fmt.Errorf("saving count for %s: %w", q.name, err)
				}

				results = append(results, map[string]any{
					"name":   q.name,
					"count":  count,
					"status": "recorded",
				})
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var flagTrending bool

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List watched queries with data-point counts and velocity trends, optionally filtered to trending-only",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # List all watched queries
  ncbi-entrez-pp-cli watch list --json

  # Show only trending queries
  ncbi-entrez-pp-cli watch list --trending --agent`,
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

			if err := ensureWatchTables(db); err != nil {
				return err
			}

			if flagTrending {
				return listWatchTrending(cmd, db, flags)
			}

			// Simple list of watched queries
			rows, err := db.DB().Query(
				`SELECT q.name, q.query, q.db_name, q.interval, q.created_at,
				        (SELECT COUNT(*) FROM watch_counts c WHERE c.name = q.name) as data_points
				 FROM watch_queries q
				 ORDER BY q.name`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var queries []map[string]any
			for rows.Next() {
				var name, query, dbName, interval, createdAt string
				var dataPoints int
				if err := rows.Scan(&name, &query, &dbName, &interval, &createdAt, &dataPoints); err != nil {
					return err
				}
				queries = append(queries, map[string]any{
					"name":        name,
					"query":       query,
					"db":          dbName,
					"interval":    interval,
					"created_at":  createdAt,
					"data_points": dataPoints,
				})
			}
			if queries == nil {
				queries = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), queries, flags)
		},
	}

	cmd.Flags().BoolVar(&flagTrending, "trending", false, "Sort by velocity change (largest delta first)")

	return cmd
}

func listWatchTrending(cmd *cobra.Command, db *store.Store, flags *rootFlags) error {
	rows, err := db.DB().Query(`SELECT name, query FROM watch_queries ORDER BY name`)
	if err != nil {
		return err
	}

	type qInfo struct{ name, query string }
	var queries []qInfo
	for rows.Next() {
		var q qInfo
		if err := rows.Scan(&q.name, &q.query); err != nil {
			rows.Close()
			return err
		}
		queries = append(queries, q)
	}
	rows.Close()

	var trends []watchTrend
	for _, q := range queries {
		// Get the two most recent counts
		countRows, err := db.DB().Query(
			`SELECT count FROM watch_counts WHERE name = ? ORDER BY recorded_at DESC LIMIT 2`,
			q.name,
		)
		if err != nil {
			continue
		}

		var counts []int
		for countRows.Next() {
			var c int
			if err := countRows.Scan(&c); err != nil {
				continue
			}
			counts = append(counts, c)
		}
		countRows.Close()

		// Get total data points
		var dataPoints int
		db.DB().QueryRow(`SELECT COUNT(*) FROM watch_counts WHERE name = ?`, q.name).Scan(&dataPoints)

		t := watchTrend{
			Name:       q.name,
			Query:      q.query,
			DataPoints: dataPoints,
		}

		if len(counts) >= 1 {
			t.LatestCount = counts[0]
		}
		if len(counts) >= 2 {
			t.PreviousCount = counts[1]
			t.Delta = t.LatestCount - t.PreviousCount
			if t.PreviousCount > 0 {
				t.VelocityChange = float64(t.Delta) / float64(t.PreviousCount) * 100
			}
		}

		trends = append(trends, t)
	}

	// Sort by absolute velocity change descending
	for i := 0; i < len(trends); i++ {
		for j := i + 1; j < len(trends); j++ {
			absI := trends[i].Delta
			if absI < 0 {
				absI = -absI
			}
			absJ := trends[j].Delta
			if absJ < 0 {
				absJ = -absJ
			}
			if absJ > absI {
				trends[i], trends[j] = trends[j], trends[i]
			}
		}
	}

	if trends == nil {
		trends = []watchTrend{}
	}

	return printJSONFiltered(cmd.OutOrStdout(), trends, flags)
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a watched query and its history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("watch name required")
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchTables(db); err != nil {
				return err
			}

			// Delete counts first (FK constraint)
			if _, err := db.DB().Exec(`DELETE FROM watch_counts WHERE name = ?`, name); err != nil {
				return fmt.Errorf("deleting counts: %w", err)
			}
			res, err := db.DB().Exec(`DELETE FROM watch_queries WHERE name = ?`, name)
			if err != nil {
				return fmt.Errorf("deleting query: %w", err)
			}

			affected, _ := res.RowsAffected()
			if affected == 0 {
				return fmt.Errorf("watch %q not found", name)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "removed",
				"name":   name,
			}, flags)
		},
	}
}

// extractCountFromEsearch pulls the total count from an ESearch JSON response.
func extractCountFromEsearch(data json.RawMessage) int {
	var resp struct {
		ESearchResult struct {
			Count string `json:"count"`
		} `json:"esearchresult"`
	}
	if json.Unmarshal(data, &resp) == nil && resp.ESearchResult.Count != "" {
		var count int
		fmt.Sscanf(resp.ESearchResult.Count, "%d", &count)
		return count
	}

	// Try direct count field
	var direct struct {
		Count interface{} `json:"count"`
	}
	if json.Unmarshal(data, &direct) == nil && direct.Count != nil {
		switch v := direct.Count.(type) {
		case float64:
			return int(v)
		case string:
			var count int
			fmt.Sscanf(v, "%d", &count)
			return count
		}
	}

	return 0
}

// Compile guard for sql import.
var _ = sql.ErrNoRows
