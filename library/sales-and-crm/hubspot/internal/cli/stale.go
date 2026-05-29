// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: surface stale contacts/deals from local store.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

// filterDescription is the help-text for every novel command that exposes
// HubSpot's bulk-operations --filter grammar. Kept identical across commands
// so operator muscle memory transfers; reading it once means knowing it
// everywhere.
const filterDescription = "HubSpot-style filter: bare 'field' (HAS), '!field' (NOT_HAS), 'field=value' (EQ), 'field~token' (CONTAINS_TOKEN). AND within one --filter, OR across multiple --filter flags."

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Find stale contacts or deals (no engagement in N days)",
		Long:        `Surface contacts or deals with no engagement in the last N days. Reads from the local SQLite mirror — run 'hubspot-pp-cli sync' first.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newStaleContactsCmd(flags))
	cmd.AddCommand(newStaleDealsCmd(flags))
	return cmd
}

func newStaleContactsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var owner string
	var limit int
	var dbPath string
	var filterFlags []string
	var filterDebug bool

	cmd := &cobra.Command{
		Use:         "contacts",
		Short:       "Contacts with no engagement in N days",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  hubspot-pp-cli stale contacts --days 30 --owner me
  hubspot-pp-cli stale contacts --days 30 --filter 'lifecyclestage=customer !do_not_call'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			expr, err := cliutil.ParseFilters(filterFlags)
			if err != nil {
				return err
			}
			if filterDebug {
				fmt.Fprint(cmd.ErrOrStderr(), expr.DebugString())
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

			// When --filter is in play we hoist the post-filter LIMIT into
			// memory so the filter sees the full candidate set: rows
			// rejected by --filter shouldn't eat into the operator's --limit
			// budget. The pre-filter cap of 5x --limit keeps memory bounded
			// when --filter is empty (the cap never triggers in the
			// no-filter case because filterMatches==len).
			sqlLimit := limit
			if !expr.IsEmpty() && limit > 0 {
				sqlLimit = limit * 20
			}
			q := `
SELECT * FROM (
  SELECT id,
    COALESCE(json_extract(data, '$.properties.firstname'), '') || ' ' ||
    COALESCE(json_extract(data, '$.properties.lastname'), '') AS name,
    json_extract(data, '$.properties.email') AS email,
    COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), '') AS owner_id,
    json_extract(data, '$.properties.notes_last_contacted') AS last_contacted,
    CAST(
      (julianday('now') - julianday(COALESCE(
        json_extract(data, '$.properties.notes_last_contacted'),
        json_extract(data, '$.properties.notes_last_updated'),
        json_extract(data, '$.properties.hs_lastmodifieddate'),
        created_at
      ))) AS INTEGER
    ) AS idle_days,
    data AS raw_data
  FROM hubspot_contacts_crm
  WHERE COALESCE(archived, 0) = 0
    AND (? = '' OR owner_id = ?)
)
WHERE idle_days >= ?
ORDER BY idle_days DESC
LIMIT ?`
			rows, err := db.DB().QueryContext(cmd.Context(), q, ownerID, ownerID, days, sqlLimit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type out struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				Email         string `json:"email"`
				OwnerID       string `json:"owner_id"`
				LastContacted string `json:"last_contacted"`
				IdleDays      int64  `json:"idle_days"`
			}
			items := []out{}
			for rows.Next() {
				var o out
				var lc, em sql.NullString
				var raw sql.NullString
				if err := rows.Scan(&o.ID, &o.Name, &em, &o.OwnerID, &lc, &o.IdleDays, &raw); err != nil {
					return err
				}
				o.Email = nullStr(em)
				o.LastContacted = nullStr(lc)
				if !expr.IsEmpty() {
					row := extractPropertiesRow(raw.String, expr.FieldsReferenced())
					if !expr.Match(row) {
						continue
					}
				}
				items = append(items, o)
				if limit > 0 && len(items) >= limit {
					break
				}
			}

			if flags.asJSON {
				return flags.printJSON(cmd, items)
			}
			headers := []string{"id", "name", "email", "owner_id", "idle_days", "last_contacted"}
			tableRows := make([][]string, 0, len(items))
			for _, it := range items {
				tableRows = append(tableRows, []string{
					it.ID, it.Name, it.Email, it.OwnerID, fmt.Sprintf("%d", it.IdleDays), it.LastContacted,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Minimum idle days")
	cmd.Flags().StringVar(&owner, "owner", "", "Filter by owner id, email, or 'me'")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringSliceVar(&filterFlags, "filter", nil, filterDescription)
	cmd.Flags().BoolVar(&filterDebug, "filter-debug", false, "Print parsed --filter expression to stderr before applying it")
	return cmd
}

func newStaleDealsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var owner string
	var stage string
	var limit int
	var dbPath string
	var filterFlags []string
	var filterDebug bool

	cmd := &cobra.Command{
		Use:         "deals",
		Short:       "Open deals with no engagement in N days",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  hubspot-pp-cli stale deals --days 21 --owner me
  hubspot-pp-cli stale deals --days 30 --filter 'pipeline=default amount~1000'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			expr, err := cliutil.ParseFilters(filterFlags)
			if err != nil {
				return err
			}
			if filterDebug {
				fmt.Fprint(cmd.ErrOrStderr(), expr.DebugString())
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

			sqlLimit := limit
			if !expr.IsEmpty() && limit > 0 {
				sqlLimit = limit * 20
			}
			q := `
