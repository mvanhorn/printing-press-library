// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newClientCardCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limitT int
	)
	cmd := &cobra.Command{
		Use:   "card [client]",
		Short: "Six-table panel: client + sites + active tickets + contracts + assets + KB",
		Long: `Joins client + site + tickets (open) + client_contract + asset locally for one
client. Resolves by numeric ID or by name (case-insensitive substring match).`,
		Example: strings.Trim(`
  # By name
  halopsa-pp-cli client card "Acme Corp" --json

  # By numeric id
  halopsa-pp-cli client card 42

  # Compact set of fields for an agent
  halopsa-pp-cli client card "Acme Corp" --agent --select client,active_tickets,contract_hours_remaining,assets_count
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
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
			// Resolve client by id or name
			var clientID, clientName string
			byID := false
			if _, err := strconv.Atoi(query); err == nil {
				byID = true
			}
			if byID {
				if err := db.DB().QueryRowContext(cmd.Context(), `SELECT id, COALESCE(name, COALESCE(json_extract(data,'$.name'),'?')) FROM client WHERE id = ?`, query).Scan(&clientID, &clientName); err != nil {
					return fmt.Errorf("client %q not found in local store: %w", query, err)
				}
			} else {
				if err := db.DB().QueryRowContext(cmd.Context(), `SELECT id, COALESCE(name, COALESCE(json_extract(data,'$.name'),'?')) FROM client WHERE LOWER(COALESCE(name, json_extract(data,'$.name'))) LIKE LOWER(?) ORDER BY length(COALESCE(name, json_extract(data,'$.name'))) LIMIT 1`, "%"+query+"%").Scan(&clientID, &clientName); err != nil {
					return fmt.Errorf("client matching %q not found in local store: %w", query, err)
				}
			}
			card := map[string]any{
				"client_id":   clientID,
				"client_name": clientName,
			}
			// Sites
			sitesRows, _ := db.DB().QueryContext(cmd.Context(), `SELECT id, COALESCE(json_extract(data,'$.name'),'?') FROM site WHERE client_id = ?`, clientID)
			sites := []map[string]any{}
			if sitesRows != nil {
				defer sitesRows.Close()
				for sitesRows.Next() {
					var id, name string
					if err := sitesRows.Scan(&id, &name); err == nil {
						sites = append(sites, map[string]any{"id": id, "name": name})
					}
				}
			}
			card["sites"] = sites

			// Active tickets
			tRows, _ := db.DB().QueryContext(cmd.Context(), `SELECT id, COALESCE(summary,''), COALESCE(agent_name,'?'),
                COALESCE(json_extract(data,'$.status_name'),'?'),
                COALESCE(json_extract(data,'$.targetdate'),'')
                FROM tickets
                WHERE client_id = ? AND COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)
                ORDER BY COALESCE(json_extract(data,'$.targetdate'),'') ASC
                LIMIT ?`, clientID, limitT)
			activeTickets := []map[string]any{}
			if tRows != nil {
				defer tRows.Close()
				for tRows.Next() {
					var id, summary, agent, status, target string
					if err := tRows.Scan(&id, &summary, &agent, &status, &target); err == nil {
						activeTickets = append(activeTickets, map[string]any{"id": id, "summary": summary, "agent": agent, "status": status, "target_date": target})
					}
				}
			}
			card["active_tickets"] = activeTickets
			card["active_tickets_count"] = len(activeTickets)

			// Contracts and remaining hours (estimated by chargehoursperperiod - already-used)
			cRows, _ := db.DB().QueryContext(cmd.Context(), `SELECT id,
                COALESCE(contracttype_name,'?'),
                COALESCE(chargehoursperperiod,0),
                COALESCE(json_extract(data,'$.contract_prepayused'),0),
                COALESCE(json_extract(data,'$.contract_prepaybalance'),0),
                COALESCE(end_date,'')
                FROM client_contract WHERE client_id = ?`, clientID)
			contracts := []map[string]any{}
			contractHoursRemaining := 0.0
			if cRows != nil {
				defer cRows.Close()
				for cRows.Next() {
					var id, ctype, end string
					var bank, used, balance sql.NullFloat64
					if err := cRows.Scan(&id, &ctype, &bank, &used, &balance, &end); err == nil {
						remaining := bank.Float64 - used.Float64
						contractHoursRemaining += remaining
						contracts = append(contracts, map[string]any{
							"id":              id,
							"type":            ctype,
							"bank_hours":      bank.Float64,
							"used_hours":      used.Float64,
							"remaining_hours": remaining,
							"balance":         balance.Float64,
							"end_date":        end,
						})
					}
				}
			}
			card["contracts"] = contracts
			card["contract_hours_remaining"] = contractHoursRemaining

			// Asset count
			var assetCount int
			_ = db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM asset WHERE client_id = ?`, clientID).Scan(&assetCount)
			card["assets_count"] = assetCount

			// Recent KB articles linked to this client's tickets (mechanical lookup if kb_link_ids JSON column exists)
			kbRows, _ := db.DB().QueryContext(cmd.Context(), `SELECT DISTINCT json_each.value FROM tickets, json_each(COALESCE(json_extract(tickets.data,'$.kb_articles'),'[]'))
                WHERE tickets.client_id = ? LIMIT 5`, clientID)
			recentKB := []any{}
			if kbRows != nil {
				defer kbRows.Close()
				for kbRows.Next() {
					var v any
					if err := kbRows.Scan(&v); err == nil {
						recentKB = append(recentKB, v)
					}
				}
			}
			card["recent_kb_links"] = recentKB

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, card)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "=== %s (id %s) ===\n\n", clientName, clientID)
			fmt.Fprintf(cmd.OutOrStdout(), "Sites:             %d\n", len(sites))
			fmt.Fprintf(cmd.OutOrStdout(), "Active tickets:    %d\n", len(activeTickets))
			fmt.Fprintf(cmd.OutOrStdout(), "Contracts:         %d (%.2f hrs remaining)\n", len(contracts), contractHoursRemaining)
			fmt.Fprintf(cmd.OutOrStdout(), "Assets:            %d\n", assetCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Recent KB links:   %d\n", len(recentKB))
			if len(activeTickets) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTop active tickets:")
				for _, t := range activeTickets {
					fmt.Fprintf(cmd.OutOrStdout(), "  #%v  %s — %s (status: %s)\n", t["id"], t["agent"], t["summary"], t["status"])
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limitT, "limit-tickets", 25, "Max active tickets to include")
	return cmd
}

func newClientOverlayCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		metric string
		top    int
	)
	cmd := &cobra.Command{
		Use:   "overlay",
		Short: "Rank all clients by a metric (open_tickets, stale, sla_at_risk, hours_over_bank)",
		Long:  `Pluggable cross-client rank. Local SQL only.`,
		Example: strings.Trim(`
  # Top 10 clients by open tickets
  halopsa-pp-cli client overlay --metric open_tickets --top 10 --json

  # Most at-risk for SLA breach (next 24h)
  halopsa-pp-cli client overlay --metric sla_at_risk --top 20
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
			var q string
			switch metric {
			case "open_tickets":
				q = `SELECT COALESCE(client_name,'?') AS client, COUNT(*) AS metric
                    FROM tickets
                    WHERE COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)
                    GROUP BY client GROUP BY client ORDER BY metric DESC LIMIT ?`
				// Note: GROUP BY twice in SQL is invalid; fix below
				q = `SELECT COALESCE(client_name,'?') AS client, COUNT(*) AS metric
                    FROM tickets
                    WHERE COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)
                    GROUP BY client ORDER BY metric DESC LIMIT ?`
			case "stale":
				q = `SELECT COALESCE(client_name,'?') AS client,
                    SUM(CASE WHEN (julianday('now') - julianday(COALESCE(NULLIF(json_extract(data,'$.lastactiondate'),''), datecreated))) > 7 THEN 1 ELSE 0 END) AS metric
                    FROM tickets
                    WHERE COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)
                    GROUP BY client ORDER BY metric DESC LIMIT ?`
			case "sla_at_risk":
				q = `SELECT COALESCE(client_name,'?') AS client,
                    SUM(CASE WHEN datetime(COALESCE(json_extract(data,'$.targetdate'),'')) BETWEEN datetime('now') AND datetime('now', '+24 hours') THEN 1 ELSE 0 END) AS metric
                    FROM tickets
                    WHERE COALESCE(json_extract(data,'$.status_id'),0) NOT IN (8,9)
                    GROUP BY client ORDER BY metric DESC LIMIT ?`
			case "hours_over_bank":
				q = `SELECT COALESCE(cc.client_name,'?') AS client,
                    SUM(COALESCE(cc.contract_prepayused,0) - COALESCE(cc.chargehoursperperiod,0)) AS metric
                    FROM client_contract cc
                    GROUP BY client
                    HAVING metric > 0
                    ORDER BY metric DESC LIMIT ?`
			default:
				return fmt.Errorf("unknown --metric %q; choose: open_tickets, stale, sla_at_risk, hours_over_bank", metric)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), q, top)
			if err != nil {
				return fmt.Errorf("overlay query: %w", err)
			}
			defer rows.Close()
			type row struct {
				Client string  `json:"client"`
				Value  float64 `json:"value"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Client, &r.Value); err != nil {
					continue
				}
				out = append(out, r)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"metric":  metric,
					"top":     top,
					"clients": out,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Client overlay (metric: %s, top %d)\n\n", metric, top)
			fmt.Fprintf(cmd.OutOrStdout(), "%-50s %12s\n", "CLIENT", "VALUE")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 65))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-50s %12.2f\n", r.Client, r.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&metric, "metric", "open_tickets", "Metric: open_tickets, stale, sla_at_risk, hours_over_bank")
	cmd.Flags().IntVar(&top, "top", 10, "Top N clients to show")
	return cmd
}
