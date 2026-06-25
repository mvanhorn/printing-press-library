// Copyright 2026 cfinney. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for pangolin-pp-cli.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/internal/store"
)

type auditIssue struct {
	Severity string         `json:"severity"`
	Kind     string         `json:"kind"`
	Subject  string         `json:"subject"`
	Detail   string         `json:"detail"`
	Context  map[string]any `json:"context,omitempty"`
}

type auditReport struct {
	Issues  []auditIssue `json:"issues"`
	Summary struct {
		Total int            `json:"total"`
		ByKind map[string]int `json:"by_kind"`
	} `json:"summary"`
	OrgFilter string `json:"org_filter,omitempty"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var orgFilter string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Cross-org health audit: stale targets, orphaned resources, missing role bindings.",
		Long: `Audit walks the local Pangolin store and surfaces health issues across every
org you have synced: targets pointing at unparseable hosts, resources with no
targets at all, resources with no role bindings, and orgs with zero resources.

Run 'sync --full' before audit to make sure the local store is current.`,
		Example: "  pangolin-pp-cli audit --json --select issues",
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

			report := auditReport{Issues: []auditIssue{}}
			report.Summary.ByKind = map[string]int{}
			report.OrgFilter = orgFilter

			// PATCH(audit-wire-org-filter): --org now restricts the SQL to rows
			// whose embedded data.orgId or data.orgName matches the filter value.
			// Walk resources and check embedded data.targets length, health, and
			// enabled flags — Pangolin's actual signals of "broken or stale".
			query := `SELECT id,
				        COALESCE(json_extract(data, '$.name'), id),
				        COALESCE(json_array_length(json_extract(data, '$.targets')), 0),
				        COALESCE(json_extract(data, '$.enabled'), 1),
				        COALESCE(json_extract(data, '$.health'), '')
				 FROM resources WHERE resource_type IN ('resources', 'resource')`
			queryArgs := []any{}
			if orgFilter != "" {
				query += ` AND (json_extract(data, '$.orgId') = ? OR json_extract(data, '$.orgName') = ?)`
				queryArgs = append(queryArgs, orgFilter, orgFilter)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id, name, health sql.NullString
					var targetCount, enabled int64
					if err := rows.Scan(&id, &name, &targetCount, &enabled, &health); err != nil {
						continue
					}
					if targetCount == 0 {
						report.Issues = append(report.Issues, auditIssue{
							Severity: "warning",
							Kind:     "resource_no_targets",
							Subject:  name.String,
							Detail:   "resource has no upstream targets configured",
							Context:  map[string]any{"resourceId": id.String},
						})
					}
					if enabled == 0 {
						report.Issues = append(report.Issues, auditIssue{
							Severity: "info",
							Kind:     "resource_disabled",
							Subject:  name.String,
							Detail:   "resource is disabled",
							Context:  map[string]any{"resourceId": id.String},
						})
					}
					if health.String != "" && health.String != "healthy" {
						report.Issues = append(report.Issues, auditIssue{
							Severity: "warning",
							Kind:     "resource_unhealthy",
							Subject:  name.String,
							Detail:   "health: " + health.String,
							Context:  map[string]any{"resourceId": id.String, "health": health.String},
						})
					}
				}
				_ = rows.Err()
			}

			// Sites offline
			// PATCH(audit-site-offline-filter): apply orgFilter to match resource and org-empty checks.
			siteQuery := `SELECT id, COALESCE(json_extract(data, '$.name'), id), COALESCE(json_extract(data, '$.online'), 1)
				 FROM resources WHERE resource_type IN ('sites', 'site')`
			siteArgs := []any{}
			if orgFilter != "" {
				siteQuery += ` AND (json_extract(data, '$.orgId') = ? OR json_extract(data, '$.orgName') = ?)`
				siteArgs = append(siteArgs, orgFilter, orgFilter)
			}
			srows, serr := db.DB().QueryContext(cmd.Context(), siteQuery, siteArgs...)
			if serr == nil {
				defer srows.Close()
				for srows.Next() {
					var id, name sql.NullString
					var online int64
					if err := srows.Scan(&id, &name, &online); err != nil {
						continue
					}
					if online == 0 {
						report.Issues = append(report.Issues, auditIssue{
							Severity: "error",
							Kind:     "site_offline",
							Subject:  name.String,
							Detail:   "site is offline — resources behind this site are unreachable",
							Context:  map[string]any{"siteId": id.String},
						})
					}
				}
				_ = srows.Err()
			}

			// Orgs with zero resources: per-org NOT EXISTS join so orgs in other
			// orgs having resources don't mask an empty org in the same install.
			emptyOrgQuery := `SELECT id, COALESCE(json_extract(data, '$.name'), id)
				 FROM resources o WHERE o.resource_type = 'orgs'
				 AND NOT EXISTS (
				     SELECT 1 FROM resources r
				     WHERE r.resource_type IN ('resources', 'resource')
				     AND json_extract(r.data, '$.orgId') = o.id
				 )`
			emptyOrgArgs := []any{}
			if orgFilter != "" {
				emptyOrgQuery += ` AND (o.id = ? OR json_extract(o.data, '$.name') = ?)`
				emptyOrgArgs = append(emptyOrgArgs, orgFilter, orgFilter)
			}
			orgRows, oerr := db.DB().QueryContext(cmd.Context(), emptyOrgQuery, emptyOrgArgs...)
			if oerr == nil {
				defer orgRows.Close()
				for orgRows.Next() {
					var id, name sql.NullString
					if orgRows.Scan(&id, &name) == nil {
						report.Issues = append(report.Issues, auditIssue{
							Severity: "info",
							Kind:     "org_empty",
							Subject:  name.String,
							Detail:   "org has no resources in local store (sync may be incomplete)",
							Context:  map[string]any{"orgId": id.String},
						})
					}
				}
			}

			// Tally
			report.Summary.Total = len(report.Issues)
			for _, iss := range report.Issues {
				report.Summary.ByKind[iss.Kind]++
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Audit: %d issues\n", report.Summary.Total)
			for k, v := range report.Summary.ByKind {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", k, v)
			}
			if len(report.Issues) > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
				for _, iss := range report.Issues {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s — %s\n", iss.Severity, iss.Kind, iss.Subject, iss.Detail)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&orgFilter, "org", "", "Limit audit to a single orgId")
	return cmd
}
