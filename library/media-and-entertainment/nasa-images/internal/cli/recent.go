// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRecentCmd(flags *rootFlags) *cobra.Command {
	var (
		q, mediaType, center, sortOrder, dbPath string
		limit                                   int
	)
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "FTS5 search over the local mirror, sorted by date_created descending",
		Long: `Search the locally-mirrored assets (populated by 'mirror search' or 'mirror album')
with FTS5 over title + description + description_508 + keywords, sorted
chronologically — the upstream API has no chronological sort (open issue
nasa/api-docs#187, no maintainer response in 3+ years).

Requires a populated local store. Run 'mirror search --q <topic>' first.`,
		Example:     "  nasa-images-pp-cli recent --q \"perseverance\" --sort date-desc --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local mirror")
				return nil
			}
			ctx := cmd.Context()
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			order := "DESC"
			switch strings.ToLower(sortOrder) {
			case "date-desc", "desc", "":
				order = "DESC"
			case "date-asc", "asc":
				order = "ASC"
			default:
				return fmt.Errorf("invalid --sort %q: must be date-desc or date-asc", sortOrder)
			}

			// Build the SQL. FTS5 search via resources_fts joined on rowid,
			// scoped to resource_type='asset', with optional media_type and
			// center filters, ordered by date_created (json_extract).
			var conds []string
			var argv []any
			conds = append(conds, "r.resource_type = 'asset'")
			if strings.TrimSpace(q) != "" {
				// resources_fts rowid is derived from (resource_type,id) via
				// ftsRowID(...) — not the same as resources.rowid. Join via
				// (id, resource_type) instead so FTS hits match the correct
				// canonical row. quoteFTS handles FTS5 syntax characters
				// (hyphen, colon, etc.) so apollo-11 isn't parsed as
				// "apollo NOT 11".
				conds = append(conds, "r.id IN (SELECT id FROM resources_fts WHERE resource_type = 'asset' AND resources_fts MATCH ?)")
				argv = append(argv, quoteFTS(q))
			}
			if mediaType != "" {
				conds = append(conds, "json_extract(r.data, '$.media_type') = ?")
				argv = append(argv, mediaType)
			}
			if center != "" {
				conds = append(conds, "json_extract(r.data, '$.center') = ?")
				argv = append(argv, center)
			}
			where := strings.Join(conds, " AND ")
			if limit <= 0 {
				limit = 25
			}
			query := fmt.Sprintf(`
				SELECT r.id, r.data
				FROM resources r
				WHERE %s
				ORDER BY COALESCE(json_extract(r.data, '$.date_created'), '') %s
				LIMIT ?
			`, where, order)
			argv = append(argv, limit)

			rows, err := s.DB().QueryContext(ctx, query, argv...)
			if err != nil {
				return fmt.Errorf("querying local mirror: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var id sql.NullString
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				var asset map[string]any
				if err := json.Unmarshal(data, &asset); err != nil {
					continue
				}
				results = append(results, asset)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No matching assets in the local mirror. Run 'nasa-images-pp-cli mirror search --q <topic>' first.")
				return flags.printJSON(cmd, []any{})
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&q, "q", "", "FTS5 query terms (matched against title, description, keywords)")
	cmd.Flags().StringVar(&mediaType, "media-type", "", "Filter by media type (image, video, audio)")
	cmd.Flags().StringVar(&center, "center", "", "Filter by NASA center code (e.g. JPL, JSC)")
	cmd.Flags().StringVar(&sortOrder, "sort", "date-desc", "Sort order: date-desc (default) or date-asc")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}