SELECT * FROM (
  SELECT id,
    COALESCE(json_extract(data, '$.properties.dealname'), '') AS name,
    CAST(json_extract(data, '$.properties.amount') AS REAL) AS amount,
    COALESCE(json_extract(data, '$.properties.dealstage'), '') AS stage,
    COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), '') AS owner_id,
    json_extract(data, '$.properties.notes_last_contacted') AS last_contacted,
    CAST(
      (julianday('now') - julianday(COALESCE(
        json_extract(data, '$.properties.notes_last_contacted'),
        json_extract(data, '$.properties.notes_last_updated'),
        json_extract(data, '$.properties.hs_lastmodifieddate'),
        created_at
      ))) AS INTEGER
    ) AS idle_days,
    data AS raw_data
  FROM hubspot_deals_crm
  WHERE COALESCE(archived, 0) = 0
    AND (? = '' OR owner_id = ?)
    AND (? = '' OR COALESCE(json_extract(data, '$.properties.dealstage'), '') = ?)
    AND COALESCE(json_extract(data, '$.properties.dealstage'), '') NOT IN ('closedwon', 'closedlost')
)
WHERE idle_days >= ?
ORDER BY idle_days DESC
LIMIT ?`
			rows, err := db.DB().QueryContext(cmd.Context(), q, ownerID, ownerID, stage, stage, days, sqlLimit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type out struct {
				ID            string  `json:"id"`
				Name          string  `json:"name"`
				Amount        float64 `json:"amount"`
				Stage         string  `json:"stage"`
				OwnerID       string  `json:"owner_id"`
				LastContacted string  `json:"last_contacted"`
				IdleDays      int64   `json:"idle_days"`
			}
			items := []out{}
			for rows.Next() {
				var o out
				var amt sql.NullFloat64
				var lc sql.NullString
				var raw sql.NullString
				if err := rows.Scan(&o.ID, &o.Name, &amt, &o.Stage, &o.OwnerID, &lc, &o.IdleDays, &raw); err != nil {
					return err
				}
				o.Amount = nullF(amt)
				o.LastContacted = nullStr(lc)
				if !expr.IsEmpty() {
					row := extractPropertiesRow(raw.String, expr.FieldsReferenced())
					if !expr.Match(row) {
						continue
					}
				}
				items = append(items, o)
				if limit > 0 && len(items) >= limit {
					break
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, items)
			}
			headers := []string{"id", "name", "amount", "stage", "owner_id", "idle_days", "last_contacted"}
			tableRows := make([][]string, 0, len(items))
			for _, it := range items {
				tableRows = append(tableRows, []string{
					it.ID, it.Name, formatAmount(it.Amount), it.Stage, it.OwnerID,
					fmt.Sprintf("%d", it.IdleDays), it.LastContacted,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().IntVar(&days, "days", 21, "Minimum idle days")
	cmd.Flags().StringVar(&owner, "owner", "", "Filter by owner id, email, or 'me'")
	cmd.Flags().StringVar(&stage, "stage", "", "Filter by dealstage id")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringSliceVar(&filterFlags, "filter", nil, filterDescription)
	cmd.Flags().BoolVar(&filterDebug, "filter-debug", false, "Print parsed --filter expression to stderr before applying it")
	return cmd
}
