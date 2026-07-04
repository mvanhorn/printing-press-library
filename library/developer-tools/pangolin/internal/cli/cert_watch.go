// Copyright 2026 cfinney. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for pangolin-pp-cli.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/internal/store"
)

type certRow struct {
	Domain          string `json:"domain"`
	DomainID        string `json:"domainId,omitempty"`
	OrgID           string `json:"orgId,omitempty"`
	Status          string `json:"status,omitempty"`
	ExpiresAt       string `json:"expiresAt,omitempty"`
	DaysUntilExpiry int    `json:"daysUntilExpiry"`
}

func newCertWatchCmd(flags *rootFlags) *cobra.Command {
	var maxDays int
	cmd := &cobra.Command{
		Use:   "cert-watch",
		Short: "List certificates sorted by days-until-expiry across every synced org.",
		Long: `cert-watch walks every certificate in the local Pangolin store, computes
days-until-expiry, and returns the list sorted soonest-first. By default it
shows certs expiring within 30 days; use --days to widen or narrow the window
(--days 0 to show every cert regardless of expiry).

Run 'sync --full' first to make sure cert data is current.`,
		Example: "  pangolin-pp-cli cert-watch --days 30 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("pangolin-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, qerr := db.DB().QueryContext(cmd.Context(),
				`SELECT id,
				        COALESCE(json_extract(data, '$.domain'), ''),
				        COALESCE(json_extract(data, '$.domainId'), ''),
				        COALESCE(json_extract(data, '$.orgId'), ''),
				        COALESCE(json_extract(data, '$.status'), ''),
				        COALESCE(json_extract(data, '$.expiresAt'), json_extract(data, '$.expires_at'), '')
				 FROM resources WHERE resource_type IN ('certificate', 'certificates')`)
			if qerr != nil {
				return fmt.Errorf("querying certificates: %w", qerr)
			}
			defer rows.Close()

			out := []certRow{}
			now := time.Now().UTC()
			for rows.Next() {
				var id, domain, domainID, orgID, status, expires sql.NullString
				if err := rows.Scan(&id, &domain, &domainID, &orgID, &status, &expires); err != nil {
					continue
				}
				row := certRow{
					Domain:    domain.String,
					DomainID:  domainID.String,
					OrgID:     orgID.String,
					Status:    status.String,
					ExpiresAt: expires.String,
				}
				if row.Domain == "" {
					row.Domain = id.String
				}
				if expires.String != "" {
					if t, perr := time.Parse(time.RFC3339, expires.String); perr == nil {
						row.DaysUntilExpiry = int(t.Sub(now).Hours() / 24)
					} else {
						row.DaysUntilExpiry = 1<<31 - 1
					}
				} else {
					row.DaysUntilExpiry = 1<<31 - 1
				}
				if maxDays > 0 && row.DaysUntilExpiry > maxDays {
					continue
				}
				out = append(out, row)
			}
			_ = rows.Err()

			sort.Slice(out, func(i, j int) bool { return out[i].DaysUntilExpiry < out[j].DaysUntilExpiry })

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No certificates match window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %-6s  %s\n", "DOMAIN", "DAYS", "STATUS")
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %-6d  %s\n", r.Domain, r.DaysUntilExpiry, r.Status)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxDays, "days", 30, "Show certs expiring within N days (0 = show all)")
	return cmd
}
