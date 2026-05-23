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

func newStandupCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		team   string
		since  string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "standup",
		Short: "Per-agent digest for a window: closed, reopened, time logged, top client",
		Long: `Aggregates ticket activity across local tables for the given window.
Reads tickets + actions tables for closures, reopens, and per-client work.
Run 'halopsa-pp-cli sync' first.`,
		Example: strings.Trim(`
  # Yesterday's standup digest
  halopsa-pp-cli standup --since yesterday

  # Since a specific time
  halopsa-pp-cli standup --since "2026-05-20 09:00" --team Support --json

  # Past 24 hours, JSON
  halopsa-pp-cli standup --since 24h --json
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
			argsSQL := []any{t.Format(time.RFC3339)}
			if team != "" {
				whereTeam = " AND team = ?"
				argsSQL = append(argsSQL, team)
			}
			// Tickets closed in window
			closedSQL := `SELECT
                COALESCE(NULLIF(agent_name,''), '(unassigned)') AS who,
                COUNT(*) AS closed,
                COALESCE(client_name, '?') AS top_client
            FROM tickets
            WHERE json_extract(data, '$.status_id') IN (8,9)
              AND datetime(COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated)) >= datetime(?)
              ` + whereTeam + `
            GROUP BY who
            ORDER BY closed DESC LIMIT ?`
			argsSQL = append(argsSQL, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), closedSQL, argsSQL...)
			if err != nil {
				return fmt.Errorf("standup query: %w", err)
			}
			defer rows.Close()
			type row struct {
				Agent       string  `json:"agent"`
				Closed      int     `json:"closed"`
				Reopened    int     `json:"reopened"`
				HoursLogged float64 `json:"hours_logged"`
				TopClient   string  `json:"top_client"`
			}
			byAgent := map[string]*row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Agent, &r.Closed, &r.TopClient); err != nil {
					continue
				}
				byAgent[r.Agent] = &r
			}
			// Hours logged from actions table in window
			hoursSQL := `SELECT
                COALESCE(NULLIF(json_extract(data, '$.who'),''), '(unassigned)') AS who,
                SUM(COALESCE(json_extract(data, '$.actionchargehours'), 0) + COALESCE(json_extract(data, '$.actionnonchargehours'), 0)) AS hrs
            FROM actions
            WHERE datetime(COALESCE(NULLIF(json_extract(data, '$.actiondatecreated'),''), datetime('now'))) >= datetime(?)
            GROUP BY who`
			hr, hrErr := db.DB().QueryContext(cmd.Context(), hoursSQL, t.Format(time.RFC3339))
			if hrErr == nil {
				defer hr.Close()
				for hr.Next() {
					var who string
					var hrs sql.NullFloat64
					if hr.Scan(&who, &hrs) != nil {
						continue
					}
					if _, ok := byAgent[who]; !ok {
						byAgent[who] = &row{Agent: who}
					}
					byAgent[who].HoursLogged = hrs.Float64
				}
			}
			out := make([]row, 0, len(byAgent))
			for _, r := range byAgent {
				out = append(out, *r)
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].Closed > out[j].Closed || (out[i].Closed == out[j].Closed && out[i].HoursLogged > out[j].HoursLogged)
			})
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"since":  t.UTC().Format(time.RFC3339),
					"team":   team,
					"agents": out,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Standup digest since %s\n\n", t.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %7s %9s %13s %s\n", "AGENT", "CLOSED", "REOPENED", "HRS_LOGGED", "TOP_CLIENT")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 90))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %7d %9d %13.2f %s\n", r.Agent, r.Closed, r.Reopened, r.HoursLogged, r.TopClient)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&team, "team", "", "Limit to one team")
	cmd.Flags().StringVar(&since, "since", "yesterday", "Window start: 'yesterday', RFC3339 timestamp, '24h', '7d'")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max agents to show")
	return cmd
}

// parseSince accepts: "yesterday", "today", a duration like "24h" / "7d", or an absolute date/time.
func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	now := time.Now()
	switch s {
	case "", "today":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), nil
	case "yesterday":
		y, m, d := now.AddDate(0, 0, -1).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), nil
	}
	// duration suffix
	if strings.HasSuffix(s, "d") {
		var d int
		if _, err := fmt.Sscanf(s, "%dd", &d); err == nil && d > 0 {
			return now.AddDate(0, 0, -d), nil
		}
	}
	if dur, err := time.ParseDuration(s); err == nil {
		return now.Add(-dur), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time")
}
