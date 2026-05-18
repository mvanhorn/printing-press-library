// PATCH: novel cross-event delivery trace — collapsed event chain (sent/delivered/opened/clicked/bounced) for one email.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newEmailsTimelineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "timeline <email-id>",
		Short: "Collapsed delivery event chain for one email (sent → delivered → opened → clicked → bounced)",
		Long: `The Resend API splits delivery state across /emails/{id} and /logs.
This command joins them locally and shows the full ordered event chain for
one email in a single table.`,
		Example: strings.Trim(`
  # Full timeline for an email
  resend-pp-cli emails timeline 4ef9a417-d4ff-4ec5-9af2-c80a4d5d2c1f --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			emailID := strings.TrimSpace(args[0])
			if emailID == "" {
				return usageErr(fmt.Errorf("email-id argument is required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("resend-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'resend-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Pull the email's headline row.
			var subject, from, lastEvent, createdAt string
			row := db.DB().QueryRow(`
				SELECT
					COALESCE(subject, ''),
					COALESCE("from", ''),
					COALESCE(last_event, ''),
					COALESCE(created_at, '')
				FROM emails WHERE id = ?
			`, emailID)
			if err := row.Scan(&subject, &from, &lastEvent, &createdAt); err != nil {
				return fmt.Errorf("email %s not found in local store: %w\nRun 'resend-pp-cli sync' first.", emailID, err)
			}

			// Pull every event that references this email_id in the events.data JSON.
			// Escape LIKE metacharacters so emailIDs containing '%' or '_'
			// don't widen the match. Email IDs are normally UUIDs, but they
			// can be any opaque string Resend assigns — defense in depth.
			escapedID := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(emailID)
			pattern := "%\"" + escapedID + "\"%"
			rows, err := db.Query(`
				SELECT
					COALESCE(name, '') AS event_name,
					COALESCE(created_at, '') AS occurred_at,
					COALESCE(json_extract(data, '$.data.click.link'), '') AS link_url
				FROM events
				WHERE data LIKE ? ESCAPE '\'
				ORDER BY created_at ASC
			`, pattern)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}
			defer rows.Close()

			type evt struct {
				Name       string `json:"event"`
				OccurredAt string `json:"occurred_at"`
				LinkURL    string `json:"link_url,omitempty"`
			}
			events := []evt{}
			for rows.Next() {
				var e evt
				if err := rows.Scan(&e.Name, &e.OccurredAt, &e.LinkURL); err != nil {
					continue
				}
				events = append(events, e)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating events: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"email_id":    emailID,
					"subject":     subject,
					"from":        from,
					"last_event":  lastEvent,
					"sent_at":     createdAt,
					"event_count": len(events),
					"events":      events,
				}, flags)
			}
			fmt.Fprintf(out, "Email %s\n  Subject: %s\n  From:    %s\n  Sent:    %s\n  Last:    %s\n\n", emailID, subject, from, createdAt, lastEvent)
			if len(events) == 0 {
				fmt.Fprintln(out, "No events found in the local store for this email.")
				fmt.Fprintln(out, "(Run 'resend-pp-cli sync events --full' to refresh.)")
				return nil
			}
			fmt.Fprintf(out, "%d event(s):\n\n", len(events))
			fmt.Fprintf(out, "%-25s %-25s %s\n", "EVENT", "OCCURRED_AT", "LINK")
			fmt.Fprintf(out, "%-25s %-25s %s\n", "-----", "-----------", "----")
			for _, e := range events {
				fmt.Fprintf(out, "%-25s %-25s %s\n", truncate(e.Name, 23), e.OccurredAt, truncate(e.LinkURL, 50))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	return cmd
}
