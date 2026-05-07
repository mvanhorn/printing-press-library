// Hand-authored novel feature: substring grep over locally-synced runs.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/internal/store"
)

func newRunsFindCmd(flags *rootFlags) *cobra.Command {
	var statusFilter string
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "find <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Substring grep over locally-synced runs (errors, tags, metadata, task identifiers)",
		Long: `Searches the local SQLite copy of runs for substrings in error messages,
tags, metadata payloads, and task identifiers. Uses a JSON-aware LIKE
predicate over the runs.data column, with status/since filters applied as
SQL WHERE clauses. Composable with --status, --since, --json, and --select.

Run 'trigger-dev-pp-cli sync --resources runs --since 30d' first to populate
the local store; without local data, this command returns "no results — sync first".`,
		Example: `  trigger-dev-pp-cli runs find "timeout" --status FAILED --since 7d
  trigger-dev-pp-cli runs find "rate limit" --json --select 'id,taskIdentifier,error.message'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would substring-grep for %q (status=%s since=%s)\n", args[0], statusFilter, since)
				return nil
			}
			query := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("trigger-dev-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening local store at %s: %w (run 'sync' first)", dbPath, err))
			}
			defer db.Close()

			esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
			likeQ := "%" + esc + "%"
			sqlText := `SELECT id, data FROM runs WHERE data LIKE ? ESCAPE '\'`
			sqlArgs := []any{likeQ}

			if statusFilter != "" {
				statusList := strings.Split(statusFilter, ",")
				placeholders := make([]string, len(statusList))
				for i, s := range statusList {
					placeholders[i] = "?"
					sqlArgs = append(sqlArgs, strings.TrimSpace(s))
				}
				sqlText += " AND json_extract(data, '$.status') IN (" + strings.Join(placeholders, ",") + ")"
			}
			if since != "" {
				cutoff, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since must be like 1d, 7d, 30d: %w", err))
				}
				sqlText += " AND datetime(json_extract(data, '$.createdAt')) >= datetime(?)"
				sqlArgs = append(sqlArgs, cutoff.Format("2006-01-02T15:04:05Z"))
			}
			sqlText += " ORDER BY datetime(json_extract(data, '$.createdAt')) DESC"
			if limit > 0 {
				sqlText += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), sqlText, sqlArgs...)
			if err != nil {
				return apiErr(fmt.Errorf("querying runs: %w", err))
			}
			defer rows.Close()

			var matches []json.RawMessage
			for rows.Next() {
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				matches = append(matches, json.RawMessage(data))
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No runs matched %q in local store. Try: sync --resources runs --since 30d\n", query)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d match(es):\n\n", len(matches))
			for _, raw := range matches {
				var run map[string]any
				if err := json.Unmarshal(raw, &run); err != nil {
					continue
				}
				printRunRow(cmd, run)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "Comma-separated status filter (FAILED,COMPLETED,...)")
	cmd.Flags().StringVar(&since, "since", "", "Only runs newer than this window (1d, 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max matches (0 for all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local store path")
	return cmd
}

func printRunRow(cmd *cobra.Command, run map[string]any) {
	id, _ := run["id"].(string)
	task, _ := run["taskIdentifier"].(string)
	status, _ := run["status"].(string)
	created, _ := run["createdAt"].(string)
	fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-30s  %-10s  %s\n",
		truncate(id, 28), truncate(task, 30), status, truncate(created, 25))
}
