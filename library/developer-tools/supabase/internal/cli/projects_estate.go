// PATCH: novel one-row-per-project estate rollup over local store; renamed from 'health' to avoid spec-derived 'projects health' collision.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/store"
)

// newProjectsEstateCmd implements the "project health rollup" novel feature.
// Named "estate" rather than "health" because the spec already emits
// `projects health` (a wrapper around /v1/projects/{ref}/health/services).
func newProjectsEstateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var orgFilter string

	cmd := &cobra.Command{
		Use:   "estate",
		Short: "One-row-per-project rollup of function/branch/api-key/secret counts",
		Long: `LEFT JOIN across locally-synced projects + functions + branches + api_keys
+ secrets. The Monday morning estate review — one screen, the whole portfolio.
Distinct from 'projects health' which calls the live Management /health/services
endpoint for one project.`,
		Example: strings.Trim(`
  # Full estate
  supabase-pp-cli projects estate --json

  # Scope to one org
  supabase-pp-cli projects estate --org acme --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("supabase-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'supabase-pp-cli sync' first.", err)
			}
			defer db.Close()

			q := `
				SELECT
					COALESCE(p.ref, '') AS ref,
					COALESCE(p.name, '') AS name,
					COALESCE(p.organization_id, '') AS org_id,
					COALESCE(p.status, '') AS status,
					COALESCE(p.region, '') AS region,
					(SELECT COUNT(*) FROM functions f WHERE f.projects_id = p.id) AS function_count,
					(SELECT COUNT(*) FROM branches b
						WHERE COALESCE(json_extract(b.data, '$.parent_project_ref'), '') = p.ref
						   OR COALESCE(json_extract(b.data, '$.project_ref'), '') = p.ref) AS branch_count,
					(SELECT COUNT(*) FROM api_keys a WHERE a.projects_id = p.id) AS api_key_count,
					(SELECT COUNT(*) FROM secrets s WHERE s.projects_id = p.id) AS secret_count,
					COALESCE(p.synced_at, '') AS synced_at
				FROM projects p
			`
			qArgs := []any{}
			if orgFilter != "" {
				q += " WHERE p.organization_id = ? OR p.organization_slug = ?"
				qArgs = append(qArgs, orgFilter, orgFilter)
			}
			q += " ORDER BY p.name"

			rows, err := db.Query(q, qArgs...)
			if err != nil {
				return fmt.Errorf("querying projects: %w", err)
			}
			defer rows.Close()

			type estate struct {
				Ref           string `json:"ref"`
				Name          string `json:"name"`
				OrgID         string `json:"org_id"`
				Status        string `json:"status"`
				Region        string `json:"region"`
				FunctionCount int    `json:"function_count"`
				BranchCount   int    `json:"branch_count"`
				APIKeyCount   int    `json:"api_key_count"`
				SecretCount   int    `json:"secret_count"`
				SyncedAt      string `json:"synced_at"`
			}
			var results []estate
			for rows.Next() {
				var e estate
				if err := rows.Scan(&e.Ref, &e.Name, &e.OrgID, &e.Status, &e.Region,
					&e.FunctionCount, &e.BranchCount, &e.APIKeyCount, &e.SecretCount, &e.SyncedAt); err != nil {
					continue
				}
				results = append(results, e)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating projects: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"org_filter": orgFilter,
					"count":      len(results),
					"projects":   results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No projects in local store. Run 'supabase-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(out, "%d project(s) estate rollup:\n\n", len(results))
			fmt.Fprintf(out, "%-25s %-25s %-6s %-6s %-6s %s\n", "NAME", "REF", "FN", "BR", "SEC", "STATUS")
			fmt.Fprintf(out, "%-25s %-25s %-6s %-6s %-6s %s\n", "----", "---", "--", "--", "---", "------")
			for _, r := range results {
				fmt.Fprintf(out, "%-25s %-25s %-6d %-6d %-6d %s\n",
					truncate(r.Name, 23), truncate(r.Ref, 23),
					r.FunctionCount, r.BranchCount, r.SecretCount, r.Status)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/supabase-pp-cli/data.db)")
	cmd.Flags().StringVar(&orgFilter, "org", "", "Filter to a single organization (id or slug)")
	return cmd
}
