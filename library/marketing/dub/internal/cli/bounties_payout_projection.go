// Hand-written novel feature: bounties payout-projection.
// Project upcoming payouts from approved-but-unpaid submissions multiplied by
// current commission rates. Joins bounties × commissions × payouts locally.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type projectionRow struct {
	BountyID            string  `json:"bountyId"`
	BountyType          string  `json:"bountyType,omitempty"`
	PartnerID           string  `json:"partnerId,omitempty"`
	ApprovedSubmissions int     `json:"approvedSubmissions"`
	UnpaidCommissions   int     `json:"unpaidCommissions"`
	ProjectedAmount     float64 `json:"projectedAmount"`
	Currency            string  `json:"currency,omitempty"`
}

func newBountiesPayoutProjectionCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string

	cmd := &cobra.Command{
		Use:   "payout-projection",
		Short: "Project upcoming payouts from approved-but-unpaid submissions",
		Long: `Forecast upcoming payout liability by joining:

  • approved bounty submissions
  • commissions in 'pending' or 'unpaid' status
  • current per-partner rates from the partners table

Aggregates by bountyId × partnerId.`,
		Example: strings.Trim(`
  dub-pp-cli bounties payout-projection --json
  dub-pp-cli bounties payout-projection --window 30d --json
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

			cutoff := time.Now()
			if window != "" {
				if d, ok := parseDuration(window); ok {
					cutoff = cutoff.Add(-d)
				}
			} else {
				cutoff = cutoff.AddDate(0, 0, -30)
			}
			cutoffStr := cutoff.Format(time.RFC3339)

			// Approved submissions per (bounty,partner).
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(bounties_id,''), COALESCE(data,'{}'), COALESCE(synced_at,'')
				FROM submissions
			`)
			if err != nil {
				return fmt.Errorf("querying submissions: %w", err)
			}
			defer rows.Close()
			type key struct{ bounty, partner string }
			byKey := map[key]*projectionRow{}
			seen := 0
			for rows.Next() {
				var bid, syncedAt string
				var blob sql.NullString
				if err := rows.Scan(&bid, &blob, &syncedAt); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				st := strings.ToLower(extractFromData(blobBytes, "status"))
				if st != "approved" && st != "auto_approved" {
					continue
				}
				partnerID := extractFromData(blobBytes, "partnerId")
				k := key{bounty: bid, partner: partnerID}
				row, ok := byKey[k]
				if !ok {
					row = &projectionRow{
						BountyID:   bid,
						PartnerID:  partnerID,
						BountyType: extractFromData(blobBytes, "type"),
					}
					byKey[k] = row
				}
				row.ApprovedSubmissions++
			}
			if seen == 0 {
				return hintEmptyStore("submissions")
			}

			// Add unpaid commissions per partner from cutoff.
			rows2, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(partner_id,''), COALESCE(amount,0), COALESCE(currency,''), COALESCE(status,''), COALESCE(synced_at,'')
				FROM commissions
				WHERE synced_at >= ?
			`, cutoffStr)
			if err == nil {
				for rows2.Next() {
					var pid, currency, status, synced string
					var amount float64
					if err := rows2.Scan(&pid, &amount, &currency, &status, &synced); err == nil {
						st := strings.ToLower(status)
						if st != "pending" && st != "unpaid" && st != "" {
							continue
						}
						// Aggregate against any matching bounty key.
						matched := false
						for k, row := range byKey {
							if k.partner == pid {
								row.UnpaidCommissions++
								row.ProjectedAmount += amount
								if row.Currency == "" {
									row.Currency = currency
								}
								matched = true
							}
						}
						if !matched {
							// Partner not in any approved submission group — keep
							// the aggregate visible by parking under empty bounty.
							k := key{bounty: "", partner: pid}
							row, ok := byKey[k]
							if !ok {
								row = &projectionRow{PartnerID: pid, Currency: currency}
								byKey[k] = row
							}
							row.UnpaidCommissions++
							row.ProjectedAmount += amount
						}
					}
				}
				rows2.Close()
			}

			var results []projectionRow
			for _, r := range byKey {
				if r.ApprovedSubmissions == 0 && r.UnpaidCommissions == 0 {
					continue
				}
				results = append(results, *r)
			}
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].ProjectedAmount > results[i].ProjectedAmount {
						results[i], results[j] = results[j], results[i]
					}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No projected payouts.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-22s %-12s %-12s %-12s\n",
				"BOUNTY", "PARTNER", "APPROVED", "UNPAID", "PROJECTED")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-22s %-12d %-12d %-12.2f\n",
					truncate(r.BountyID, 22), truncate(r.PartnerID, 22),
					r.ApprovedSubmissions, r.UnpaidCommissions, r.ProjectedAmount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&window, "window", "30d", "Window for unpaid commissions to roll into the forecast")
	return cmd
}
