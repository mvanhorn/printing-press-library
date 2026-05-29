// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: per-rep per-stage deal rollup over local store.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelOwnerLoadCmd(flags *rootFlags) *cobra.Command {
	var pipeline string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "owner-load",
		Short:       "Open deals per rep per stage with $ totals, count, and oldest age",
		Long:        `Roll up open deals by owner and dealstage. Reads from the local SQLite mirror — run 'hubspot-pp-cli sync' first.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli owner-load --pipeline default`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			q := `
SELECT
  COALESCE(json_extract(d.data, '$.properties.hubspot_owner_id'), 'unowned') AS owner_id,
  COALESCE(json_extract(o.data, '$.firstName'), '') || ' ' || COALESCE(json_extract(o.data, '$.lastName'), '') AS owner_name,
  COALESCE(json_extract(d.data, '$.properties.dealstage'), 'unknown') AS stage,
  COUNT(*) AS deal_count,
  COALESCE(SUM(CAST(json_extract(d.data, '$.properties.amount') AS REAL)), 0) AS total_amount,
  MAX(CAST((julianday('now') - julianday(d.created_at)) AS INTEGER)) AS oldest_age_days
FROM hubspot_deals_crm d
LEFT JOIN hubspot_owners_crm o
  ON owner_id = o.id
WHERE COALESCE(d.archived, 0) = 0
  AND stage NOT IN ('closedwon', 'closedlost')
  AND (? = '' OR json_extract(d.data, '$.properties.pipeline') = ?)
GROUP BY owner_id, stage
ORDER BY owner_name, stage`
			rows, err := db.DB().QueryContext(cmd.Context(), q, pipeline, pipeline)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type row struct {
				OwnerID       string  `json:"owner_id"`
				OwnerName     string  `json:"owner_name"`
				Stage         string  `json:"stage"`
				Count         int64   `json:"count"`
				TotalAmount   float64 `json:"total_amount"`
				OldestAgeDays int64   `json:"oldest_age_days"`
			}
			items := []row{}
			for rows.Next() {
				var r row
				var name sql.NullString
				if err := rows.Scan(&r.OwnerID, &name, &r.Stage, &r.Count, &r.TotalAmount, &r.OldestAgeDays); err != nil {
					return err
				}
				r.OwnerName = nullStr(name)
				items = append(items, r)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, items)
			}
			headers := []string{"owner", "stage", "count", "total_amount", "oldest_age_days"}
			var tableRows [][]string
			lastOwner := ""
			var ownerTotal float64
			var ownerCount int64
			flush := func() {
				if lastOwner != "" {
					tableRows = append(tableRows, []string{
						lastOwner + " (total)", "", fmt.Sprintf("%d", ownerCount), formatAmount(ownerTotal), "",
					})
				}
			}
			for _, it := range items {
				owner := firstNonEmptyString(it.OwnerName, it.OwnerID)
				if owner != lastOwner {
					flush()
					lastOwner = owner
					ownerTotal = 0
					ownerCount = 0
				}
				ownerTotal += it.TotalAmount
				ownerCount += it.Count
				tableRows = append(tableRows, []string{
					owner, it.Stage, fmt.Sprintf("%d", it.Count),
					formatAmount(it.TotalAmount), fmt.Sprintf("%d", it.OldestAgeDays),
				})
			}
			flush()
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Restrict to a single pipeline id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
