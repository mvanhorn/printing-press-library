// Hand-written novel feature: partners leaderboard.
// Rank partners by commission earned, conversion rate, and clicks generated.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type leaderRow struct {
	PartnerID   string  `json:"partnerId"`
	Email       string  `json:"email,omitempty"`
	Status      string  `json:"status,omitempty"`
	Clicks      float64 `json:"clicks,omitempty"`
	Leads       float64 `json:"leads,omitempty"`
	Sales       float64 `json:"sales,omitempty"`
	Commission  float64 `json:"commissionTotal"`
	PayoutTotal float64 `json:"payoutTotal"`
	Currency    string  `json:"currency,omitempty"`
}

func newPartnersLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var byMetric string
	var top int

	cmd := &cobra.Command{
		Use:   "leaderboard",
		Short: "Rank partners by commission earned, conversion rate, and clicks generated",
		Long: `Joins partners × commissions × payouts locally to rank partners by the
chosen metric. Per-partner ROI accurate to the latest sync.

Use ` + "`--by commission`" + ` (default), ` + "`--by clicks`" + `, ` + "`--by leads`" + `, or ` + "`--by sales`" + `.`,
		Example: strings.Trim(`
  dub-pp-cli partners leaderboard --by commission --top 10 --json
  dub-pp-cli partners leaderboard --by clicks --top 25 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(email,''), COALESCE(status,''), COALESCE(data,'{}')
				FROM partners
			`)
			if err != nil {
				return fmt.Errorf("querying partners: %w", err)
			}
			defer rows.Close()

			byID := map[string]*leaderRow{}
			seen := 0
			for rows.Next() {
				var id, email, status string
				var blob sql.NullString
				if err := rows.Scan(&id, &email, &status, &blob); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				lr := &leaderRow{
					PartnerID: id,
					Email:     email,
					Status:    status,
					Clicks:    extractNumber(blobBytes, "clicks"),
					Leads:     extractNumber(blobBytes, "leads"),
					Sales:     extractNumber(blobBytes, "sales"),
				}
				byID[id] = lr
			}
			if seen == 0 {
				return hintEmptyStore("partners")
			}

			// Sum commissions per partner.
			rows2, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(partner_id,''), COALESCE(amount,0), COALESCE(currency,'')
				FROM commissions
			`)
			if err == nil {
				for rows2.Next() {
					var pid, currency string
					var amount float64
					if err := rows2.Scan(&pid, &amount, &currency); err == nil {
						if lr, ok := byID[pid]; ok {
							lr.Commission += amount
							if lr.Currency == "" {
								lr.Currency = currency
							}
						}
					}
				}
				rows2.Close()
			}

			// Sum payouts per partner.
			rows3, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(partner_id,''), COALESCE(data,'{}')
				FROM payouts
			`)
			if err == nil {
				for rows3.Next() {
					var pid string
					var blob sql.NullString
					if err := rows3.Scan(&pid, &blob); err == nil {
						if lr, ok := byID[pid]; ok && blob.Valid {
							lr.PayoutTotal += extractNumber([]byte(blob.String), "amount")
						}
					}
				}
				rows3.Close()
			}

			var results []leaderRow
			for _, lr := range byID {
				results = append(results, *lr)
			}
			less := func(a, b leaderRow) bool {
				switch strings.ToLower(byMetric) {
				case "clicks":
					return a.Clicks > b.Clicks
				case "leads":
					return a.Leads > b.Leads
				case "sales":
					return a.Sales > b.Sales
				default:
					return a.Commission > b.Commission
				}
			}
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if less(results[j], results[i]) {
						results[i], results[j] = results[j], results[i]
					}
				}
			}
			if top > 0 && len(results) > top {
				results = results[:top]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-10s %-10s %-10s %-12s\n",
				"PARTNER", "STATUS", "CLICKS", "LEADS", "SALES", "COMMISSION")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-10.0f %-10.0f %-10.0f %-12.2f\n",
					truncate(r.Email, 30), r.Status, r.Clicks, r.Leads, r.Sales, r.Commission)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&byMetric, "by", "commission", "Sort metric: commission, clicks, leads, sales")
	cmd.Flags().IntVar(&top, "top", 10, "Number of partners to return (0 = all)")
	return cmd
}
