package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/europe-pmc/internal/store"
	"github.com/spf13/cobra"
)

func ensurePPRIntelTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ppr_topics (
			topic TEXT PRIMARY KEY,
			query TEXT,
			last_checked DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS ppr_snapshots (
			topic TEXT NOT NULL,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			count INTEGER DEFAULT 0,
			sample_ids_json TEXT,
			PRIMARY KEY (topic, snapshot_at)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newPPRIntelCmd(flags *rootFlags) *cobra.Command {
	var flagTopic string
	var flagSince string
	var flagPageSize int

	cmd := &cobra.Command{
		Use:   "ppr-intel",
		Short: "Monitor preprint servers by topic with trend tracking",
		Long: `Monitor Europe PMC preprint sources (SRC:PPR) for new preprints
by topic. Takes snapshots of counts over time for trend analysis.`,
		Example: `  europe-pmc-pp-cli ppr-intel --topic "machine learning" --since 2025-01
  europe-pmc-pp-cli ppr-intel --topic "CRISPR"
  europe-pmc-pp-cli ppr-intel list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagTopic == "" {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePPRIntelTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			// Build query for preprint source
			query := fmt.Sprintf("SRC:PPR AND %s", flagTopic)
			if flagSince != "" {
				query = fmt.Sprintf("SRC:PPR AND %s AND (FIRST_PDATE:[%s TO *])", flagTopic, flagSince)
			}

			params := map[string]string{
				"query":      query,
				"format":     "json",
				"resultType": "core",
				"pageSize":   fmt.Sprintf("%d", flagPageSize),
			}
			data, err := c.Get("/search", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope struct {
				HitCount   int `json:"hitCount"`
				ResultList struct {
					Result []struct {
						ID string `json:"id"`
					} `json:"result"`
				} `json:"resultList"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			// Collect sample IDs
			sampleIDs := make([]string, 0)
			for i, r := range envelope.ResultList.Result {
				if i >= 10 {
					break
				}
				sampleIDs = append(sampleIDs, r.ID)
			}
			sampleJSON, _ := json.Marshal(sampleIDs)

			now := time.Now()

			// Upsert topic
			_, err = db.DB().Exec(
				`INSERT INTO ppr_topics (topic, query, last_checked) VALUES (?, ?, ?)
				 ON CONFLICT(topic) DO UPDATE SET query = excluded.query, last_checked = excluded.last_checked`,
				flagTopic, query, now,
			)
			if err != nil {
				return fmt.Errorf("storing topic: %w", err)
			}

			// Store snapshot
			_, err = db.DB().Exec(
				`INSERT INTO ppr_snapshots (topic, snapshot_at, count, sample_ids_json) VALUES (?, ?, ?, ?)`,
				flagTopic, now, envelope.HitCount, string(sampleJSON),
			)
			if err != nil {
				return fmt.Errorf("storing snapshot: %w", err)
			}

			result := map[string]any{
				"topic":      flagTopic,
				"query":      query,
				"hit_count":  envelope.HitCount,
				"sample_ids": sampleIDs,
				"snapshot":   now.Format(time.RFC3339),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagTopic, "topic", "", "Topic to monitor (e.g. 'machine learning')")
	cmd.Flags().StringVar(&flagSince, "since", "", "Date filter YYYY-MM (e.g. 2025-01)")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 25, "Results per page")

	cmd.AddCommand(newPPRIntelListCmd(flags))
	return cmd
}

func newPPRIntelListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracked preprint topics with snapshot history and latest hit counts",
		Example: `  europe-pmc-pp-cli ppr-intel list
  europe-pmc-pp-cli ppr-intel list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open(defaultDBPath("europe-pmc-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePPRIntelTables(db); err != nil {
				return fmt.Errorf("creating tables: %w", err)
			}

			rows, err := db.DB().Query(
				`SELECT t.topic, t.query, t.last_checked,
				        (SELECT COUNT(*) FROM ppr_snapshots s WHERE s.topic = t.topic) as snapshot_count,
				        (SELECT s.count FROM ppr_snapshots s WHERE s.topic = t.topic ORDER BY s.snapshot_at DESC LIMIT 1) as latest_count
				 FROM ppr_topics t ORDER BY t.last_checked DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying topics: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var topic, query string
				var lastChecked sql.NullTime
				var snapshotCount int
				var latestCount sql.NullInt64
				if err := rows.Scan(&topic, &query, &lastChecked, &snapshotCount, &latestCount); err != nil {
					continue
				}
				row := map[string]any{
					"topic":          topic,
					"query":          query,
					"snapshot_count": snapshotCount,
				}
				if lastChecked.Valid {
					row["last_checked"] = lastChecked.Time.Format(time.RFC3339)
				}
				if latestCount.Valid {
					row["latest_hit_count"] = latestCount.Int64
				}
				results = append(results, row)
			}
			if len(results) == 0 {
				results = []map[string]any{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}
