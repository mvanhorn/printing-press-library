// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newAgentWorkloadCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		team   string
		since  string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Per-agent open tickets, touched-this-week, billable hours, oldest open age",
		Long: `Cross-joins tickets x actions for each agent over the window. Surfaces
"who's overloaded" and where re-balance moves help.`,
		Example: strings.Trim(`
  # Workload across all agents this week
  halopsa-pp-cli agent workload --json

  # Scope to a team and last 14 days
  halopsa-pp-cli agent workload --team Support --since 14d
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			t, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("--since %q: %w", since, err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			whereTeam := ""
			argsSQL := []any{}
			if team != "" {
				whereTeam = " AND (team = ? OR json_extract(data,'$.team') = ?)"
				argsSQL = append(argsSQL, team, team)
			}
			// Open counts + oldest age
			openSQL := `SELECT
                COALESCE(NULLIF(agent_name,''),'(unassigned)') AS agent,
                COUNT(*) AS open_count,
                CAST(MAX(julianday('now') - julianday(datecreated)) AS INTEGER) AS oldest_days
                FROM tickets
                WHERE COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9) ` + whereTeam + `
                GROUP BY agent`
			rows, err := db.DB().QueryContext(cmd.Context(), openSQL, argsSQL...)
			if err != nil {
				return fmt.Errorf("workload open: %w", err)
			}
			defer rows.Close()
			type entry struct {
				Agent       string  `json:"agent"`
				Open        int     `json:"open"`
				TouchedWeek int     `json:"touched_in_window"`
				Hours       float64 `json:"hours_logged"`
				Oldest      int     `json:"oldest_days"`
			}
			byAgent := map[string]*entry{}
			for rows.Next() {
				var e entry
				var oldest sql.NullInt64
				if err := rows.Scan(&e.Agent, &e.Open, &oldest); err != nil {
					continue
				}
				e.Oldest = int(oldest.Int64)
				byAgent[e.Agent] = &e
			}
			// Touched in window: tickets whose lastactiondate falls within
			touchedSQL := `SELECT COALESCE(NULLIF(agent_name,''),'(unassigned)'), COUNT(*) FROM tickets
                WHERE datetime(COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated)) >= datetime(?) ` + whereTeam + `
                GROUP BY agent_name`
			tArgs := append([]any{t.Format(time.RFC3339)}, argsSQL...)
			if tRows, terr := db.DB().QueryContext(cmd.Context(), touchedSQL, tArgs...); terr == nil {
				defer tRows.Close()
				for tRows.Next() {
					var ag string
					var n int
					if tRows.Scan(&ag, &n) != nil {
						continue
					}
					if _, ok := byAgent[ag]; !ok {
						byAgent[ag] = &entry{Agent: ag}
					}
					byAgent[ag].TouchedWeek = n
				}
			}
			// Hours logged in window via actions.actor name
			hSQL := `SELECT COALESCE(NULLIF(json_extract(data,'$.who'),''),'(unassigned)') AS who,
                SUM(COALESCE(json_extract(data,'$.actionchargehours'),0) + COALESCE(json_extract(data,'$.actionnonchargehours'),0)) AS hrs
                FROM actions
                WHERE datetime(COALESCE(NULLIF(json_extract(data,'$.actiondatecreated'),''), datetime('now'))) >= datetime(?)
                GROUP BY who`
			if hRows, herr := db.DB().QueryContext(cmd.Context(), hSQL, t.Format(time.RFC3339)); herr == nil {
				defer hRows.Close()
				for hRows.Next() {
					var ag string
					var hrs sql.NullFloat64
					if hRows.Scan(&ag, &hrs) != nil {
						continue
					}
					if _, ok := byAgent[ag]; !ok {
						byAgent[ag] = &entry{Agent: ag}
					}
					byAgent[ag].Hours = hrs.Float64
				}
			}
			out := make([]entry, 0, len(byAgent))
			for _, e := range byAgent {
				out = append(out, *e)
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Open > out[j].Open })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"team":   team,
					"since":  t.Format(time.RFC3339),
					"agents": out,
					"count":  len(out),
				})
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agents matched. Run 'halopsa-pp-cli sync' if the database looks empty.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Agent workload (since %s)\n\n", t.Format("2006-01-02"))
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %6s %15s %10s %12s\n", "AGENT", "OPEN", "TOUCHED_WIN", "HRS_LOG", "OLDEST(d)")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %6d %15d %10.2f %12d\n", e.Agent, e.Open, e.TouchedWeek, e.Hours, e.Oldest)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&team, "team", "", "Limit to one team")
	cmd.Flags().StringVar(&since, "since", "7d", "Window for 'touched' + 'hours': '7d', 'yesterday', '2026-05-15T00:00:00Z'")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max agents to return")
	return cmd
}
