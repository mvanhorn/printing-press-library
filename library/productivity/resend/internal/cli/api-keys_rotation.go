// PATCH: novel API-key rotation audit — keys sorted by age + last-used (joined from logs); flags stale keys for quarterly security reviews.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newAPIKeysRotationCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var olderThan string

	cmd := &cobra.Command{
		Use:   "rotation",
		Short: "API keys sorted by age + last-used; flags stale keys older than N days",
		Long: `Cross-resource rotation audit. The /api-keys list endpoint does not include
last_used_at at scale — this command joins api_keys with logs to derive each
key's last-used timestamp and surfaces stale keys for quarterly rotation
reviews.`,
		Example: strings.Trim(`
  # Keys older than 90 days
  resend-pp-cli api-keys rotation --json

  # Stricter window
  resend-pp-cli api-keys rotation --older-than 30d --json --select name,created_at,last_used_at,days_since_use
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
				dbPath = defaultDBPath("resend-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'resend-pp-cli sync' first.", err)
			}
			defer db.Close()

			// NULL created_at must not silently pass the age filter. SQLite's
			// `COALESCE(x, '') < '<iso>'` is TRUE for the empty string (lex
			// sorts before any digit), so NULL-created keys would otherwise
			// appear in every result regardless of --older-than. Require a
			// real timestamp explicitly and let SQL's NULL semantics drop
			// the ambiguous rows from the comparison.
			rows, err := db.Query(`
				SELECT
					k.id,
					COALESCE(json_extract(k.data, '$.name'), '') AS name,
					COALESCE(k.created_at, '') AS created_at,
					COALESCE(json_extract(k.data, '$.permission'), '') AS permission,
					(SELECT MAX(l.created_at) FROM logs l WHERE l.data LIKE '%"' || k.id || '"%') AS last_used_at
				FROM api_keys k
				WHERE k.created_at IS NOT NULL AND k.created_at < ?
				ORDER BY k.created_at ASC
			`, cutoff)
			if err != nil {
				return fmt.Errorf("querying api_keys: %w", err)
			}
			defer rows.Close()

			type entry struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				CreatedAt    string `json:"created_at"`
				Permission   string `json:"permission"`
				LastUsedAt   string `json:"last_used_at"`
				DaysSinceUse int    `json:"days_since_use"`
				AgeDays      int    `json:"age_days"`
			}
			results := []entry{}
			now := time.Now()
			for rows.Next() {
				var e entry
				var lastUsed *string
				if err := rows.Scan(&e.ID, &e.Name, &e.CreatedAt, &e.Permission, &lastUsed); err != nil {
					continue
				}
				if lastUsed != nil {
					e.LastUsedAt = *lastUsed
					if t, ok := parseTimestamp(e.LastUsedAt); ok {
						e.DaysSinceUse = int(now.Sub(t).Hours() / 24)
					}
				}
				if t, ok := parseTimestamp(e.CreatedAt); ok {
					e.AgeDays = int(now.Sub(t).Hours() / 24)
				}
				results = append(results, e)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating api_keys: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"older_than":  olderThan,
					"cutoff":      cutoff,
					"match_count": len(results),
					"api_keys":    results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No API keys older than %s in the local store.\n", olderThan)
				return nil
			}
			fmt.Fprintf(out, "%d API key(s) older than %s:\n\n", len(results), olderThan)
			fmt.Fprintf(out, "%-25s %-12s %-10s %-12s %s\n", "NAME", "AGE_DAYS", "LAST_USED", "PERMISSION", "ID")
			fmt.Fprintf(out, "%-25s %-12s %-10s %-12s %s\n", "----", "--------", "---------", "----------", "--")
			for _, r := range results {
				lu := r.LastUsedAt
				if lu == "" {
					lu = "never"
				} else if r.DaysSinceUse > 0 {
					lu = fmt.Sprintf("%dd ago", r.DaysSinceUse)
				}
				fmt.Fprintf(out, "%-25s %-12d %-10s %-12s %s\n", truncate(r.Name, 23), r.AgeDays, truncate(lu, 10), truncate(r.Permission, 10), truncate(r.ID, 30))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	cmd.Flags().StringVar(&olderThan, "older-than", "90d", "Age threshold: keys older than this surface (e.g., 30d, 90d, 180d)")
	return cmd
}
