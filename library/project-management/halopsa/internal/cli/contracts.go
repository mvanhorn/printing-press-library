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

// newContractsCmd is a novel parent ("contracts burn"). The generated CRUD
// for contracts lives under "client-contract".
func newContractsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contracts",
		Short: "Contract analytics (burn-down) across local client_contract + actions",
		Long:  "Cross-entity contract analyses the API and Halo Reports don't expose directly.",
	}
	cmd.AddCommand(newContractsBurnCmd(flags))
	return cmd
}

func newContractsBurnCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		client string
		month  string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Per contract: hours bank, hours consumed this period, projected overage",
		Long: `Sum billable time on each client's tickets for the period, compared to the
contract's prepaid hours bank. Surfaces tracking-over-bank contracts mid-month so
the conversation isn't a surprise. Reads client_contract + actions locally.`,
		Example: strings.Trim(`
  # Current month, all contracts
  halopsa-pp-cli contracts burn --month current --json

  # One client
  halopsa-pp-cli contracts burn --client "Acme Corp" --month current
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			start, end, err := parseMonth(month)
			if err != nil {
				return fmt.Errorf("--month %q: %w", month, err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()

			whereClient := ""
			argsBase := []any{}
			if client != "" {
				whereClient = " AND (cc.client_name = ? OR LOWER(cc.client_name) = LOWER(?))"
				argsBase = []any{client, client}
			}
			q := `SELECT
                cc.id,
                cc.client_id,
                COALESCE(cc.client_name, '?') AS client_name,
                COALESCE(cc.contracttype_name, '?') AS contract_type,
                COALESCE(cc.chargehoursperperiod, 0) AS bank_hrs
            FROM client_contract cc
            WHERE 1=1 ` + whereClient + `
            ORDER BY client_name LIMIT ?`
			finalArgs := append(argsBase, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), q, finalArgs...)
			if err != nil {
				return fmt.Errorf("contracts query: %w", err)
			}
			defer rows.Close()

			type burn struct {
				ContractID string  `json:"contract_id"`
				ClientID   string  `json:"client_id"`
				ClientName string  `json:"client_name"`
				Type       string  `json:"contract_type"`
				BankHours  float64 `json:"bank_hours"`
				UsedHours  float64 `json:"used_hours"`
				Remaining  float64 `json:"remaining_hours"`
				PctUsed    float64 `json:"percent_used"`
				DaysLeft   int     `json:"days_remaining"`
				Projected  float64 `json:"projected_overage_hours"`
			}
			out := []burn{}
			today := time.Now()
			daysInPeriod := int(end.Sub(start).Hours()/24) + 1
			daysElapsed := int(today.Sub(start).Hours()/24) + 1
			if daysElapsed < 1 {
				daysElapsed = 1
			}
			if daysElapsed > daysInPeriod {
				daysElapsed = daysInPeriod
			}
			daysLeft := daysInPeriod - daysElapsed
			for rows.Next() {
				var b burn
				var bankHrs sql.NullFloat64
				if err := rows.Scan(&b.ContractID, &b.ClientID, &b.ClientName, &b.Type, &bankHrs); err != nil {
					continue
				}
				b.BankHours = bankHrs.Float64
				// Sum hours on this client's tickets in window
				hSQL := `SELECT COALESCE(SUM(
                    COALESCE(json_extract(a.data,'$.actionchargehours'),0) +
                    COALESCE(json_extract(a.data,'$.actionnonchargehours'),0)
                ),0)
                FROM actions a, tickets t
                WHERE json_extract(a.data,'$.ticket_id') = t.id
                  AND t.client_id = ?
                  AND datetime(COALESCE(NULLIF(json_extract(a.data,'$.actiondatecreated'),''), a.datecreated))
                      BETWEEN datetime(?) AND datetime(?)`
				var used sql.NullFloat64
				_ = db.DB().QueryRowContext(cmd.Context(), hSQL, b.ClientID, start.Format(time.RFC3339), end.Format(time.RFC3339)).Scan(&used)
				b.UsedHours = used.Float64
				b.Remaining = b.BankHours - b.UsedHours
				if b.BankHours > 0 {
					b.PctUsed = (b.UsedHours / b.BankHours) * 100.0
				}
				b.DaysLeft = daysLeft
				if daysElapsed > 0 {
					projected := (b.UsedHours / float64(daysElapsed)) * float64(daysInPeriod)
					if projected > b.BankHours {
						b.Projected = projected - b.BankHours
					}
				}
				out = append(out, b)
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Projected > out[j].Projected })
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"period_start":   start.Format("2006-01-02"),
					"period_end":     end.Format("2006-01-02"),
					"days_in_period": daysInPeriod,
					"days_elapsed":   daysElapsed,
					"days_left":      daysLeft,
					"contracts":      out,
				})
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No contracts found for this scope.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Contract burn (%s..%s, %d days, %d elapsed)\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"), daysInPeriod, daysElapsed)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %12s %10s %10s %8s %10s\n", "CLIENT", "TYPE", "BANK", "USED", "PCT", "PROJ-OVER")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 90))
			for _, b := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %12s %10.2f %10.2f %7.1f%% %10.2f\n", b.ClientName, b.Type, b.BankHours, b.UsedHours, b.PctUsed, b.Projected)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&client, "client", "", "Limit to one client (by name)")
	cmd.Flags().StringVar(&month, "month", "current", "Period: current, last, or 'YYYY-MM'")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max contracts to scan")
	_ = json.Compact
	return cmd
}

func parseMonth(m string) (time.Time, time.Time, error) {
	now := time.Now()
	m = strings.TrimSpace(strings.ToLower(m))
	switch m {
	case "", "current", "this":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return first, first.AddDate(0, 1, 0), nil
	case "last", "previous":
		first := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
		return first, first.AddDate(0, 1, 0), nil
	}
	if t, err := time.Parse("2006-01", m); err == nil {
		return t, t.AddDate(0, 1, 0), nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("expected current|last|YYYY-MM")
}
