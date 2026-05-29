// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: cross-object delta since a timestamp.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

type sinceItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Label     string `json:"name_or_label"`
	OwnerID   string `json:"owner"`
	UpdatedAt string `json:"updated_at"`
}

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var typesCSV string
	var owner string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "since [duration|timestamp]",
		Short:       "What changed across contacts, deals, engagements, companies since a given time",
		Long:        `Cross-object delta. Duration: Nh, Nd, Nw (hours, days, weeks). Or an RFC3339 timestamp. Reads from the local SQLite mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli since 1d --types deals,contacts`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseDurationOrTimestamp(args[0])
			if err != nil {
				return err
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

			types := []string{"deals", "contacts", "engagements", "companies"}
			if typesCSV != "" {
				types = splitCSV(typesCSV)
			}

			rows := []sinceItem{}

			for _, t := range types {
				switch t {
				case "engagements":
					for et, info := range engagementTables {
						q := fmt.Sprintf(`SELECT id,
  COALESCE(json_extract(data, '$.properties.hs_engagement_title'),
           json_extract(data, '$.properties.hs_task_subject'),
           json_extract(data, '$.properties.hs_meeting_title'),
           json_extract(data, '$.properties.hs_email_subject'),
           '') AS name,
  COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), '') AS owner_id,
  COALESCE(updated_at, '') AS updated_at
FROM %s
WHERE updated_at > ?
  AND (? = '' OR owner_id = ?)
ORDER BY updated_at DESC
LIMIT ?`, info.Table)
						if err := scanItems(db, cmd.Context(), q, et, ownerID, cutoff, limit, &rows); err != nil {
							return err
						}
					}
				case "deals":
					q := `SELECT id,
  COALESCE(json_extract(data, '$.properties.dealname'), '') AS name,
  COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), '') AS owner_id,
  COALESCE(updated_at, '') AS updated_at
FROM hubspot_deals_crm
WHERE updated_at > ?
  AND (? = '' OR owner_id = ?)
ORDER BY updated_at DESC
LIMIT ?`
					if err := scanItems(db, cmd.Context(), q, "deals", ownerID, cutoff, limit, &rows); err != nil {
						return err
					}
				case "contacts":
					q := `SELECT id,
  TRIM(COALESCE(json_extract(data, '$.properties.firstname'), '') || ' ' ||
       COALESCE(json_extract(data, '$.properties.lastname'), '')) AS name,
  COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), '') AS owner_id,
  COALESCE(updated_at, '') AS updated_at
FROM hubspot_contacts_crm
WHERE updated_at > ?
  AND (? = '' OR owner_id = ?)
ORDER BY updated_at DESC
LIMIT ?`
					if err := scanItems(db, cmd.Context(), q, "contacts", ownerID, cutoff, limit, &rows); err != nil {
						return err
					}
				case "companies":
					q := `SELECT id,
  COALESCE(json_extract(data, '$.properties.name'), '') AS name,
  COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), '') AS owner_id,
  COALESCE(updated_at, '') AS updated_at
FROM hubspot_companies_crm
WHERE updated_at > ?
  AND (? = '' OR owner_id = ?)
ORDER BY updated_at DESC
LIMIT ?`
					if err := scanItems(db, cmd.Context(), q, "companies", ownerID, cutoff, limit, &rows); err != nil {
						return err
					}
				default:
					fmt.Fprintf(cmd.ErrOrStderr(), "unknown --types value %q, skipping\n", t)
				}
			}

			sort.Slice(rows, func(i, j int) bool { return rows[i].UpdatedAt > rows[j].UpdatedAt })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"since":   cutoff,
					"results": rows,
				})
			}
			headers := []string{"type", "id", "name_or_label", "owner", "updated_at"}
			outRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				outRows = append(outRows, []string{r.Type, r.ID, r.Label, r.OwnerID, r.UpdatedAt})
			}
			return flags.printTable(cmd, headers, outRows)
		},
	}
	cmd.Flags().StringVar(&typesCSV, "types", "", "Limit to types (deals,contacts,engagements,companies)")
	cmd.Flags().StringVar(&owner, "owner", "", "Filter by owner id, email, or 'me'")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max rows per type and total")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// scanItems runs the parameterized SELECT and appends rows.
func scanItems(db *store.Store, ctx context.Context, q, kind, ownerID, cutoff string, limit int, out *[]sinceItem) error {
	rows, err := db.DB().QueryContext(ctx, q, cutoff, ownerID, ownerID, limit)
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, owner, updated sql.NullString
		if err := rows.Scan(&id, &name, &owner, &updated); err != nil {
			return err
		}
		*out = append(*out, sinceItem{
			Type:      kind,
			ID:        nullStr(id),
			Label:     strings.TrimSpace(nullStr(name)),
			OwnerID:   nullStr(owner),
			UpdatedAt: nullStr(updated),
		})
	}
	return nil
}
