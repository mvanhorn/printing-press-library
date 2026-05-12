// Hand-authored: novel feature `audit` — chronological actions ledger.
// Created during Phase 3 build; survives regen because it lives in its own file.
package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var workspace string
	var computer string
	var since string
	var kindList string
	var limit int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Chronological table of every CLI-driven action against your computers in a time window.",
		Long: `Read the local actions ledger that orgo-pp-cli writes every time
you run bash, click, screenshot, exec, or key against a computer. The
data has no live-API equivalent — it lives in the SQLite store at
~/.local/share/orgo-pp-cli/data.db.

Filter by workspace, computer, kind list, and a relative since window.
Output is auto-JSON when piped or when --json/--agent is set; otherwise
a chronological table with the oldest action at the top.`,
		Example: strings.Trim(`
  # Last week of actions, all workspaces, all kinds
  orgo-pp-cli audit --since 7d

  # Just bash + exec on one computer, last 24 hours
  orgo-pp-cli audit --computer abc123 --since 24h --kind bash,exec

  # JSON for piping
  orgo-pp-cli audit --since 30d --agent --select ts,computer_id,kind,command
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			filter := store.ActionFilter{
				WorkspaceID: workspace,
				ComputerID:  computer,
				Limit:       limit,
			}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				filter.Since = ts
			}
			if kindList != "" {
				for _, k := range strings.Split(kindList, ",") {
					k = strings.TrimSpace(k)
					if k != "" {
						filter.Kinds = append(filter.Kinds, k)
					}
				}
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			actions, err := db.QueryActions(cmd.Context(), filter)
			if err != nil {
				return fmt.Errorf("querying actions: %w", err)
			}

			// QueryActions returns DESC; reverse to chronological for display.
			reversed := make([]store.Action, len(actions))
			for i, a := range actions {
				reversed[len(actions)-1-i] = a
			}

			if len(reversed) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No actions logged in this window. Run actions through the CLI first (bash/click/screenshot/exec/key) — actions are logged automatically.")
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				return nil
			}

			rows := make([]map[string]any, 0, len(reversed))
			for _, a := range reversed {
				rows = append(rows, map[string]any{
					"ts":               a.Timestamp.UTC().Format(time.RFC3339),
					"computer_id":      truncate(a.ComputerID, 8),
					"computer_id_full": a.ComputerID,
					"workspace_id":     a.WorkspaceID,
					"kind":             a.Kind,
					"summary":          truncate(a.Command, 80),
					"command":          a.Command,
					"args":             a.ArgsJSON,
					"status":           nullIntValue(a.StatusCode),
					"duration_ms":      nullIntValue(a.DurationMS),
				})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			// Compact table with the brief's required columns.
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("TS")+"\t"+bold("COMPUTER_ID")+"\t"+bold("KIND")+"\t"+bold("SUMMARY"))
			for _, r := range rows {
				fmt.Fprintf(tw, "%v\t%v\t%v\t%v\n", r["ts"], r["computer_id"], r["kind"], r["summary"])
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter by workspace ID (matches actions.workspace_id; may be empty)")
	cmd.Flags().StringVar(&computer, "computer", "", "Filter by computer ID")
	cmd.Flags().StringVar(&since, "since", "7d", "Relative time window (e.g. 30m, 24h, 7d, 30d)")
	cmd.Flags().StringVar(&kindList, "kind", "", "Comma-separated kinds to include (bash,click,screenshot,exec,key)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of actions to return")

	return cmd
}

// nullIntValue returns nil for invalid SQL nulls so JSON output
// preserves the absence of a value rather than emitting 0.
func nullIntValue(n sql.NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}
