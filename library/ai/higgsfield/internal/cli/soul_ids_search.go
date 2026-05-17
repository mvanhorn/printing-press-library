// Copyright 2026 higgsfield-ai. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for higgsfield-pp-cli (Phase 3 transcendence).

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/higgsfield/internal/store"
)

type soulSearchResult struct {
	SoulID     string `json:"soul_id"`
	Name       string `json:"name"`
	Status     string `json:"status,omitempty"`
	UsageCount int    `json:"usage_count"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

func newSoulIdsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across Soul ID names plus the prompts each Soul ID has been used in",
		Long: `Searches the synced soul_ids table by name and joins into past generations
that referenced each Soul ID. Ranks results by last-used. Requires a prior
'higgsfield-pp-cli sync' so the local store is populated.`,
		Example: strings.Trim(`
  higgsfield-pp-cli soul-ids search "riggs"
  higgsfield-pp-cli soul-ids search "studio" --json --limit 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("higgsfield-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			query := strings.TrimSpace(args[0])
			pattern := "%" + strings.ToLower(query) + "%"

			// Match Soul IDs by name (case-insensitive LIKE), then count usage in
			// generations.data via JSON_EXTRACT of soul_id. SQLite's json_extract
			// is available in the sqlite3 driver via the standard JSON1 build.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(s.data, '$.soul_id'), json_extract(s.data, '$.id')) AS soul_id,
					COALESCE(json_extract(s.data, '$.name'), '') AS name,
					COALESCE(json_extract(s.data, '$.status'), '') AS status,
					COALESCE(json_extract(s.data, '$.last_used_at'), '') AS last_used_at,
					(
						SELECT COUNT(*) FROM resources g
						WHERE g.resource_type IN ('generations')
						  AND json_extract(g.data, '$.soul_id') = COALESCE(json_extract(s.data, '$.soul_id'), json_extract(s.data, '$.id'))
					) AS usage_count
				FROM resources s
				WHERE s.resource_type IN ('soul_ids', 'soul-ids')
				  AND lower(COALESCE(json_extract(s.data, '$.name'), '')) LIKE ?
				ORDER BY usage_count DESC, last_used_at DESC
				LIMIT ?`, pattern, limit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			var results []soulSearchResult
			for rows.Next() {
				var r soulSearchResult
				var lastUsed sql.NullString
				if err := rows.Scan(&r.SoulID, &r.Name, &r.Status, &lastUsed, &r.UsageCount); err != nil {
					return err
				}
				if lastUsed.Valid {
					r.LastUsedAt = lastUsed.String
				}
				results = append(results, r)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No Soul IDs match %q. Run `higgsfield-pp-cli sync` if you have not synced recently.\n", query)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-36s %-24s %-6s %s\n", "SOUL_ID", "NAME", "USED", "LAST_USED")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-36s %-24s %-6d %s\n", r.SoulID, truncate(r.Name, 22), r.UsageCount, r.LastUsedAt)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override path to the local SQLite database")
	return cmd
}

type soulUsageRow struct {
	RequestID string `json:"request_id"`
	Model     string `json:"model"`
	Prompt    string `json:"prompt,omitempty"`
	Status    string `json:"status"`
	Cost      int    `json:"cost"`
	CreatedAt string `json:"created_at"`
	ResultURL string `json:"result_url,omitempty"`
}

func newSoulIdsUsageCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "usage <soul_id>",
		Short: "Every generation that used a given Soul ID, ordered by date, with thumbnails and total cost",
		Long: `Local SQL join over the synced generations and soul_ids tables. Returns one
row per generation that referenced this Soul ID. Add --since 30d to bound the
window.`,
		Example: strings.Trim(`
  higgsfield-pp-cli soul-ids usage soul_riggs_42
  higgsfield-pp-cli soul-ids usage soul_riggs_42 --since 30d --json --select request_id,model,prompt`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("higgsfield-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			soulID := args[0]

			query := `
				SELECT
					COALESCE(json_extract(g.data, '$.request_id'), json_extract(g.data, '$.id'), '') AS request_id,
					COALESCE(json_extract(g.data, '$.model'), '') AS model,
					COALESCE(json_extract(g.data, '$.prompt'), '') AS prompt,
					COALESCE(json_extract(g.data, '$.status'), '') AS status,
					COALESCE(CAST(json_extract(g.data, '$.cost') AS INTEGER), 0) AS cost,
					COALESCE(json_extract(g.data, '$.created_at'), '') AS created_at,
					COALESCE(json_extract(g.data, '$.result_url'), '') AS result_url
				FROM resources g
				WHERE g.resource_type IN ('generations')
				  AND json_extract(g.data, '$.soul_id') = ?
			`
			params := []any{soulID}
			if since != "" {
				query += " AND COALESCE(json_extract(g.data, '$.created_at'), '') >= ?"
				params = append(params, since)
			}
			query += " ORDER BY created_at DESC LIMIT ?"
			params = append(params, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), query, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			var results []soulUsageRow
			var totalCost int
			for rows.Next() {
				var r soulUsageRow
				if err := rows.Scan(&r.RequestID, &r.Model, &r.Prompt, &r.Status, &r.Cost, &r.CreatedAt, &r.ResultURL); err != nil {
					return err
				}
				results = append(results, r)
				totalCost += r.Cost
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"soul_id":     soulID,
					"total_cost":  totalCost,
					"row_count":   len(results),
					"generations": results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No generations found for Soul ID %s. Run `higgsfield-pp-cli sync` if you have not synced recently.\n", soulID)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Soul ID: %s — %d generations, %d credits\n\n", soulID, len(results), totalCost)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-16s %-6s %-20s %s\n", "REQUEST_ID", "MODEL", "COST", "CREATED", "PROMPT")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-16s %-6d %-20s %s\n", truncate(r.RequestID, 18), r.Model, r.Cost, r.CreatedAt, truncate(r.Prompt, 50))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only include generations created on or after this ISO timestamp (e.g. 2026-04-01)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override path to the local SQLite database")
	return cmd
}

// (helpers.go already provides a truncate function; reused here.)
