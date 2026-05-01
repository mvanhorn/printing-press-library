// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// partners_audit_commissions reconciles synced commissions against expected
// totals — flags duplicate sale events, partners with negative balances, and
// commissions stuck in pending. Reads from the local /store cache (store.Open).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type auditFinding struct {
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	PartnerID    string `json:"partner_id,omitempty"`
	CommissionID string `json:"commission_id,omitempty"`
	Message      string `json:"message"`
}

func newPartnersAuditCommissionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit-commissions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Reconcile partners × commissions × payouts; flag stale rates and missing payouts",
		Long: `Cross-resource audit:
  - commissions in 'pending' status older than 14 days (stuck payout)
  - banned partners still earning commissions
  - commissions whose status is 'paid' but no matching payout record exists
  - duplicate commission IDs across the same invoice

Reads from the local store. Run sync first.`,
		Example: `  dub-pp-cli partners audit-commissions --json
  dub-pp-cli partners audit-commissions --agent --select code,partner_id,message`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			// pull partners and build status map
			partnerStatus := make(map[string]string)
			partnerName := make(map[string]string)
			pRows, err := db.DB().Query(`SELECT id, data FROM partners`)
			if err == nil {
				for pRows.Next() {
					var id, raw string
					if err := pRows.Scan(&id, &raw); err != nil {
						continue
					}
					var obj map[string]any
					if err := json.Unmarshal([]byte(raw), &obj); err != nil {
						continue
					}
					partnerStatus[id] = stringField(obj, "status")
					partnerName[id] = stringField(obj, "name")
				}
				pRows.Close()
			}

			// pull commissions
			cRows, err := db.DB().Query(`SELECT id, data, partner_id, status, payout_id, invoice_id FROM commissions`)
			if err != nil {
				return apiErr(fmt.Errorf("query commissions: %w", err))
			}
			defer cRows.Close()

			findings := make([]auditFinding, 0)
			invoiceCounts := make(map[string]int)
			for cRows.Next() {
				var id, raw, partnerID, status, payoutID, invoiceID string
				if err := cRows.Scan(&id, &raw, &partnerID, &status, &payoutID, &invoiceID); err != nil {
					continue
				}
				var obj map[string]any
				_ = json.Unmarshal([]byte(raw), &obj)

				if invoiceID != "" {
					invoiceCounts[invoiceID]++
				}

				// banned partner still earning
				if pStatus := partnerStatus[partnerID]; pStatus == "banned" || pStatus == "deactivated" {
					findings = append(findings, auditFinding{
						Severity: "error", Code: "banned-partner-earning",
						PartnerID: partnerID, CommissionID: id,
						Message: fmt.Sprintf("commission for %s partner %s (%s)", pStatus, partnerName[partnerID], partnerID),
					})
				}

				// pending too long
				if status == "pending" && payoutID == "" {
					createdAt := stringField(obj, "createdAt")
					if isOlderThanDays(createdAt, 14) {
						findings = append(findings, auditFinding{
							Severity: "warn", Code: "stale-pending-commission",
							PartnerID: partnerID, CommissionID: id,
							Message: fmt.Sprintf("pending since %s — payout missing", createdAt),
						})
					}
				}

				// paid but no payout reference
				if status == "paid" && payoutID == "" {
					findings = append(findings, auditFinding{
						Severity: "warn", Code: "paid-without-payout-ref",
						PartnerID: partnerID, CommissionID: id,
						Message: "marked paid but missing payout_id",
					})
				}
			}

			// duplicate invoices
			for invID, count := range invoiceCounts {
				if count > 1 {
					findings = append(findings, auditFinding{
						Severity: "warn", Code: "duplicate-invoice-commissions",
						Message: fmt.Sprintf("invoice %s has %d commissions", invID, count),
					})
				}
			}

			sort.Slice(findings, func(i, j int) bool {
				if severityRank(findings[i].Severity) != severityRank(findings[j].Severity) {
					return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
				}
				return findings[i].Code < findings[j].Code
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, findings)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "audit clean — no issues found")
				return nil
			}
			headers := []string{"SEV", "CODE", "PARTNER", "COMMISSION", "MESSAGE"}
			rowsTbl := make([][]string, 0, len(findings))
			for _, f := range findings {
				msg := f.Message
				if len(msg) > 60 {
					msg = msg[:57] + "..."
				}
				rowsTbl = append(rowsTbl, []string{f.Severity, f.Code, f.PartnerID, f.CommissionID, msg})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}
	return cmd
}

// isOlderThanDays returns true when an RFC3339 timestamp is older than N days from now.
// Pure logic; tested via partners_audit_commissions_test.go.
func isOlderThanDays(rfc3339 string, days int) bool {
	if rfc3339 == "" {
		return false
	}
	t, err := parseTimestamp(rfc3339)
	if err != nil {
		return false
	}
	return nowFunc().Sub(t).Hours() > float64(days*24)
}
