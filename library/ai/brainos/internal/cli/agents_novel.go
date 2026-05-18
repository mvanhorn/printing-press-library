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

func newAgentsNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "agents",
		Short:       "Agent coordination: deadlock, throughput",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAgentsDeadlockCmd(flags))
	cmd.AddCommand(newAgentsThroughputCmd(flags))
	return cmd
}

func newAgentsDeadlockCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "deadlock",
		Short: "Agents holding shared_state locks while silent (potential deadlock)",
		Example: strings.Trim(`
  brainos-pp-cli agents deadlock --json
  brainos-pp-cli agents deadlock --agent`, "\n"),
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

			// Agents with active locks
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT DISTINCT
					json_extract(data, '$.locked_by') as agent,
					json_extract(data, '$.namespace') as namespace,
					json_extract(data, '$.key') as key,
					json_extract(data, '$.locked_at') as locked_at
				FROM resources WHERE resource_type = 'shared-state'
				AND json_extract(data, '$.locked_by') IS NOT NULL
				AND json_extract(data, '$.locked_by') != ''
			`)
			if err != nil {
				return fmt.Errorf("querying shared_state: %w", err)
			}
			defer rows.Close()

			type lockRow struct {
				Agent     string
				Namespace string
				Key       string
				LockedAt  string
			}
			var locks []lockRow
			for rows.Next() {
				var agent, ns, key, lockedAt sql.NullString
				if err := rows.Scan(&agent, &ns, &key, &lockedAt); err == nil {
					locks = append(locks, lockRow{agent.String, ns.String, key.String, lockedAt.String})
				}
			}

			// Check last message time per locking agent
			silentThreshold := time.Now().Add(-30 * time.Minute).UTC().Format("2006-01-02T15:04:05Z")
			type result struct {
				Agent         string `json:"agent"`
				Namespace     string `json:"namespace"`
				Key           string `json:"key"`
				LockedAt      string `json:"locked_at"`
				LastMessageAt string `json:"last_message_at"`
				IsDeadlock    bool   `json:"is_deadlock"`
			}
			var results []result
			for _, l := range locks {
				var lastMsg sql.NullString
				db.DB().QueryRowContext(cmd.Context(), `
					SELECT MAX(json_extract(data, '$.created_at'))
					FROM resources WHERE resource_type = 'agent-messages'
					AND json_extract(data, '$.sender') = ?
				`, l.Agent).Scan(&lastMsg)

				isDeadlock := !lastMsg.Valid || lastMsg.String < silentThreshold
				results = append(results, result{
					Agent:         l.Agent,
					Namespace:     l.Namespace,
					Key:           l.Key,
					LockedAt:      l.LockedAt,
					LastMessageAt: lastMsg.String,
					IsDeadlock:    isDeadlock,
				})
			}
			if results == nil {
				results = []result{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newAgentsThroughputCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var hours int
	cmd := &cobra.Command{
		Use:   "throughput",
		Short: "NatureOS task completion rates by agent type over a rolling window",
		Example: strings.Trim(`
  brainos-pp-cli agents throughput --json
  brainos-pp-cli agents throughput --hours 24 --agent`, "\n"),
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

			minutes := hours * 60
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					json_extract(data, '$.agent_type') as agent_type,
					json_extract(data, '$.status') as status,
					COUNT(*) as count
				FROM resources WHERE resource_type = 'natureos-task-queue'
				AND datetime(json_extract(data, '$.created_at')) >= datetime('now', '-' || ? || ' minutes')
				GROUP BY json_extract(data, '$.agent_type'), json_extract(data, '$.status')
				ORDER BY agent_type, status
			`, fmt.Sprintf("%d", minutes))
			if err != nil {
				return fmt.Errorf("querying task_queue: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var agentType, status sql.NullString
				var count sql.NullInt64
				if err := rows.Scan(&agentType, &status, &count); err == nil {
					results = append(results, map[string]any{
						"agent_type": agentType.String,
						"status":     status.String,
						"count":      count.Int64,
						"hours":      hours,
					})
				}
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
	cmd.Flags().IntVar(&hours, "hours", 24, "Rolling window in hours")
	return cmd
}
