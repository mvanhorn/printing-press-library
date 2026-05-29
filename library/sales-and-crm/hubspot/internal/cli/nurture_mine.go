// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: cold-contact-with-open-deal query.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelNurtureMineCmd(flags *rootFlags) *cobra.Command {
	var owner string
	var staleDays int
	var stageUnder string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "nurture-mine",
		Short:       "Cold contacts owned by you that still have open deals",
		Long:        `Surface contacts who haven't been contacted in N days but still have an open deal. Reads from the local SQLite mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli nurture-mine --owner me --stale-days 14`,
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
			ownerID, err := resolveOwnerArg(db, owner)
			if err != nil {
				return err
			}

			rows, source, err := queryNurtureMine(cmd, db, ownerID, staleDays, stageUnder, limit)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"data_source": source,
					"results":     rows,
				})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "data_source: %s\n", source)
			headers := []string{"contact_id", "name", "email", "last_contacted", "days_stale", "deal_count", "latest_deal_stage"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.ContactID, r.Name, r.Email, r.LastContacted,
					fmt.Sprintf("%d", r.DaysStale),
					fmt.Sprintf("%d", r.DealCount),
					r.LatestDealStage,
				})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "me", "Filter by owner id, email, or 'me'")
	cmd.Flags().IntVar(&staleDays, "stale-days", 14, "Minimum days since last contact")
	cmd.Flags().StringVar(&stageUnder, "stage-under", "closedwon", "Treat deals AT or PAST this stage id as closed (excluded)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type nurtureMineRow struct {
	ContactID       string  `json:"contact_id"`
	Name            string  `json:"name"`
	Email           string  `json:"email"`
	LastContacted   string  `json:"last_contacted"`
	DaysStale       int64   `json:"days_stale"`
	DealCount       int64   `json:"deal_count"`
	LatestDealStage string  `json:"latest_deal_stage"`
	TopDealAmount   float64 `json:"top_deal_amount"`
}

func queryNurtureMine(cmd *cobra.Command, db *store.Store, ownerID string, staleDays int, stageUnder string, limit int) ([]nurtureMineRow, string, error) {
	var assocCount int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM hubspot_associations`).Scan(&assocCount)
	source := "local"

	if assocCount > 0 {
		q := `
SELECT * FROM (
  SELECT c.id,
    TRIM(COALESCE(json_extract(c.data, '$.properties.firstname'), '') || ' ' ||
         COALESCE(json_extract(c.data, '$.properties.lastname'), '')) AS name,
    COALESCE(json_extract(c.data, '$.properties.email'), '') AS email,
    json_extract(c.data, '$.properties.notes_last_contacted') AS last_contacted,
    CAST((julianday('now') - julianday(COALESCE(
      json_extract(c.data, '$.properties.notes_last_contacted'),
      json_extract(c.data, '$.properties.notes_last_updated'),
      json_extract(c.data, '$.properties.hs_lastmodifieddate'),
      c.created_at
    ))) AS INTEGER) AS days_stale,
    (SELECT COUNT(*) FROM hubspot_associations a
     JOIN hubspot_deals_crm d ON a.from_type = 'contacts' AND a.to_type = 'deals'
     WHERE a.from_id = c.id AND a.to_id = d.id
       AND COALESCE(d.archived, 0) = 0
       AND json_extract(d.data, '$.properties.dealstage') NOT IN ('closedwon', 'closedlost', ?)
    ) AS deal_count,
    (SELECT json_extract(d.data, '$.properties.dealstage') FROM hubspot_associations a
     JOIN hubspot_deals_crm d ON a.from_type = 'contacts' AND a.to_type = 'deals'
     WHERE a.from_id = c.id AND a.to_id = d.id
       AND COALESCE(d.archived, 0) = 0
     ORDER BY d.updated_at DESC LIMIT 1
    ) AS latest_deal_stage,
    COALESCE((SELECT MAX(CAST(json_extract(d.data, '$.properties.amount') AS REAL)) FROM hubspot_associations a
     JOIN hubspot_deals_crm d ON a.from_type = 'contacts' AND a.to_type = 'deals'
     WHERE a.from_id = c.id AND a.to_id = d.id
       AND COALESCE(d.archived, 0) = 0
    ), 0) AS top_deal_amount
  FROM hubspot_contacts_crm c
  WHERE COALESCE(c.archived, 0) = 0
    AND (? = '' OR json_extract(c.data, '$.properties.hubspot_owner_id') = ?)
)
WHERE days_stale >= ? AND deal_count > 0
ORDER BY days_stale DESC
LIMIT ?`
		rows, err := db.DB().QueryContext(cmd.Context(), q, stageUnder, ownerID, ownerID, staleDays, limit)
		if err != nil {
			return nil, source, fmt.Errorf("query: %w", err)
		}
		defer rows.Close()
		items := []nurtureMineRow{}
		for rows.Next() {
			var r nurtureMineRow
			var lc, stage sql.NullString
			var amt sql.NullFloat64
			if err := rows.Scan(&r.ContactID, &r.Name, &r.Email, &lc, &r.DaysStale, &r.DealCount, &stage, &amt); err != nil {
				return nil, source, err
			}
			r.LastContacted = nullStr(lc)
			r.LatestDealStage = nullStr(stage)
			r.TopDealAmount = nullF(amt)
			items = append(items, r)
		}
		return items, source, nil
	}

	source = "local-no-associations"
	q := `
SELECT * FROM (
  SELECT c.id,
    TRIM(COALESCE(json_extract(c.data, '$.properties.firstname'), '') || ' ' ||
         COALESCE(json_extract(c.data, '$.properties.lastname'), '')) AS name,
    COALESCE(json_extract(c.data, '$.properties.email'), '') AS email,
    json_extract(c.data, '$.properties.notes_last_contacted') AS last_contacted,
    CAST((julianday('now') - julianday(COALESCE(
      json_extract(c.data, '$.properties.notes_last_contacted'),
      json_extract(c.data, '$.properties.notes_last_updated'),
      json_extract(c.data, '$.properties.hs_lastmodifieddate'),
      c.created_at
    ))) AS INTEGER) AS days_stale
  FROM hubspot_contacts_crm c
  WHERE COALESCE(c.archived, 0) = 0
    AND (? = '' OR json_extract(c.data, '$.properties.hubspot_owner_id') = ?)
)
WHERE days_stale >= ?
ORDER BY days_stale DESC
LIMIT ?`
	rows, err := db.DB().QueryContext(cmd.Context(), q, ownerID, ownerID, staleDays, limit)
	if err != nil {
		return nil, source, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	items := []nurtureMineRow{}
	for rows.Next() {
		var r nurtureMineRow
		var lc sql.NullString
		if err := rows.Scan(&r.ContactID, &r.Name, &r.Email, &lc, &r.DaysStale); err != nil {
			return nil, source, err
		}
		r.LastContacted = nullStr(lc)
		items = append(items, r)
	}
	return items, source, nil
}
