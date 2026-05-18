package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/store"
	"github.com/spf13/cobra"
)

func newMemoryNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "memory",
		Short:       "Active memory analytics: load by agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMemoryLoadCmd(flags))
	return cmd
}

func newMemoryLoadCmd(flags *rootFlags) *cobra.Command {
	var dbPath, agent string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Active memory load ranked by importance, with expiry alerts",
		Example: strings.Trim(`
  brainos-pp-cli memory load --json
  brainos-pp-cli memory load --agent navigator --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[]`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `
				SELECT
					json_extract(data, '$.agent_id') as agent_id,
					COUNT(*) as memory_count,
					AVG(CAST(json_extract(data, '$.importance') AS REAL)) as avg_importance,
					SUM(CASE WHEN json_extract(data, '$.expires_at') <= datetime('now', '+24 hours') THEN 1 ELSE 0 END) as expiring_soon,
					MIN(json_extract(data, '$.expires_at')) as next_expiry
				FROM resources WHERE resource_type = 'active-memory'`
			var args2 []any
			if agent != "" {
				query += " AND json_extract(data, '$.agent_id') = ?"
				args2 = append(args2, agent)
			}
			query += " GROUP BY json_extract(data, '$.agent_id') ORDER BY avg_importance DESC"

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying active_memory: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var agentID, nextExpiry sql.NullString
				var count, expiring sql.NullInt64
				var avgImp sql.NullFloat64
				if err := rows.Scan(&agentID, &count, &avgImp, &expiring, &nextExpiry); err != nil {
					continue
				}
				results = append(results, map[string]any{
					"agent_id":       agentID.String,
					"memory_count":   count.Int64,
					"avg_importance": avgImp.Float64,
					"expiring_soon":  expiring.Int64,
					"next_expiry":    nextExpiry.String,
				})
			}
			if results == nil {
				results = []map[string]any{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&agent, "agent", "", "Filter by agent ID")
	return cmd
}
