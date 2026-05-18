package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/store"
	"github.com/spf13/cobra"
)

func newMcpNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "mcp",
		Short:       "MCP infrastructure analytics: reliability, blast-radius",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMcpReliabilityCmd(flags))
	cmd.AddCommand(newMcpBlastRadiusCmd(flags))
	return cmd
}

func newMcpReliabilityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var top int
	cmd := &cobra.Command{
		Use:   "reliability",
		Short: "Latency percentiles and error rates per MCP server",
		Example: strings.Trim(`
  brainos-pp-cli mcp reliability --json
  brainos-pp-cli mcp reliability --top 5 --agent`, "\n"),
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

			limitClause := ""
			if top > 0 {
				limitClause = fmt.Sprintf(" LIMIT %d", top)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					json_extract(data, '$.server_id') as server_id,
					COUNT(*) as total_calls,
					AVG(CAST(json_extract(data, '$.latency_ms') AS REAL)) as avg_latency_ms,
					SUM(CASE WHEN CAST(json_extract(data, '$.status_code') AS INTEGER) >= 400 THEN 1 ELSE 0 END) as errors,
					MAX(json_extract(data, '$.created_at')) as last_seen
				FROM resources WHERE resource_type = 'mcp-activity-logs'
				GROUP BY json_extract(data, '$.server_id')
				ORDER BY errors DESC, avg_latency_ms DESC`+limitClause)
			if err != nil {
				return fmt.Errorf("querying activity logs: %w", err)
			}
			defer rows.Close()

			type serverRow struct {
				ServerID     string  `json:"server_id"`
				TotalCalls   int64   `json:"total_calls"`
				AvgLatencyMs float64 `json:"avg_latency_ms"`
				Errors       int64   `json:"errors"`
				LastSeen     string  `json:"last_seen"`
				AuthErrors   int64   `json:"auth_errors"`
			}
			serverMap := map[string]*serverRow{}
			var order []string
			for rows.Next() {
				var sid, lastSeen sql.NullString
				var total, errs sql.NullInt64
				var latency sql.NullFloat64
				if err := rows.Scan(&sid, &total, &latency, &errs, &lastSeen); err != nil {
					continue
				}
				s := &serverRow{
					ServerID:     sid.String,
					TotalCalls:   total.Int64,
					AvgLatencyMs: latency.Float64,
					Errors:       errs.Int64,
					LastSeen:     lastSeen.String,
				}
				serverMap[sid.String] = s
				order = append(order, sid.String)
			}

			authRows, _ := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.server_id'),
				       SUM(CAST(json_extract(data, '$.occurrence_count') AS INTEGER))
				FROM resources WHERE resource_type = 'mcp-auth-errors'
				GROUP BY json_extract(data, '$.server_id')
			`)
			if authRows != nil {
				defer authRows.Close()
				for authRows.Next() {
					var sid sql.NullString
					var count sql.NullInt64
					if err := authRows.Scan(&sid, &count); err == nil {
						if s, ok := serverMap[sid.String]; ok {
							s.AuthErrors = count.Int64
						}
					}
				}
			}

			var results []any
			for _, sid := range order {
				results = append(results, serverMap[sid])
			}
			if results == nil {
				results = []any{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&top, "top", 0, "Show only top N servers (0 = all)")
	return cmd
}

func newMcpBlastRadiusCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since string
	cmd := &cobra.Command{
		Use:   "blast-radius",
		Short: "NatureOS tasks that failed downstream of MCP auth errors",
		Example: strings.Trim(`
  brainos-pp-cli mcp blast-radius --since 1h --json
  brainos-pp-cli mcp blast-radius --since 6h --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"auth_errors":[],"failed_tasks":[],"blast_radius":0}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			dur, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since %q: use 1h, 30m, 6h", since)
			}
			windowStart := time.Now().Add(-dur).UTC().Format("2006-01-02T15:04:05Z")

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			authRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.server_id'), json_extract(data, '$.error_type'), json_extract(data, '$.last_occurrence')
				FROM resources WHERE resource_type = 'mcp-auth-errors'
				AND json_extract(data, '$.last_occurrence') >= ?
			`, windowStart)
			if err != nil {
				return fmt.Errorf("querying auth errors: %w", err)
			}
			defer authRows.Close()

			type authErr struct {
				ServerID       string `json:"server_id"`
				ErrorType      string `json:"error_type"`
				LastOccurrence string `json:"last_occurrence"`
			}
			var authErrors []authErr
			for authRows.Next() {
				var sid, et, lo sql.NullString
				if err := authRows.Scan(&sid, &et, &lo); err == nil {
					authErrors = append(authErrors, authErr{sid.String, et.String, lo.String})
				}
			}

			taskRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.run_id'), json_extract(data, '$.agent_type'),
				       json_extract(data, '$.tool'), json_extract(data, '$.status'),
				       json_extract(data, '$.error_message'), json_extract(data, '$.created_at')
				FROM resources WHERE resource_type = 'natureos-task-queue'
				AND json_extract(data, '$.status') IN ('failed','error')
				AND json_extract(data, '$.created_at') >= ?
			`, windowStart)
			if err != nil {
				return fmt.Errorf("querying task queue: %w", err)
			}
			defer taskRows.Close()

			type taskRow struct {
				RunID        string `json:"run_id"`
				AgentType    string `json:"agent_type"`
				Tool         string `json:"tool"`
				Status       string `json:"status"`
				ErrorMessage string `json:"error_message"`
				CreatedAt    string `json:"created_at"`
			}
			var failedTasks []taskRow
			for taskRows.Next() {
				var runID, agType, tool, status, errMsg, createdAt sql.NullString
				if err := taskRows.Scan(&runID, &agType, &tool, &status, &errMsg, &createdAt); err == nil {
					failedTasks = append(failedTasks, taskRow{runID.String, agType.String, tool.String, status.String, errMsg.String, createdAt.String})
				}
			}

			if authErrors == nil {
				authErrors = []authErr{}
			}
			if failedTasks == nil {
				failedTasks = []taskRow{}
			}
			result := map[string]any{
				"window_start": windowStart,
				"auth_errors":  authErrors,
				"failed_tasks": failedTasks,
				"blast_radius": len(failedTasks),
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "1h", "Time window (e.g. 1h, 30m, 6h)")
	return cmd
}
