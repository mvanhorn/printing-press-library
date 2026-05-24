// Hand-authored novel feature: what-changed-since. Survives regen.
package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type changedEvent struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	Calendar   string `json:"calendar"`
	Status     string `json:"status"`
	ChangeType string `json:"change_type"` // created | updated | cancelled
	Updated    string `json:"updated"`
	Start      string `json:"start,omitempty"`
}

func newChangesCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, since string
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Show events created, updated, or cancelled since a date",
		Long: "List events that changed since --since, classified as created / updated / cancelled.\n\n" +
			"Cancelled (deleted) events are always included. Reads the local event store's update timestamps\n" +
			"(or fetches with updatedMin), so it answers \"what moved on my calendar since Friday?\" — impossible\n" +
			"for live-only tools that can only show the current state.",
		Example: "  google-calendar-pp-cli changes --since 2026-05-17 --calendars primary --agent --select id,summary,status,updated",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sinceTime, err := parsePointTime(since)
			if err != nil {
				return usageErr(err)
			}
			cals := resolveCalendars(calendarsCSV)
			events, _, err := gcalLoadEvents(cmd, flags, eventQuery{
				calendars:   cals,
				updatedMin:  sinceTime,
				showDeleted: true,
			})
			if err != nil {
				return err
			}

			var out []changedEvent
			for _, ev := range events {
				ct := "updated"
				if ev.Status == "cancelled" {
					ct = "cancelled"
				} else if !ev.Created.IsZero() && !ev.Created.Before(sinceTime) {
					ct = "created"
				}
				ce := changedEvent{
					ID:         ev.ID,
					Summary:    ev.Summary,
					Calendar:   ev.CalendarID,
					Status:     ev.Status,
					ChangeType: ct,
					Updated:    ev.Updated.Format(time.RFC3339),
				}
				if !ev.Start.IsZero() {
					ce.Start = ev.Start.Format(time.RFC3339)
				}
				out = append(out, ce)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
			if out == nil {
				out = []changedEvent{}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to check")
	cmd.Flags().StringVar(&since, "since", "7d", "Show changes since this point (2026-05-17, RFC3339, or 7d)")
	return cmd
}
