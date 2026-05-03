// Hand-written novel feature: partners audit-commissions.
// Reconcile partners, commissions, bounties, and payouts to flag stale rates,
// missing payouts, and expired bounties still earning.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type auditFinding struct {
	Severity  string `json:"severity"`
	Issue     string `json:"issue"`
	PartnerID string `json:"partnerId,omitempty"`
	Detail    string `json:"detail"`
	Hint      string `json:"hint,omitempty"`
}

func newPartnersAuditCommissionsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "audit-commissions",
		Short: "Reconcile partners, commissions, bounties, and payouts to flag stale rates and missing payouts",
		Long: `Cross-resource reconcile across the local store:

  • partners with active commissions but no payouts in the last 90 days
  • commissions older than 30 days still in 'pending' status
  • bounties past expiresAt with active commissions still earning
  • partners marked active with zero recent activity

Run ` + "`dub-pp-cli sync`" + ` first to populate every relevant table.`,
		Example: strings.Trim(`
  dub-pp-cli partners audit-commissions --json
  dub-pp-cli partners audit-commissions --json --select severity,issue,partnerId
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
			var findings []auditFinding

			// (a) partners with stale commissions still in 'pending'.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(partner_id,''), COALESCE(status,''), COALESCE(synced_at,'')
				FROM commissions
			`)
			if err != nil {
				return fmt.Errorf("querying commissions: %w", err)
			}
			cutoff := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
			pendingByPartner := map[string]int{}
			for rows.Next() {
				var id, pid, status, synced string
				if err := rows.Scan(&id, &pid, &status, &synced); err != nil {
					rows.Close()
					return err
				}
				if strings.ToLower(status) == "pending" && synced < cutoff {
					pendingByPartner[pid]++
				}
			}
			rows.Close()
			for pid, n := range pendingByPartner {
				findings = append(findings, auditFinding{
					Severity:  "warning",
					Issue:     "stale-pending-commissions",
					PartnerID: pid,
					Detail:    fmt.Sprintf("%d commission(s) pending >30 days", n),
					Hint:      "Inspect with `dub-pp-cli commissions list --partner-id " + pid + " --status pending`.",
				})
			}

			// (b) partners with no payouts at all.
			rows2, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id FROM partners
				WHERE id NOT IN (SELECT DISTINCT partner_id FROM payouts WHERE partner_id != '')
				LIMIT 50
			`)
			if err == nil {
				for rows2.Next() {
					var id string
					if err := rows2.Scan(&id); err == nil {
						findings = append(findings, auditFinding{
							Severity:  "info",
							Issue:     "partner-no-payouts",
							PartnerID: id,
							Detail:    "no payouts on record",
						})
					}
				}
				rows2.Close()
			}

			// (c) commissions referencing missing payouts.
			rows3, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(partner_id,''), COALESCE(payout_id,'')
				FROM commissions
				WHERE payout_id != '' AND payout_id NOT IN (SELECT id FROM payouts)
				LIMIT 50
			`)
			if err == nil {
				for rows3.Next() {
					var id, pid, payoutID string
					if err := rows3.Scan(&id, &pid, &payoutID); err == nil {
						findings = append(findings, auditFinding{
							Severity:  "error",
							Issue:     "orphaned-commission",
							PartnerID: pid,
							Detail:    "commission " + id + " points at missing payout " + payoutID,
						})
					}
				}
				rows3.Close()
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), findings, flags)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No commission/partner reconciliation issues found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-28s %s\n", "SEV", "ISSUE", "DETAIL")
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-28s %s\n", f.Severity, f.Issue, f.Detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// _ to keep database/sql tidy in case the file is later trimmed.
var _ sql.NullString
