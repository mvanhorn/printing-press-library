// PATCH: novel cross-resource lookup — every email sent to a single recipient address, with delivery state and timestamps.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newEmailsToCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "to <recipient>",
		Short: "Find every email sent to a recipient address (newest first)",
		Long: `Cross-resource lookup over locally-synced emails. The Resend API has no
"emails by recipient" filter — answering "what did we send to alice@?" today
requires manual scanning. This command joins emails and events from the local
store and returns the full timeline for one address.`,
		Example: strings.Trim(`
  # Every email sent to alice@example.invalid
  resend-pp-cli emails to alice@example.invalid --json --select id,subject,status,sent_at

  # Last 10 only
  resend-pp-cli emails to bob@example.invalid --limit 10
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			recipient := strings.TrimSpace(args[0])
			if recipient == "" {
				return usageErr(fmt.Errorf("recipient argument is required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("resend-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'resend-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Escape LIKE metacharacters so addresses containing '%' or '_'
			// (e.g., alice_b@example.com) don't widen the match.
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(recipient)
			pattern := "%\"" + escaped + "\"%"
			rows, err := db.Query(`
				SELECT
					e.id,
					COALESCE(e.subject, '') AS subject,
					COALESCE(e."from", '') AS from_addr,
					COALESCE(e.last_event, '') AS status,
					COALESCE(e.created_at, '') AS sent_at,
					COALESCE(json_extract(e.data, '$.to'), '') AS to_field
				FROM emails e
				WHERE json_extract(e.data, '$.to') LIKE ? ESCAPE '\'
					OR json_extract(e.data, '$.to[0]') = ?
				ORDER BY e.created_at DESC
				LIMIT ?
			`, pattern, recipient, limit)
			if err != nil {
				return fmt.Errorf("querying emails: %w", err)
			}
			defer rows.Close()

			type email struct {
				ID      string `json:"id"`
				Subject string `json:"subject"`
				From    string `json:"from"`
				Status  string `json:"status"`
				SentAt  string `json:"sent_at"`
				ToField string `json:"to_field,omitempty"`
			}
			results := []email{}
			for rows.Next() {
				var e email
				if err := rows.Scan(&e.ID, &e.Subject, &e.From, &e.Status, &e.SentAt, &e.ToField); err != nil {
					continue
				}
				results = append(results, e)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating emails: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"recipient":   recipient,
					"match_count": len(results),
					"emails":      results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No emails in the local store sent to %q.\n", recipient)
				fmt.Fprintln(out, "(Run 'resend-pp-cli sync --full' to refresh.)")
				return nil
			}
			fmt.Fprintf(out, "%d email(s) sent to %s:\n\n", len(results), recipient)
			fmt.Fprintf(out, "%-38s %-40s %-12s %s\n", "ID", "SUBJECT", "STATUS", "SENT_AT")
			fmt.Fprintf(out, "%-38s %-40s %-12s %s\n", "--", "-------", "------", "-------")
			for _, r := range results {
				fmt.Fprintf(out, "%-38s %-40s %-12s %s\n", truncate(r.ID, 36), truncate(r.Subject, 38), truncate(r.Status, 12), r.SentAt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum emails to return")
	return cmd
}
