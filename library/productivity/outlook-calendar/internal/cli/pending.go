// Hand-built (Phase 3): future events whose RSVP is still pending.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newPendingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var horizon string

	cmd := &cobra.Command{
		Use:   "pending",
		Short: "List future events whose RSVP is still pending (response = none)",
		Long:  `Local-data filter that surfaces events you have not yet accepted, declined, or tentatively-accepted, ordered by start time.`,
		Example: strings.Trim(`
  outlook-calendar-pp-cli pending --json
  outlook-calendar-pp-cli pending --within 30d --json --select subject,start,organizer.email
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := parseRelativeDays(horizon)
			if err != nil {
				return usageErr(fmt.Errorf("--within %q: %w", horizon, err))
			}
			now := time.Now()
			end := now.Add(time.Duration(window) * 24 * time.Hour)
			if dbPath == "" {
				dbPath = defaultDBPath("outlook-calendar-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			events, err := loadEvents(cmd.Context(), db.DB(), now, end)
			if err != nil {
				return apiErr(err)
			}

			type pendingRow struct {
				ID        string `json:"id"`
				Subject   string `json:"subject"`
				Start     string `json:"start"`
				End       string `json:"end"`
				Location  string `json:"location"`
				Organizer struct {
					Email string `json:"email"`
					Name  string `json:"name"`
				} `json:"organizer"`
				Response string `json:"response"`
				WebLink  string `json:"web_link"`
			}
			rows := []pendingRow{}
			for _, ev := range events {
				resp := strings.ToLower(ev.ResponseStatus.Response)
				if resp != "none" && resp != "notresponded" {
					continue
				}
				if ev.IsOrganizer {
					continue // we don't RSVP to ourselves
				}
				row := pendingRow{
					ID:       ev.ID,
					Subject:  ev.Subject,
					Start:    ev.Start.DateTime,
					End:      ev.End.DateTime,
					Location: ev.Location,
					Response: ev.ResponseStatus.Response,
					WebLink:  ev.WebLink,
				}
				row.Organizer.Email = strings.ToLower(ev.Organizer.EmailAddress.Address)
				row.Organizer.Name = ev.Organizer.EmailAddress.Name
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Start < rows[j].Start })

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(w, "No pending invites.")
				return nil
			}
			fmt.Fprintf(w, "%d pending invite(s):\n", len(rows))
			for _, r := range rows {
				fmt.Fprintf(w, "  %s  [%s]  %s — %s\n", r.Start, r.Organizer.Email, r.Subject, r.Location)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&horizon, "within", "60d", "Look ahead N days (e.g. 30d, 60d)")
	return cmd
}
