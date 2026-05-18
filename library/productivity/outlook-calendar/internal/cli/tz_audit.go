// Hand-built (Phase 3): time-zone audit. Finds events whose start/end TZs
// disagree, or whose start TZ differs from the calendar's default.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newTzAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "tz-audit",
		Short:       "Surface events whose start/end time zones disagree or differ from the default calendar",
		Long:        `Pure local filter that flags events whose start.timeZone differs from end.timeZone, or where start.timeZone differs from the default-calendar TZ. These are the silent-bug class behind "this rendered at the wrong hour for the attendee".`,
		Example:     "  outlook-calendar-pp-cli tz-audit --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("outlook-calendar-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			// Resolve default calendar TZ if recorded in the synced calendar row.
			var defaultTZ string
			row := db.DB().QueryRowContext(cmd.Context(),
				`SELECT json_extract(data, '$.timeZone') FROM resources WHERE resource_type = 'me_settings' LIMIT 1`)
			_ = row.Scan(&defaultTZ)

			events, err := loadEvents(cmd.Context(), db.DB(), time.Time{}, time.Time{})
			if err != nil {
				return apiErr(err)
			}

			type finding struct {
				ID      string `json:"id"`
				Subject string `json:"subject"`
				Start   string `json:"start"`
				End     string `json:"end"`
				StartTZ string `json:"start_tz"`
				EndTZ   string `json:"end_tz"`
				Reason  string `json:"reason"`
				WebLink string `json:"web_link,omitempty"`
			}
			rows := []finding{}
			for _, ev := range events {
				if ev.Start.TimeZone == "" || ev.End.TimeZone == "" {
					continue
				}
				reasons := []string{}
				if !strings.EqualFold(ev.Start.TimeZone, ev.End.TimeZone) {
					reasons = append(reasons, "start_tz_ne_end_tz")
				}
				if defaultTZ != "" && !strings.EqualFold(ev.Start.TimeZone, defaultTZ) {
					reasons = append(reasons, "start_tz_ne_default")
				}
				if len(reasons) == 0 {
					continue
				}
				rows = append(rows, finding{
					ID:      ev.ID,
					Subject: ev.Subject,
					Start:   ev.Start.DateTime,
					End:     ev.End.DateTime,
					StartTZ: ev.Start.TimeZone,
					EndTZ:   ev.End.TimeZone,
					Reason:  strings.Join(reasons, ","),
					WebLink: ev.WebLink,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Start < rows[j].Start })

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(w, "No timezone mismatches.")
				return nil
			}
			fmt.Fprintf(w, "%d event(s) with TZ inconsistencies:\n", len(rows))
			for _, r := range rows {
				fmt.Fprintf(w, "  %s  [%s/%s]  %s\n", r.Start, r.StartTZ, r.EndTZ, r.Subject)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
