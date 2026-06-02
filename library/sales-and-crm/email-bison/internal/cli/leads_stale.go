// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: leads stale.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type staleLeadRow struct {
	LeadID     string `json:"lead_id"`
	Email      string `json:"email"`
	Company    string `json:"company"`
	LastSentAt string `json:"last_sent_at"`
}

func newNovelLeadsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find leads stuck mid-sequence: in a live campaign, already emailed, no reply, and no recent send.",
		Long: `Joins leads, their sent emails, and their replies in the local store to find leads
that were emailed more than N days ago, never replied, and have had no send since,
i.e. stuck mid-sequence. Sync first with 'email-bison-pp-cli sync'.`,
		Example:     "  email-bison-pp-cli leads stale --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openNovelStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			cutoffExpr := fmt.Sprintf("date('now', '-%d days')", flagDays)
			q := `
SELECT
  CAST(COALESCE(json_extract(l.data,'$.id'), l.id) AS TEXT)                       AS lead_id,
  COALESCE(json_extract(l.data,'$.email'), '')                                    AS email,
  COALESCE(json_extract(l.data,'$.company'), '')                                  AS company,
  (SELECT MAX(date(COALESCE(json_extract(se.data,'$.sent_at'),
                            json_extract(se.data,'$.created_at'), se.synced_at)))
     FROM sent_emails se
     WHERE CAST(se.leads_id AS TEXT) = CAST(COALESCE(json_extract(l.data,'$.id'), l.id) AS TEXT)) AS last_sent
FROM resources l
WHERE l.resource_type = 'leads'
  AND EXISTS (SELECT 1 FROM sent_emails se
                WHERE CAST(se.leads_id AS TEXT) = CAST(COALESCE(json_extract(l.data,'$.id'), l.id) AS TEXT))
  AND NOT EXISTS (SELECT 1 FROM leads_replies lr
                WHERE CAST(lr.leads_id AS TEXT) = CAST(COALESCE(json_extract(l.data,'$.id'), l.id) AS TEXT))
  AND (SELECT MAX(date(COALESCE(json_extract(se.data,'$.sent_at'),
                                json_extract(se.data,'$.created_at'), se.synced_at)))
         FROM sent_emails se
         WHERE CAST(se.leads_id AS TEXT) = CAST(COALESCE(json_extract(l.data,'$.id'), l.id) AS TEXT))
      <= ` + cutoffExpr + `
ORDER BY last_sent ASC;`
			rows, err := db.DB().QueryContext(cmd.Context(), q)
			if err != nil {
				return fmt.Errorf("querying stale leads: %w", err)
			}
			defer rows.Close()

			results := []staleLeadRow{}
			for rows.Next() {
				var r staleLeadRow
				var email, company, lastSent sql.NullString
				if err := rows.Scan(&r.LeadID, &email, &company, &lastSent); err != nil {
					return fmt.Errorf("scanning stale lead: %w", err)
				}
				r.Email = email.String
				r.Company = company.String
				r.LastSentAt = lastSent.String
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating stale leads: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No leads stale beyond %d days. Run 'email-bison-pp-cli sync' first if the store is empty.\n", flagDays)
				return nil
			}
			fmt.Fprintln(out, "LEAD\tEMAIL\tCOMPANY\tLAST SENT")
			for _, r := range results {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", r.LeadID, r.Email, r.Company, r.LastSentAt)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Flag leads with no send and no reply for at least this many days.")
	return cmd
}
