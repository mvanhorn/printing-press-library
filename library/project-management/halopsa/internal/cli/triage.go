// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newTriageCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		team      string
		agent     string
		staleDays int
		breachHrs int
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Per-agent open ticket load + stale count + 24h SLA-breach count, in one table",
		Long: `Cross-entity dispatcher view that Halo's UI scatters across five tabs:
per-agent open ticket count, stale-ticket count, and tickets approaching SLA breach
in the next N hours. Joins tickets x agents x statuses locally.

Run 'halopsa-pp-cli sync' first to populate the local database.`,
		Example: strings.Trim(`
  # Default triage across all agents
  halopsa-pp-cli triage --json

  # Scope to a team and the last 14 days of staleness
  halopsa-pp-cli triage --team Support --stale-days 14

  # Tighten breach window
  halopsa-pp-cli triage --breach-within 4 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
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

			where := []string{"COALESCE(json_extract(data, '$.status_id'), 0) NOT IN (8, 9)"}
			argsSQL := []any{}
			if team != "" {
				where = append(where, "(team = ? OR json_extract(data, '$.team') = ?)")
				argsSQL = append(argsSQL, team, team)
			}
			if agent != "" {
				where = append(where, "(agent_name = ? OR LOWER(agent_name) = LOWER(?))")
				argsSQL = append(argsSQL, agent, agent)
			}
			q := `SELECT
                COALESCE(NULLIF(agent_name,''), '(unassigned)') AS who,
                COUNT(*) AS open_count,
                SUM(CASE WHEN (julianday('now') - julianday(COALESCE(NULLIF(json_extract(data, '$.lastactiondate'),''), datecreated))) > ? THEN 1 ELSE 0 END) AS stale_count,
                SUM(CASE WHEN datetime(COALESCE(json_extract(data, '$.targetdate'), '')) BETWEEN datetime('now') AND datetime('now', '+' || ? || ' hours') THEN 1 ELSE 0 END) AS breach_count,
                CAST(MAX(julianday('now') - julianday(datecreated)) AS INTEGER) AS oldest_days
            FROM tickets
            WHERE ` + strings.Join(where, " AND ") + `
            GROUP BY who
            ORDER BY open_count DESC, breach_count DESC
            LIMIT ?`
			finalArgs := append([]any{staleDays, breachHrs}, argsSQL...)
			finalArgs = append(finalArgs, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), q, finalArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type row struct {
				Agent       string `json:"agent"`
				OpenCount   int    `json:"open"`
				StaleCount  int    `json:"stale"`
				BreachCount int    `json:"breaching"`
				OldestDays  int    `json:"oldest_days"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var stale, breach, oldest sql.NullInt64
				if err := rows.Scan(&r.Agent, &r.OpenCount, &stale, &breach, &oldest); err != nil {
					continue
				}
				r.StaleCount = int(stale.Int64)
				r.BreachCount = int(breach.Int64)
				r.OldestDays = int(oldest.Int64)
				out = append(out, r)
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].OpenCount > out[j].OpenCount })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"team":          team,
					"stale_days":    staleDays,
					"breach_within": breachHrs,
					"agents":        out,
					"generated_at":  time.Now().UTC().Format(time.RFC3339),
				})
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open tickets in scope. Run 'halopsa-pp-cli sync' if the database looks empty.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Triage (stale > %dd, breach within %dh)\n\n", staleDays, breachHrs)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %6s %6s %9s %12s\n", "AGENT", "OPEN", "STALE", "BREACHING", "OLDEST(DAYS)")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 70))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %6d %6d %9d %12d\n", r.Agent, r.OpenCount, r.StaleCount, r.BreachCount, r.OldestDays)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&team, "team", "", "Limit to one team")
	cmd.Flags().StringVar(&agent, "agent", "", "Limit to one agent (matches agent_name)")
	cmd.Flags().IntVar(&staleDays, "stale-days", 7, "Days without action before a ticket counts as stale")
	cmd.Flags().IntVar(&breachHrs, "breach-within", 24, "Hours-to-breach window")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max agents to show")
	_ = json.Compact
	return cmd
}
