// PATCH: novel cross-project secret-name rollups (where-name, rotation) reading from the local store; not in the Management API.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/store"
)

// newSecretsTopCmd is the top-level `secrets` parent. Coexists with the
// spec-derived `projects secrets ...` commands (which target one project at a
// time via the Management API); this parent groups cross-project rollups that
// read from the local store.
func newSecretsTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Cross-project secret rollups (where-name, rotation audit)",
		Long: `Cross-project queries over locally-synced secret names. Secret VALUES are
never stored — these commands operate on secret names and updated_at timestamps only.
For project-scoped secret CRUD see 'projects secrets ...'.`,
	}
	cmd.AddCommand(newSecretsWhereNameCmd(flags))
	cmd.AddCommand(newSecretsRotationCmd(flags))
	return cmd
}

func newSecretsWhereNameCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "where-name <NAME>",
		Short: "Find every project (across orgs) holding a secret with the given name",
		Long: `Cross-project audit: returns every project where a secret with the given name
exists, plus its updated_at and last sync time. Joins locally-synced secrets,
projects, and organizations tables; no live API calls.`,
		Example: strings.Trim(`
  # Find every project holding STRIPE_KEY
  supabase-pp-cli secrets where-name STRIPE_KEY --json

  # Compact agent-mode for piping into jq
  supabase-pp-cli secrets where-name OPENAI_API_KEY --agent --select project_ref,org_slug
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(args[0])
			if name == "" {
				return usageErr(fmt.Errorf("NAME argument is required (e.g., STRIPE_KEY)"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("supabase-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'supabase-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Secrets store rows tagged with their parent project via the
			// generator's standard parent-fk column. Org info needs a join
			// back through projects -> organizations.
			rows, err := db.Query(`
				SELECT
					COALESCE(p.ref, '') AS project_ref,
					COALESCE(p.name, '') AS project_name,
					COALESCE(p.organization_id, '') AS org_id,
					COALESCE(s.synced_at, '') AS synced_at,
					COALESCE(json_extract(s.data, '$.updated_at'), '') AS updated_at
				FROM secrets s
				LEFT JOIN projects p ON s.projects_id = p.id
				WHERE json_extract(s.data, '$.name') = ?
				ORDER BY p.name
				LIMIT ?
			`, name, limit)
			if err != nil {
				return fmt.Errorf("querying secrets: %w", err)
			}
			defer rows.Close()

			type match struct {
				ProjectRef  string `json:"project_ref"`
				ProjectName string `json:"project_name"`
				OrgID       string `json:"org_id"`
				SyncedAt    string `json:"synced_at"`
				UpdatedAt   string `json:"updated_at"`
			}
			var results []match
			for rows.Next() {
				var m match
				if err := rows.Scan(&m.ProjectRef, &m.ProjectName, &m.OrgID, &m.SyncedAt, &m.UpdatedAt); err != nil {
					continue
				}
				results = append(results, m)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating secrets: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"name":        name,
					"match_count": len(results),
					"projects":    results,
					"queried_at":  time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No projects in the local store hold a secret named %q.\n", name)
				fmt.Fprintln(out, "(If you expect results, run 'supabase-pp-cli sync' to refresh the local store.)")
				return nil
			}
			fmt.Fprintf(out, "Found %d project(s) holding secret %q:\n\n", len(results), name)
			fmt.Fprintf(out, "%-30s %-30s %s\n", "PROJECT_REF", "PROJECT_NAME", "ORG_ID")
			fmt.Fprintf(out, "%-30s %-30s %s\n", "-----------", "------------", "------")
			for _, r := range results {
				fmt.Fprintf(out, "%-30s %-30s %s\n", truncate(r.ProjectRef, 28), truncate(r.ProjectName, 28), r.OrgID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/supabase-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum projects to return")
	return cmd
}

func newSecretsRotationCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var olderThan string
	var limit int

	cmd := &cobra.Command{
		Use:   "rotation",
		Short: "Secrets sorted by age (oldest updated_at first) for rotation hygiene",
		Long: `Age-sort over locally-synced secret_names rows. Defaults to listing secrets
older than 180 days. Tune --older-than (e.g., 90d, 365d) for different policies.`,
		Example: strings.Trim(`
  # Default: secrets not updated in 180+ days
  supabase-pp-cli secrets rotation --json

  # Strict: 90+ days
  supabase-pp-cli secrets rotation --older-than 90d --json
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
					COALESCE(json_extract(s.data, '$.name'), '') AS name,
					COALESCE(p.ref, '') AS project_ref,
					COALESCE(json_extract(s.data, '$.updated_at'), '') AS updated_at
				FROM secrets s
				LEFT JOIN projects p ON s.projects_id = p.id
				WHERE json_extract(s.data, '$.updated_at') < ?
				ORDER BY json_extract(s.data, '$.updated_at') ASC
				LIMIT ?
			`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying secrets: %w", err)
			}
			defer rows.Close()

			type stale struct {
				Name       string `json:"name"`
				ProjectRef string `json:"project_ref"`
				UpdatedAt  string `json:"updated_at"`
				DaysSince  int    `json:"days_since_update"`
			}
			var results []stale
			now := time.Now()
			for rows.Next() {
				var s stale
				if err := rows.Scan(&s.Name, &s.ProjectRef, &s.UpdatedAt); err != nil {
					continue
				}
				if t, perr := time.Parse(time.RFC3339, s.UpdatedAt); perr == nil {
					s.DaysSince = int(now.Sub(t).Hours() / 24)
				}
				results = append(results, s)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating secrets: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"older_than":  olderThan,
					"cutoff":      cutoff,
					"match_count": len(results),
					"secrets":     results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No secrets older than %s in the local store.\n", olderThan)
				return nil
			}
			fmt.Fprintf(out, "%d secret(s) not updated in %s+:\n\n", len(results), olderThan)
			fmt.Fprintf(out, "%-30s %-25s %s\n", "SECRET", "PROJECT_REF", "DAYS_SINCE")
			fmt.Fprintf(out, "%-30s %-25s %s\n", "------", "-----------", "----------")
			for _, r := range results {
				fmt.Fprintf(out, "%-30s %-25s %d\n", truncate(r.Name, 28), truncate(r.ProjectRef, 23), r.DaysSince)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/supabase-pp-cli/data.db)")
	cmd.Flags().StringVar(&olderThan, "older-than", "180d", "Age threshold: secrets older than this surface (e.g., 90d, 180d, 365d)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum secrets to return")
	return cmd
}

// parseDayDuration accepts "Nd" / "Nh" / "Nh30m" / Go duration shapes.
func parseDayDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err != nil {
			return 0, fmt.Errorf("invalid duration %q (use Nd for days, or Go duration like 24h)", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
