// Hand-built (Phase 3): meeting prep dossier for the next N hours.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newPrepCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var horizon string

	cmd := &cobra.Command{
		Use:   "prep",
		Short: "Pre-joined dossier for upcoming events: subject, location, attendees, body excerpt, attachments-meta, recurrence and online-meeting flags",
		Example: strings.Trim(`
  outlook-calendar-pp-cli prep --next 4h --json
  outlook-calendar-pp-cli prep --next 24h --json --select subject,start,attendees,body_preview
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := time.ParseDuration(horizon)
			if err != nil || d <= 0 {
				return usageErr(fmt.Errorf("--next must be a positive duration like 4h or 24h"))
			}
			now := time.Now()
			if dbPath == "" {
				dbPath = defaultDBPath("outlook-calendar-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			events, err := loadEvents(cmd.Context(), db.DB(), now, now.Add(d))
			if err != nil {
				return apiErr(err)
			}

			type attendee struct {
				Email    string `json:"email"`
				Name     string `json:"name"`
				Type     string `json:"type"`
				Response string `json:"response"`
			}
			type prepRow struct {
				ID             string `json:"id"`
				Subject        string `json:"subject"`
				Start          string `json:"start"`
				End            string `json:"end"`
				StartTZ        string `json:"start_tz"`
				Location       string `json:"location"`
				IsOnline       bool   `json:"is_online"`
				OnlineProvider string `json:"online_provider"`
				IsOrganizer    bool   `json:"is_organizer"`
				IsRecurring    bool   `json:"is_recurring"`
				ShowAs         string `json:"show_as"`
				BodyPreview    string `json:"body_preview"`
				Organizer      struct {
					Email string `json:"email"`
					Name  string `json:"name"`
				} `json:"organizer"`
				Attendees []attendee `json:"attendees"`
				Response  string     `json:"response"`
				WebLink   string     `json:"web_link"`
			}

			rows := []prepRow{}
			for _, ev := range events {
				if ev.IsCancelled {
					continue
				}
				row := prepRow{
					ID:             ev.ID,
					Subject:        ev.Subject,
					Start:          ev.Start.DateTime,
					End:            ev.End.DateTime,
					StartTZ:        ev.Start.TimeZone,
					Location:       ev.Location,
					IsOnline:       ev.OnlineMeeting,
					OnlineProvider: ev.OnlineProvider,
					IsOrganizer:    ev.IsOrganizer,
					IsRecurring:    ev.Type == "occurrence" || ev.Type == "exception" || ev.Type == "seriesMaster",
					ShowAs:         ev.ShowAs,
					BodyPreview:    ev.BodyPreview,
					Response:       ev.ResponseStatus.Response,
					WebLink:        ev.WebLink,
				}
				row.Organizer.Email = strings.ToLower(ev.Organizer.EmailAddress.Address)
				row.Organizer.Name = ev.Organizer.EmailAddress.Name
				for _, a := range ev.Attendees {
					row.Attendees = append(row.Attendees, attendee{
						Email:    a.Email,
						Name:     a.Name,
						Type:     a.Type,
						Response: a.Response,
					})
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Start < rows[j].Start })

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(w, "No upcoming events in the prep window.")
				return nil
			}
			fmt.Fprintf(w, "%d upcoming event(s):\n", len(rows))
			for _, r := range rows {
				fmt.Fprintf(w, "\n  %s  %s  [%d attendee(s)]\n", r.Start, r.Subject, len(r.Attendees))
				if r.Location != "" {
					fmt.Fprintf(w, "    Location: %s\n", r.Location)
				}
				if r.BodyPreview != "" {
					preview := r.BodyPreview
					if len(preview) > 160 {
						preview = preview[:160] + "..."
					}
					fmt.Fprintf(w, "    Body:     %s\n", preview)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&horizon, "next", "4h", "Look ahead duration (e.g. 4h, 24h)")
	return cmd
}
