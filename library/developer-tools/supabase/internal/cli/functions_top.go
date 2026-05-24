// PATCH: novel cross-project edge-function inventory rollup reading from the local store; not in the Management API.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/store"
)

// newFunctionsTopCmd is the top-level `functions` parent for cross-project
// edge function rollups. Coexists with the spec-derived `projects functions ...`
// commands which target a single project's functions via the Management API.
func newFunctionsTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "functions",
		Short: "Cross-project edge-function rollups (inventory)",
		Long: `Cross-project queries over locally-synced edge functions. For single-project
function CRUD (list/create/deploy/delete) see 'projects functions ...'.`,
	}
	cmd.AddCommand(newFunctionsInventoryCmd(flags))
	return cmd
}

func newFunctionsInventoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var orgFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Per-project, per-org rollup of every edge function (slug, version, status, deployed_at)",
		Long: `Aggregates locally-synced functions by parent project. Surfaces 'which projects
deployed stripe-webhook?' and 'which functions haven't redeployed recently?' in
one table. Use --org to scope to a single organization.`,
		Example: strings.Trim(`
  # Full inventory across all orgs
  supabase-pp-cli functions inventory --json

  # Scoped to one org
  supabase-pp-cli functions inventory --org acme --json
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
					COALESCE(json_extract(f.data, '$.slug'), '') AS slug,
					COALESCE(json_extract(f.data, '$.name'), '') AS name,
					COALESCE(json_extract(f.data, '$.status'), '') AS status,
					COALESCE(CAST(json_extract(f.data, '$.version') AS TEXT), '') AS version,
					COALESCE(json_extract(f.data, '$.updated_at'), '') AS deployed_at,
					COALESCE(p.ref, '') AS project_ref,
					COALESCE(p.name, '') AS project_name,
					COALESCE(p.organization_id, '') AS org_id
				FROM functions f
				LEFT JOIN projects p ON f.projects_id = p.id
			`
			qArgs := []any{}
			if orgFilter != "" {
				q += " WHERE p.organization_id = ? OR p.organization_slug = ?"
				qArgs = append(qArgs, orgFilter, orgFilter)
			}
			q += " ORDER BY p.name, json_extract(f.data, '$.slug') LIMIT ?"
			qArgs = append(qArgs, limit)

			rows, err := db.Query(q, qArgs...)
			if err != nil {
				return fmt.Errorf("querying functions: %w", err)
			}
			defer rows.Close()

			type fn struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Status      string `json:"status"`
				Version     string `json:"version"`
				DeployedAt  string `json:"deployed_at"`
				ProjectRef  string `json:"project_ref"`
				ProjectName string `json:"project_name"`
				OrgID       string `json:"org_id"`
			}
			var results []fn
			for rows.Next() {
				var f fn
				if err := rows.Scan(&f.Slug, &f.Name, &f.Status, &f.Version, &f.DeployedAt, &f.ProjectRef, &f.ProjectName, &f.OrgID); err != nil {
					continue
				}
				results = append(results, f)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating functions: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"org_filter": orgFilter,
					"count":      len(results),
					"functions":  results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No functions in the local store. Run 'supabase-pp-cli sync' to refresh.")
				return nil
			}
			fmt.Fprintf(out, "%d function(s):\n\n", len(results))
			fmt.Fprintf(out, "%-25s %-25s %-10s %s\n", "SLUG", "PROJECT", "STATUS", "DEPLOYED_AT")
			fmt.Fprintf(out, "%-25s %-25s %-10s %s\n", "----", "-------", "------", "-----------")
			for _, r := range results {
				fmt.Fprintf(out, "%-25s %-25s %-10s %s\n", truncate(r.Slug, 23), truncate(r.ProjectName, 23), truncate(r.Status, 10), r.DeployedAt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/supabase-pp-cli/data.db)")
	cmd.Flags().StringVar(&orgFilter, "org", "", "Filter to a single organization (id or slug)")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum functions to return")
	return cmd
}
