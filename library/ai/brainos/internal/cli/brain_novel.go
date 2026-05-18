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

func newBrainNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "brain",
		Short:       "Brain analytics: since, lag, anomalies",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newBrainSinceCmd(flags))
	cmd.AddCommand(newBrainLagCmd(flags))
	cmd.AddCommand(newBrainAnomaliesCmd(flags))
	return cmd
}

func newBrainSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "since [duration]",
		Short: "Cross-domain activity: thoughts, messages, state changes in a time window",
		Example: strings.Trim(`
  brainos-pp-cli brain since 2h --json
  brainos-pp-cli brain since 30m --agent --select type,content,ts`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[]`)
				return nil
			}
			durStr := "2h"
			if len(args) > 0 {
				durStr = args[0]
			}
			dur, err := time.ParseDuration(durStr)
			if err != nil {
				return fmt.Errorf("invalid duration %q: use 2h, 30m, 24h", durStr)
			}
			windowStart := time.Now().Add(-dur).UTC().Format("2006-01-02T15:04:05Z")

			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			type event struct {
				Type    string `json:"type"`
				Content string `json:"content"`
				Meta    string `json:"meta,omitempty"`
				Ts      string `json:"ts"`
			}
			var events []event

			// Thoughts
			tRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.content'), json_extract(data, '$.topics'), json_extract(data, '$.created_at')
				FROM resources WHERE resource_type = 'thoughts'
				AND json_extract(data, '$.created_at') >= ?
				ORDER BY json_extract(data, '$.created_at') DESC
			`, windowStart)
			if err == nil {
				defer tRows.Close()
				for tRows.Next() {
					var content, topics, ts sql.NullString
					if err := tRows.Scan(&content, &topics, &ts); err == nil {
						events = append(events, event{"thought", content.String, topics.String, ts.String})
					}
				}
			}

			// Agent messages
			mRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.content'), json_extract(data, '$.sender') || ' → ' || json_extract(data, '$.recipient'), json_extract(data, '$.created_at')
				FROM resources WHERE resource_type = 'agent-messages'
				AND json_extract(data, '$.created_at') >= ?
				ORDER BY json_extract(data, '$.created_at') DESC
			`, windowStart)
			if err == nil {
				defer mRows.Close()
				for mRows.Next() {
					var content, meta, ts sql.NullString
					if err := mRows.Scan(&content, &meta, &ts); err == nil {
						events = append(events, event{"agent_message", content.String, meta.String, ts.String})
					}
				}
			}

			// Shared state changes
			sRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.key'), json_extract(data, '$.namespace'), json_extract(data, '$.updated_at')
				FROM resources WHERE resource_type = 'shared-state'
				AND json_extract(data, '$.updated_at') >= ?
				ORDER BY json_extract(data, '$.updated_at') DESC
			`, windowStart)
			if err == nil {
				defer sRows.Close()
				for sRows.Next() {
					var key, ns, ts sql.NullString
					if err := sRows.Scan(&key, &ns, &ts); err == nil {
						events = append(events, event{"state_change", key.String, ns.String, ts.String})
					}
				}
			}

			if events == nil {
				events = []event{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(events)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newBrainLagCmd(flags *rootFlags) *cobra.Command {
	var dbPath, topic string
	cmd := &cobra.Command{
		Use:   "lag",
		Short: "How long before captured thoughts become NatureOS agent tasks",
		Example: strings.Trim(`
  brainos-pp-cli brain lag --json
  brainos-pp-cli brain lag --topic ops --agent`, "\n"),
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

			thoughtQuery := `
				SELECT json_extract(data, '$.id'), json_extract(data, '$.topics'), json_extract(data, '$.content'), json_extract(data, '$.created_at')
				FROM resources WHERE resource_type = 'thoughts'`
			var tArgs []any
			if topic != "" {
				thoughtQuery += " AND json_extract(data, '$.topics') LIKE ?"
				tArgs = append(tArgs, "%"+topic+"%")
			}
			thoughtQuery += " ORDER BY json_extract(data, '$.created_at') DESC LIMIT 50"

			tRows, err := db.DB().QueryContext(cmd.Context(), thoughtQuery, tArgs...)
			if err != nil {
				return fmt.Errorf("querying thoughts: %w", err)
			}
			defer tRows.Close()

			type thoughtRow struct {
				ID        string
				Topics    string
				Content   string
				CreatedAt string
			}
			var thoughts []thoughtRow
			for tRows.Next() {
				var id, topics, content, ts sql.NullString
				if err := tRows.Scan(&id, &topics, &content, &ts); err == nil {
					thoughts = append(thoughts, thoughtRow{id.String, topics.String, content.String, ts.String})
				}
			}

			// Load tasks for fuzzy matching
			taskRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.project_slug'), json_extract(data, '$.goal'), json_extract(data, '$.created_at')
				FROM resources WHERE resource_type = 'natureos-dual-model-tasks'
				ORDER BY json_extract(data, '$.created_at') DESC LIMIT 200
			`)
			type taskRow struct {
				Slug      string
				Goal      string
				CreatedAt string
			}
			var tasks []taskRow
			if err == nil {
				defer taskRows.Close()
				for taskRows.Next() {
					var slug, goal, ts sql.NullString
					if err := taskRows.Scan(&slug, &goal, &ts); err == nil {
						tasks = append(tasks, taskRow{slug.String, goal.String, ts.String})
					}
				}
			}

			type lagResult struct {
				Topics      string   `json:"topics"`
				Content     string   `json:"content"`
				ThoughtAt   string   `json:"thought_created"`
				TaskAt      *string  `json:"task_created"`
				LagHours    *float64 `json:"lag_hours"`
				MatchedTask string   `json:"matched_task,omitempty"`
			}

			var results []lagResult
			for _, t := range thoughts {
				r := lagResult{Topics: t.Topics, Content: t.Content, ThoughtAt: t.CreatedAt}
				// Fuzzy match: topic words appear in task goal or slug
				topicWords := strings.Fields(strings.ToLower(t.Topics))
				for _, task := range tasks {
					combined := strings.ToLower(task.Goal + " " + task.Slug)
					matched := false
					for _, w := range topicWords {
						if len(w) > 2 && strings.Contains(combined, w) {
							matched = true
							break
						}
					}
					if matched {
						r.TaskAt = &task.CreatedAt
						r.MatchedTask = task.Goal
						tThought, err1 := time.Parse(time.RFC3339, t.CreatedAt)
						tTask, err2 := time.Parse(time.RFC3339, task.CreatedAt)
						if err1 == nil && err2 == nil && tTask.After(tThought) {
							lag := tTask.Sub(tThought).Hours()
							r.LagHours = &lag
						}
						break
					}
				}
				results = append(results, r)
			}
			if results == nil {
				results = []lagResult{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&topic, "topic", "", "Filter by topic keyword")
	return cmd
}

func newBrainAnomaliesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var hours int
	cmd := &cobra.Command{
		Use:   "anomalies",
		Short: "Cross-domain spike detection: MCP errors, memory expiry, task backlog",
		Example: strings.Trim(`
  brainos-pp-cli brain anomalies --json
  brainos-pp-cli brain anomalies --hours 1 --agent`, "\n"),
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

			windowStart := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format("2006-01-02T15:04:05Z")

			type anomaly struct {
				Domain   string `json:"domain"`
				Signal   string `json:"signal"`
				Count    int64  `json:"count"`
				Severity string `json:"severity"`
			}
			var anomalies []anomaly

			// MCP auth errors in window
			var mcpAuthCount int64
			db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources WHERE resource_type = 'mcp-auth-errors'
				AND json_extract(data, '$.last_occurrence') >= ?
			`, windowStart).Scan(&mcpAuthCount)
			if mcpAuthCount > 0 {
				sev := "warning"
				if mcpAuthCount >= 10 {
					sev = "critical"
				}
				anomalies = append(anomalies, anomaly{"mcp", "auth_errors_in_window", mcpAuthCount, sev})
			}

			// Memory expiring in next 1h
			var expiringCount int64
			db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources WHERE resource_type = 'active-memory'
				AND json_extract(data, '$.expires_at') <= datetime('now', '+1 hour')
				AND json_extract(data, '$.expires_at') > datetime('now')
			`).Scan(&expiringCount)
			if expiringCount > 0 {
				sev := "warning"
				if expiringCount >= 5 {
					sev = "critical"
				}
				anomalies = append(anomalies, anomaly{"memory", "expiring_soon", expiringCount, sev})
			}

			// Task queue backlog
			var pendingCount int64
			db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources WHERE resource_type = 'natureos-task-queue'
				AND json_extract(data, '$.status') = 'pending'
			`).Scan(&pendingCount)
			if pendingCount > 20 {
				sev := "warning"
				if pendingCount >= 50 {
					sev = "critical"
				}
				anomalies = append(anomalies, anomaly{"agents", "task_queue_backlog", pendingCount, sev})
			}

			// Unread high-priority agent messages
			var urgentMsgCount int64
			db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources WHERE resource_type = 'agent-messages'
				AND json_extract(data, '$.status') = 'unread'
				AND json_extract(data, '$.priority') IN ('high','urgent','critical')
			`).Scan(&urgentMsgCount)
			if urgentMsgCount > 0 {
				sev := "warning"
				if urgentMsgCount >= 3 {
					sev = "critical"
				}
				anomalies = append(anomalies, anomaly{"agents", "unread_high_priority_messages", urgentMsgCount, sev})
			}

			if anomalies == nil {
				anomalies = []anomaly{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(anomalies)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&hours, "hours", 1, "Time window in hours for spike detection")
	return cmd
}
