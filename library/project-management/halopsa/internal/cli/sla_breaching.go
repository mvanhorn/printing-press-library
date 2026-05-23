// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newSLABreachingCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		team   string
		within string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "breaching",
		Short: "Tickets whose targetdate falls in the next N hours, sorted by time-to-breach",
		Long: `Reads tickets.targetdate locally; joins agent + client + current status.
The Friday-afternoon-before-handoff command.`,
		Example: strings.Trim(`
  # 24-hour breach window across all teams
  halopsa-pp-cli sla breaching --within 24h --json

  # Tighter window scoped to the on-call team
  halopsa-pp-cli sla breaching --within 4h --team OnCall
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			hours, err := parseWithinHours(within)
			if err != nil {
				return fmt.Errorf("--within %q: %w", within, err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			where := []string{
				"COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)",
				"datetime(COALESCE(json_extract(data,'$.targetdate'),'')) BETWEEN datetime('now') AND datetime('now', '+' || ? || ' hours')",
			}
			argsSQL := []any{hours}
			if team != "" {
				where = append(where, "(team = ? OR json_extract(data,'$.team') = ?)")
				argsSQL = append(argsSQL, team, team)
			}
			q := `SELECT id,
                COALESCE(client_name,'?') AS client,
                COALESCE(agent_name,'?') AS agent,
                COALESCE(json_extract(data,'$.status_name'),'?') AS status,
                COALESCE(summary,'') AS summary,
                COALESCE(json_extract(data,'$.targetdate'),'') AS targetdate,
                CAST((julianday(COALESCE(json_extract(data,'$.targetdate'),'')) - julianday('now')) * 24 * 60 AS INTEGER) AS minutes_to_breach
                FROM tickets WHERE ` + strings.Join(where, " AND ") + `
                ORDER BY minutes_to_breach ASC LIMIT ?`
			argsSQL = append(argsSQL, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), q, argsSQL...)
			if err != nil {
				return fmt.Errorf("sla query: %w", err)
			}
			defer rows.Close()
			type row struct {
				ID              string `json:"id"`
				ClientName      string `json:"client_name"`
				AgentName       string `json:"agent_name"`
				Status          string `json:"status"`
				Summary         string `json:"summary"`
				TargetDate      string `json:"target_date"`
				MinutesToBreach int    `json:"minutes_to_breach"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var mins sql.NullInt64
				if err := rows.Scan(&r.ID, &r.ClientName, &r.AgentName, &r.Status, &r.Summary, &r.TargetDate, &mins); err != nil {
					continue
				}
				r.MinutesToBreach = int(mins.Int64)
				out = append(out, r)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"within_hours": hours,
					"team":         team,
					"tickets":      out,
					"count":        len(out),
					"generated_at": time.Now().UTC().Format(time.RFC3339),
				})
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No tickets breaching SLA in the next %g hours.\n", hours)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Tickets breaching SLA in the next %g hours (%d)\n\n", hours, len(out))
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-7s %-22s %-22s %-12s %s\n", "ID", "MIN→", "CLIENT", "AGENT", "STATUS", "SUMMARY")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 110))
			for _, r := range out {
				summary := r.Summary
				if len(summary) > 30 {
					summary = summary[:30] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-7d %-22s %-22s %-12s %s\n", r.ID, r.MinutesToBreach, r.ClientName, r.AgentName, r.Status, summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&team, "team", "", "Limit to one team")
	cmd.Flags().StringVar(&within, "within", "24h", "Breach window (e.g. '24h', '4h', '15m')")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max tickets to return")
	return cmd
}

func parseWithinHours(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 24, nil
	}
	if dur, err := time.ParseDuration(s); err == nil {
		return dur.Hours(), nil
	}
	// bare integer treated as hours
	var h float64
	if _, err := fmt.Sscanf(s, "%f", &h); err == nil && h > 0 {
		return h, nil
	}
	return 0, fmt.Errorf("expected a duration like '24h' or a number of hours")
}
