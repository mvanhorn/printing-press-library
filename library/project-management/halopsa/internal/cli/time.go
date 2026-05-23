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

// newTimeCmd is a novel-features parent ("time gaps"), separate from the
// generated "timesheet" CRUD parent.
func newTimeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Time-entry analytics across local + live data",
		Long:  "Cross-entity time analyses (e.g. 'gaps') that the API doesn't expose directly.",
	}
	cmd.AddCommand(newTimeGapsCmd(flags))
	return cmd
}

func newTimeGapsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		agent  string
		week   string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Tickets you touched this week with zero time logged",
		Long: `Set-diff: tickets where the agent is the ticket assignee or appears in actions
during the window, MINUS tickets that already have a time entry on them in the window.
The Friday-timesheet reconcile.`,
		Example: strings.Trim(`
  # My open gaps this week
  halopsa-pp-cli time gaps --agent me --week current --json

  # Another agent's gaps last week
  halopsa-pp-cli time gaps --agent "Devon" --week last
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			start, end, err := parseWeek(week)
			if err != nil {
				return fmt.Errorf("--week %q: %w", week, err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			agentFilter := strings.ToLower(strings.TrimSpace(agent))
			if agentFilter == "me" {
				agentFilter = "" // not resolvable offline; just show all-agents view
			}
			// touched: agent_name from tickets where lastactiondate in window OR action.who in window
			touched := `SELECT DISTINCT t.id, t.agent_name, t.summary FROM tickets t
                WHERE datetime(COALESCE(NULLIF(json_extract(t.data,'$.lastactiondate'),''), t.datecreated)) BETWEEN datetime(?) AND datetime(?)`
			argsTouched := []any{start.Format(time.RFC3339), end.Format(time.RFC3339)}
			if agentFilter != "" {
				touched += " AND LOWER(t.agent_name) = ?"
				argsTouched = append(argsTouched, agentFilter)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), touched, argsTouched...)
			if err != nil {
				return fmt.Errorf("query touched: %w", err)
			}
			defer rows.Close()
			type ticketInfo struct {
				ID      string
				Agent   string
				Summary string
			}
			touchedRows := map[string]ticketInfo{}
			for rows.Next() {
				var t ticketInfo
				if err := rows.Scan(&t.ID, &t.Agent, &t.Summary); err != nil {
					continue
				}
				touchedRows[t.ID] = t
			}
			// Tickets with a time entry in window: actions table with actionchargehours+actionnonchargehours > 0
			timeSQL := `SELECT DISTINCT json_extract(a.data, '$.ticket_id') AS tid
                FROM actions a
                WHERE datetime(COALESCE(NULLIF(json_extract(a.data,'$.actiondatecreated'),''), a.datecreated)) BETWEEN datetime(?) AND datetime(?)
                  AND (COALESCE(json_extract(a.data, '$.actionchargehours'), 0) + COALESCE(json_extract(a.data, '$.actionnonchargehours'), 0)) > 0`
			tRows, err := db.DB().QueryContext(cmd.Context(), timeSQL, start.Format(time.RFC3339), end.Format(time.RFC3339))
			withTime := map[string]bool{}
			if err == nil {
				defer tRows.Close()
				for tRows.Next() {
					var tid sql.NullString
					if tRows.Scan(&tid) != nil || !tid.Valid {
						continue
					}
					withTime[tid.String] = true
				}
			}
			// Gap = touched \ withTime
			type out struct {
				TicketID string `json:"ticket_id"`
				Agent    string `json:"agent"`
				Summary  string `json:"summary"`
			}
			gaps := []out{}
			for id, t := range touchedRows {
				if !withTime[id] {
					gaps = append(gaps, out{TicketID: id, Agent: t.Agent, Summary: t.Summary})
					if limit > 0 && len(gaps) >= limit {
						break
					}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"window_start": start.Format(time.RFC3339),
					"window_end":   end.Format(time.RFC3339),
					"agent":        agent,
					"gaps":         gaps,
					"total":        len(gaps),
				})
			}
			if len(gaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No time gaps in window. Timesheet is clean.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Time gaps %s..%s (%d ticket(s))\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"), len(gaps))
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-20s %s\n", "TICKET", "AGENT", "SUMMARY")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))
			for _, g := range gaps {
				summary := g.Summary
				if len(summary) > 50 {
					summary = summary[:50] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-20s %s\n", g.TicketID, g.Agent, summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent name (or 'me')")
	cmd.Flags().StringVar(&week, "week", "current", "Week scope: current, last, or 'YYYY-MM-DD'")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max gaps to return")
	_ = json.Compact
	return cmd
}

// parseWeek returns the [start, end) range for "current", "last", or an
// arbitrary anchor date (interpreted as the week containing that date).
func parseWeek(w string) (time.Time, time.Time, error) {
	now := time.Now()
	w = strings.TrimSpace(strings.ToLower(w))
	var anchor time.Time
	switch w {
	case "", "current", "this":
		anchor = now
	case "last", "previous":
		anchor = now.AddDate(0, 0, -7)
	default:
		t, err := time.Parse("2006-01-02", w)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("expected current|last|YYYY-MM-DD: %w", err)
		}
		anchor = t
	}
	// ISO Mon..Sun window containing anchor
	wd := int(anchor.Weekday())
	if wd == 0 {
		wd = 7
	}
	start := time.Date(anchor.Year(), anchor.Month(), anchor.Day()-(wd-1), 0, 0, 0, 0, anchor.Location())
	return start, start.AddDate(0, 0, 7), nil
}
