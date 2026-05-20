// PATCH: novel broadcast performance dashboard — open / click / bounce rate across all broadcasts; dashboard caps at 30d single-broadcast.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newBroadcastsPerformanceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Open / click / bounce rates across all broadcasts (no 30d window cap)",
		Long: `Aggregates locally-synced broadcasts and joins per-broadcast event counts
from the events table. The Resend dashboard shows one broadcast at a time
and caps the window at 30 days; this command lists every broadcast in the
local store with delivery / open / click / bounce counts.`,
		Example: strings.Trim(`
  # All broadcasts
  resend-pp-cli broadcasts performance --json

  # Only sent broadcasts
  resend-pp-cli broadcasts performance --status sent --json --select name,open_rate,click_rate,sent_at
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

			// Single LEFT JOIN with GROUP BY replaces 4 correlated subqueries
			// that each did a full LIKE scan of the events table per broadcast.
			// Now one events scan per broadcast row, producing all 4 counts.
			q := `
				SELECT
					b.id,
					COALESCE(b.name, '') AS name,
					COALESCE(b.subject, '') AS subject,
					COALESCE(b.status, '') AS status,
					COALESCE(b.sent_at, '') AS sent_at,
					COALESCE(b.audience_id, '') AS audience_id,
					COALESCE(SUM(CASE WHEN ev.name = 'email.delivered' THEN 1 ELSE 0 END), 0) AS delivered,
					COALESCE(SUM(CASE WHEN ev.name = 'email.opened'    THEN 1 ELSE 0 END), 0) AS opened,
					COALESCE(SUM(CASE WHEN ev.name = 'email.clicked'   THEN 1 ELSE 0 END), 0) AS clicked,
					COALESCE(SUM(CASE WHEN ev.name = 'email.bounced'   THEN 1 ELSE 0 END), 0) AS bounced
				FROM broadcasts b
				LEFT JOIN events ev
					ON ev.name IN ('email.delivered','email.opened','email.clicked','email.bounced')
					AND ev.data LIKE '%"' || b.id || '"%'
			`
			qArgs := []any{}
			if statusFilter != "" {
				q += " WHERE b.status = ?"
				qArgs = append(qArgs, statusFilter)
			}
			q += " GROUP BY b.id, b.name, b.subject, b.status, b.sent_at, b.audience_id, b.created_at"
			q += " ORDER BY b.sent_at DESC, b.created_at DESC"

			rows, err := db.Query(q, qArgs...)
			if err != nil {
				return fmt.Errorf("querying broadcasts: %w", err)
			}
			defer rows.Close()

			type perf struct {
				ID         string  `json:"id"`
				Name       string  `json:"name"`
				Subject    string  `json:"subject"`
				Status     string  `json:"status"`
				SentAt     string  `json:"sent_at"`
				AudienceID string  `json:"audience_id"`
				Delivered  int     `json:"delivered"`
				Opened     int     `json:"opened"`
				Clicked    int     `json:"clicked"`
				Bounced    int     `json:"bounced"`
				OpenRate   float64 `json:"open_rate"`
				ClickRate  float64 `json:"click_rate"`
				BounceRate float64 `json:"bounce_rate"`
			}
			results := []perf{}
			for rows.Next() {
				var p perf
				if err := rows.Scan(&p.ID, &p.Name, &p.Subject, &p.Status, &p.SentAt, &p.AudienceID,
					&p.Delivered, &p.Opened, &p.Clicked, &p.Bounced); err != nil {
					continue
				}
				if p.Delivered > 0 {
					p.OpenRate = float64(p.Opened) / float64(p.Delivered)
					p.ClickRate = float64(p.Clicked) / float64(p.Delivered)
					p.BounceRate = float64(p.Bounced) / float64(p.Delivered)
				}
				results = append(results, p)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating broadcasts: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"count":      len(results),
					"status":     statusFilter,
					"broadcasts": results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No broadcasts in the local store.")
				fmt.Fprintln(out, "(Run 'resend-pp-cli sync --full' to refresh.)")
				return nil
			}
			fmt.Fprintf(out, "%d broadcast(s):\n\n", len(results))
			fmt.Fprintf(out, "%-25s %-12s %-10s %-8s %-8s %s\n", "NAME", "STATUS", "DELIVERED", "OPEN%", "CLICK%", "SENT_AT")
			fmt.Fprintf(out, "%-25s %-12s %-10s %-8s %-8s %s\n", "----", "------", "---------", "-----", "------", "-------")
			for _, r := range results {
				fmt.Fprintf(out, "%-25s %-12s %-10d %-8.1f %-8.1f %s\n",
					truncate(r.Name, 23), truncate(r.Status, 10), r.Delivered, r.OpenRate*100, r.ClickRate*100, r.SentAt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter to a single status (draft, scheduled, sent)")
	return cmd
}
