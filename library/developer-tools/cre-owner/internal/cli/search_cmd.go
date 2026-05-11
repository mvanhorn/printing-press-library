package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

func newSearchUserCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath       string
		resourceType string
		market       string
		propertyType string
		limit        int
		stale        bool
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search across all synced data — parcels, owners, entities, listings",
		Long: `Unified full-text search across all resource types in the local SQLite store.
Supports FTS5 queries and field-specific filters via flags.

The --stale flag is a special mode that surfaces listings with 180+ days on market,
indicating potentially motivated sellers.`,
		Example: strings.Trim(`
  cre-owner-pp-cli search "downtown office" --json
  cre-owner-pp-cli search "123 Main St" --type parcels
  cre-owner-pp-cli search --market "Chicago" --type listings
  cre-owner-pp-cli search --stale --market "Dallas"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && resourceType == "" && market == "" && propertyType == "" && !stale {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Stale listing detection mode
			if stale {
				return searchStaleListings(cmd, db, market, limit, flags)
			}

			// FTS query when a search term is provided
			query := ""
			if len(args) > 0 {
				query = args[0]
			}

			// Field-specific SQL filtering
			if query == "" || resourceType != "" || market != "" || propertyType != "" {
				return searchWithFilters(cmd, db, query, resourceType, market, propertyType, limit, flags)
			}

			// Pure FTS search
			results, err := db.Search(query, limit)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			if len(results) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&resourceType, "type", "", "Filter by resource type (parcels, owners, entities, listings, sales, tax_records, contacts, signals)")
	cmd.Flags().StringVar(&market, "market", "", "Filter by market field")
	cmd.Flags().StringVar(&propertyType, "property-type", "", "Filter by property type")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().BoolVar(&stale, "stale", false, "Show listings with 180+ days on market (motivated seller detection)")
	return cmd
}

func searchWithFilters(cmd *cobra.Command, db *store.Store, query, resourceType, market, propertyType string, limit int, flags *rootFlags) error {
	var conditions []string
	var queryArgs []any

	if resourceType != "" {
		conditions = append(conditions, "r.resource_type = ?")
		queryArgs = append(queryArgs, resourceType)
	}
	if market != "" {
		conditions = append(conditions,
			`(LOWER(json_extract(r.data, '$.market')) LIKE LOWER(?) OR LOWER(json_extract(r.data, '$.submarket')) LIKE LOWER(?))`)
		queryArgs = append(queryArgs, "%"+market+"%", "%"+market+"%")
	}
	if propertyType != "" {
		conditions = append(conditions,
			`(LOWER(json_extract(r.data, '$.property_type')) LIKE LOWER(?) OR LOWER(json_extract(r.data, '$.propertyType')) LIKE LOWER(?))`)
		queryArgs = append(queryArgs, "%"+propertyType+"%", "%"+propertyType+"%")
	}

	var sql string
	if query != "" {
		// Combine FTS with field filters
		sql = `SELECT r.data FROM resources r
			JOIN resources_fts f ON r.id = f.id
			WHERE resources_fts MATCH ?`
		queryArgs = append([]any{query}, queryArgs...)
		if len(conditions) > 0 {
			sql += " AND " + strings.Join(conditions, " AND ")
		}
	} else {
		sql = `SELECT r.data FROM resources r WHERE ` + strings.Join(conditions, " AND ")
	}
	sql += " ORDER BY r.updated_at DESC LIMIT ?"
	queryArgs = append(queryArgs, limit)

	rows, err := db.DB().Query(sql, queryArgs...)
	if err != nil {
		return fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		results = append(results, jsonRaw(data))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading results: %w", err)
	}
	if results == nil {
		results = []any{}
	}
	return printJSONFiltered(cmd.OutOrStdout(), results, flags)
}

func searchStaleListings(cmd *cobra.Command, db *store.Store, market string, limit int, flags *rootFlags) error {
	sql := `SELECT data FROM resources
		WHERE resource_type = 'listings'
		AND CAST(json_extract(data, '$.days_on_market') AS INTEGER) > 180`
	var args []any

	if market != "" {
		sql += ` AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?) OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`
		args = append(args, "%"+market+"%", "%"+market+"%")
	}
	sql += ` ORDER BY CAST(json_extract(data, '$.days_on_market') AS INTEGER) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.DB().Query(sql, args...)
	if err != nil {
		return fmt.Errorf("stale listing query failed: %w", err)
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		results = append(results, jsonRaw(data))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading results: %w", err)
	}
	if results == nil {
		results = []any{}
	}
	return printJSONFiltered(cmd.OutOrStdout(), results, flags)
}

// jsonRaw wraps a raw JSON string so it marshals without double-encoding.
type jsonRaw string

func (j jsonRaw) MarshalJSON() ([]byte, error) {
	return []byte(j), nil
}
