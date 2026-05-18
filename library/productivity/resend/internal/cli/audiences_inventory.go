// PATCH: novel per-audience rollup — contact count, unsubscribed count, last-broadcast timestamp. No aggregate API endpoint exists.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newAudiencesInventoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Per-audience rollup: contact count, unsubscribed count, last broadcast (no aggregate API)",
		Long: `Aggregates locally-synced audiences with derived counts. Resend has no
aggregate endpoint for this — the dashboard shows one audience at a time.
Useful before planning a broadcast to spot audiences with high unsubscribe
rates or stale engagement.`,
		Example: strings.Trim(`
  # Full inventory across all audiences
  resend-pp-cli audiences inventory --json

  # Compact agent-mode
  resend-pp-cli audiences inventory --agent --select id,name,contact_count,unsubscribed_count
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("resend-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'resend-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT
					a.id,
					COALESCE(json_extract(a.data, '$.name'), '') AS name,
					COALESCE(a.created_at, '') AS created_at,
					(SELECT COUNT(*) FROM contacts c WHERE json_extract(c.data, '$.audience_id') = a.id) AS contact_count,
					(SELECT COUNT(*) FROM contacts c WHERE json_extract(c.data, '$.audience_id') = a.id AND COALESCE(c.unsubscribed, 0) = 1) AS unsubscribed_count,
					(SELECT MAX(b.sent_at) FROM broadcasts b WHERE b.audience_id = a.id) AS last_broadcast_at
				FROM audiences a
				ORDER BY name
			`)
			if err != nil {
				return fmt.Errorf("querying audiences: %w", err)
			}
			defer rows.Close()

			type row struct {
				ID                string `json:"id"`
				Name              string `json:"name"`
				CreatedAt         string `json:"created_at"`
				ContactCount      int    `json:"contact_count"`
				UnsubscribedCount int    `json:"unsubscribed_count"`
				LastBroadcastAt   string `json:"last_broadcast_at"`
			}
			results := []row{}
			for rows.Next() {
				var r row
				var last *string
				if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt, &r.ContactCount, &r.UnsubscribedCount, &last); err != nil {
					continue
				}
				if last != nil {
					r.LastBroadcastAt = *last
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating audiences: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"count":     len(results),
					"audiences": results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No audiences in the local store.")
				fmt.Fprintln(out, "(Run 'resend-pp-cli sync --full' to refresh.)")
				return nil
			}
			fmt.Fprintf(out, "%d audience(s):\n\n", len(results))
			fmt.Fprintf(out, "%-30s %-10s %-12s %s\n", "NAME", "CONTACTS", "UNSUB", "LAST_BROADCAST")
			fmt.Fprintf(out, "%-30s %-10s %-12s %s\n", "----", "--------", "-----", "--------------")
			for _, r := range results {
				fmt.Fprintf(out, "%-30s %-10d %-12d %s\n", truncate(r.Name, 28), r.ContactCount, r.UnsubscribedCount, r.LastBroadcastAt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	return cmd
}
