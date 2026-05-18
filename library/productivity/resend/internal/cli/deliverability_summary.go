// PATCH: novel deliverability summary — bounce / complaint / suppression rates over rolling window from local events. Direct answer to documented "deliverability blind spot" complaint.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newDeliverabilityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliverability",
		Short: "Rolling-window deliverability summary (bounce / complaint / suppression rates)",
		Long: `Cross-event deliverability metrics computed from the local events table.
The Resend API has no aggregate bounce/complaint endpoint — this is the
documented "deliverability blind spot" users have been asking about.`,
	}
	cmd.AddCommand(newDeliverabilitySummaryCmd(flags))
	return cmd
}

func newDeliverabilitySummaryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var windowSpec string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Bounce rate, complaint rate, and suppression count over a rolling window (default 7d)",
		Long: `Computes bounce / complaint / suppression metrics from locally-synced
events over a rolling window (default 7 days). Returns rates per million
sends as well as raw counts.`,
		Example: strings.Trim(`
  # Last 7 days
  resend-pp-cli deliverability summary --json

  # Last 30 days
  resend-pp-cli deliverability summary --window 30d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDayDuration(windowSpec)
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

			var sent, delivered, bounced, complained, suppressed int
			row := db.DB().QueryRow(`
				SELECT
					(SELECT COUNT(*) FROM events WHERE name = 'email.sent'       AND created_at >= ?),
					(SELECT COUNT(*) FROM events WHERE name = 'email.delivered'  AND created_at >= ?),
					(SELECT COUNT(*) FROM events WHERE name = 'email.bounced'    AND created_at >= ?),
					(SELECT COUNT(*) FROM events WHERE name = 'email.complained' AND created_at >= ?),
					(SELECT COUNT(*) FROM events WHERE name = 'email.suppressed' AND created_at >= ?)
			`, cutoff, cutoff, cutoff, cutoff, cutoff)
			if err := row.Scan(&sent, &delivered, &bounced, &complained, &suppressed); err != nil {
				return fmt.Errorf("computing deliverability: %w", err)
			}

			denom := sent
			if denom == 0 {
				denom = delivered
			}
			rate := func(n int) float64 {
				if denom == 0 {
					return 0
				}
				return float64(n) / float64(denom)
			}
			bounceRate := rate(bounced)
			complaintRate := rate(complained)
			suppressionRate := rate(suppressed)

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"window":             windowSpec,
					"cutoff":             cutoff,
					"sent":               sent,
					"delivered":          delivered,
					"bounced":            bounced,
					"complained":         complained,
					"suppressed":         suppressed,
					"bounce_rate":        bounceRate,
					"complaint_rate":     complaintRate,
					"suppression_count":  suppressed,
					"suppression_rate":   suppressionRate,
				}, flags)
			}
			fmt.Fprintf(out, "Deliverability over last %s (since %s)\n\n", windowSpec, cutoff)
			fmt.Fprintf(out, "  Sent:        %d\n", sent)
			fmt.Fprintf(out, "  Delivered:   %d\n", delivered)
			fmt.Fprintf(out, "  Bounced:     %d   (%.2f%%)\n", bounced, bounceRate*100)
			fmt.Fprintf(out, "  Complained:  %d   (%.2f%%)\n", complained, complaintRate*100)
			fmt.Fprintf(out, "  Suppressed:  %d   (%.2f%%)\n", suppressed, suppressionRate*100)
			if sent == 0 && delivered == 0 {
				fmt.Fprintln(out, "\n(No events in the local store for this window. Run 'resend-pp-cli sync events --full'.)")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	cmd.Flags().StringVar(&windowSpec, "window", "7d", "Time window: 24h, 7d, 30d, etc.")
	return cmd
}

// parseDayDuration accepts "Nd" / "Nh" / Go-duration shapes.
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
