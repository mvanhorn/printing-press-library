// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newTicketsAgeOutCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		status     string
		staleDays  int
		actionNote string
		apply      bool
		limit      int
	)
	cmd := &cobra.Command{
		Use:   "age-out",
		Short: "Find tickets stale in a status for N days, preview then bulk-close with --apply",
		Long: `Local SQLite filter on tickets.status + lastactiondate. Without --apply,
prints what would be closed. With --apply, issues a batch action + status update
through the API per ticket.`,
		Example: strings.Trim(`
  # Preview stale "Awaiting Customer Reply" tickets
  halopsa-pp-cli tickets age-out --status "Awaiting Customer Reply" --stale-days 14

  # Bulk-close with a templated action
  halopsa-pp-cli tickets age-out --status "Awaiting Customer Reply" \
      --stale-days 14 --action-note "Auto-closing per policy" --apply
`, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			where := []string{"COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)"}
			argsSQL := []any{}
			if status != "" {
				where = append(where, "(LOWER(COALESCE(json_extract(data,'$.status_name'),''))=LOWER(?) OR LOWER(COALESCE(json_extract(data,'$.status'),''))=LOWER(?))")
				argsSQL = append(argsSQL, status, status)
			}
			where = append(where, "(julianday('now') - julianday(COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated))) >= ?")
			argsSQL = append(argsSQL, staleDays)
			q := `SELECT id, COALESCE(client_name,'?'), COALESCE(agent_name,'?'),
                COALESCE(json_extract(data,'$.status_name'), json_extract(data,'$.status'), '?'),
                COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated),
                COALESCE(summary,'')
                FROM tickets WHERE ` + strings.Join(where, " AND ") + ` ORDER BY id LIMIT ?`
			argsSQL = append(argsSQL, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), q, argsSQL...)
			if err != nil {
				return fmt.Errorf("age-out query: %w", err)
			}
			defer rows.Close()
			type row struct {
				ID          string `json:"id"`
				Client      string `json:"client"`
				Agent       string `json:"agent"`
				Status      string `json:"status"`
				LastAction  string `json:"last_action"`
				Summary     string `json:"summary"`
				ApplyResult string `json:"apply_result,omitempty"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var lastAction sql.NullString
				if err := rows.Scan(&r.ID, &r.Client, &r.Agent, &r.Status, &lastAction, &r.Summary); err != nil {
					continue
				}
				r.LastAction = lastAction.String
				out = append(out, r)
			}

			if apply {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				for i := range out {
					body := map[string]any{
						"ticket_id":      out[i].ID,
						"outcome":        "Closed",
						"note":           actionNote,
						"hiddenfromuser": false,
					}
					if _, _, err := c.Post("/Actions", []any{body}); err != nil {
						out[i].ApplyResult = "error: " + err.Error()
					} else {
						out[i].ApplyResult = "closed"
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"status":       status,
					"stale_days":   staleDays,
					"applied":      apply,
					"action_note":  actionNote,
					"tickets":      out,
					"count":        len(out),
					"generated_at": time.Now().UTC().Format(time.RFC3339),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Age-out preview (status=%q, stale>=%dd, %d tickets)\n\n", status, staleDays, len(out))
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-25s %-25s %-12s %s\n", "ID", "CLIENT", "AGENT", "STATUS", "SUMMARY")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 100))
			for _, r := range out {
				summary := r.Summary
				if len(summary) > 30 {
					summary = summary[:30] + "..."
				}
				suffix := ""
				if apply {
					suffix = " [" + r.ApplyResult + "]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-25s %-25s %-12s %s%s\n", r.ID, r.Client, r.Agent, r.Status, summary, suffix)
			}
			if !apply {
				fmt.Fprintln(cmd.OutOrStdout(), "\n(preview only — rerun with --apply to close)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&status, "status", "", "Status name to filter by (e.g., \"Awaiting Customer Reply\")")
	cmd.Flags().IntVar(&staleDays, "stale-days", 14, "Minimum days since last action")
	cmd.Flags().StringVar(&actionNote, "action-note", "Auto-closing per policy", "Action body to post when --apply")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually close the tickets (default: preview)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max tickets to scan")
	_ = json.Compact
	return cmd
}

func newTicketsChangedSinceCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		since  string
		mine   bool
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "changed-since [<when>]",
		Short: "Tickets where any action or status change occurred since <when>",
		Long: `Queries the local tickets table by lastupdated/last action date.
Backed by incremental sync; replaces brittle 'tickets updated since' ETLs.`,
		Example: strings.Trim(`
  # Since 9am today
  halopsa-pp-cli tickets changed-since 09:00 --json

  # Last 24h, only mine
  halopsa-pp-cli tickets changed-since 24h --mine
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) > 0 && since == "" {
				since = args[0]
			}
			if since == "" {
				return cmd.Help()
			}
			t, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("parsing <when> %q: %w", since, err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			where := []string{"datetime(COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated)) >= datetime(?)"}
			argsSQL := []any{t.Format(time.RFC3339)}
			if mine {
				where = append(where, "json_extract(data,'$.is_mine') = 1")
			}
			q := `SELECT id, COALESCE(client_name,'?') AS client,
                COALESCE(agent_name,'?') AS agent,
                COALESCE(json_extract(data,'$.status_name'),'?') AS status,
                COALESCE(summary,'') AS summary,
                COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated) AS last_action
                FROM tickets WHERE ` + strings.Join(where, " AND ") + `
                ORDER BY last_action DESC LIMIT ?`
			argsSQL = append(argsSQL, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), q, argsSQL...)
			if err != nil {
				return fmt.Errorf("changed-since query: %w", err)
			}
			defer rows.Close()
			type row struct {
				ID         string `json:"id"`
				Client     string `json:"client"`
				Agent      string `json:"agent"`
				Status     string `json:"status"`
				Summary    string `json:"summary"`
				LastAction string `json:"last_action"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.ID, &r.Client, &r.Agent, &r.Status, &r.Summary, &r.LastAction); err != nil {
					continue
				}
				out = append(out, r)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"since":   t.Format(time.RFC3339),
					"mine":    mine,
					"tickets": out,
					"count":   len(out),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Changed since %s (%d tickets)\n\n", t.Format(time.RFC3339), len(out))
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-22s %-22s %-12s %-20s %s\n", "ID", "CLIENT", "AGENT", "STATUS", "LAST_ACTION", "SUMMARY")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 110))
			for _, r := range out {
				summary := r.Summary
				if len(summary) > 30 {
					summary = summary[:30] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-22s %-22s %-12s %-20s %s\n", r.ID, r.Client, r.Agent, r.Status, truncTime(r.LastAction), summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "", "Window start (e.g. 'yesterday', '24h', '2026-05-20 09:00')")
	cmd.Flags().BoolVar(&mine, "mine", false, "Only tickets where is_mine is set")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max tickets to return")
	return cmd
}

func truncTime(s string) string {
	if len(s) >= 19 {
		return s[:19]
	}
	return s
}
