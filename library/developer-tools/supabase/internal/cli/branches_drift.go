// PATCH: novel branches-drift sweep over locally-synced preview branches; not in the Management API.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/store"
)

func newBranchesDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var olderThan string
	var limit int

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "List preview branches older than N days that haven't been merged/deleted",
		Long: `Stale-branch sweep across locally-synced branches. Returns rows whose status is
not 'merged' or 'deleted' and whose created_at is older than --older-than (default 7d),
grouped by parent project. The Tuesday cleanup target list.`,
		Example: strings.Trim(`
  # Default: branches older than 7 days
  supabase-pp-cli branches drift --json

  # Stricter window
  supabase-pp-cli branches drift --older-than 14d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDayDuration(olderThan)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-d).UTC().Format(time.RFC3339)

			if dbPath == "" {
				dbPath = defaultDBPath("supabase-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'supabase-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT
					COALESCE(json_extract(b.data, '$.id'), '') AS id,
					COALESCE(json_extract(b.data, '$.name'), '') AS name,
					COALESCE(json_extract(b.data, '$.status'), 'unknown') AS status,
					COALESCE(json_extract(b.data, '$.created_at'), '') AS created_at,
					COALESCE(json_extract(b.data, '$.parent_project_ref'), '') AS parent_ref,
					COALESCE(json_extract(b.data, '$.project_ref'), '') AS project_ref
				FROM branches b
				WHERE COALESCE(json_extract(b.data, '$.created_at'), '') < ?
				  AND COALESCE(json_extract(b.data, '$.status'), '') NOT IN ('MERGED', 'DELETED', 'merged', 'deleted')
				ORDER BY json_extract(b.data, '$.created_at') ASC
				LIMIT ?
			`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying branches: %w", err)
			}
			defer rows.Close()

			type drift struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Status     string `json:"status"`
				CreatedAt  string `json:"created_at"`
				ParentRef  string `json:"parent_project_ref"`
				ProjectRef string `json:"project_ref"`
				AgeDays    int    `json:"age_days"`
			}
			var results []drift
			now := time.Now()
			for rows.Next() {
				var d drift
				if err := rows.Scan(&d.ID, &d.Name, &d.Status, &d.CreatedAt, &d.ParentRef, &d.ProjectRef); err != nil {
					continue
				}
				if t, perr := time.Parse(time.RFC3339, d.CreatedAt); perr == nil {
					d.AgeDays = int(now.Sub(t).Hours() / 24)
				}
				results = append(results, d)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating branches: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"older_than":  olderThan,
					"cutoff":      cutoff,
					"match_count": len(results),
					"branches":    results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No stale branches older than %s.\n", olderThan)
				return nil
			}
			fmt.Fprintf(out, "%d stale branch(es) older than %s:\n\n", len(results), olderThan)
			fmt.Fprintf(out, "%-25s %-15s %-10s %s\n", "NAME", "PARENT_REF", "AGE_DAYS", "STATUS")
			fmt.Fprintf(out, "%-25s %-15s %-10s %s\n", "----", "----------", "--------", "------")
			for _, r := range results {
				fmt.Fprintf(out, "%-25s %-15s %-10d %s\n", truncate(r.Name, 23), truncate(r.ParentRef, 13), r.AgeDays, r.Status)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/supabase-pp-cli/data.db)")
	cmd.Flags().StringVar(&olderThan, "older-than", "7d", "Age threshold: branches older than this surface (e.g., 3d, 7d, 14d)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum branches to return")
	return cmd
}
