// Hand-built (Phase 3): weekly review — what changed in my calendar since X.
// Approximates "added / rescheduled / cancelled / RSVP-changed" by reading
// synced_at + created/last_modified timestamps from the events table. A more
// precise diff is possible by retaining pre-sync snapshots; this v1 covers the
// common "what changed since I last synced" question.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newReviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Diff: what was added, rescheduled, cancelled, or had its RSVP change since the given time",
		Long: `Review reads synced_at, created_date_time, and last_modified_date_time on each
event row and emits buckets:

  added       — created and first synced after --since
  rescheduled — last_modified after --since but created before
  cancelled   — isCancelled=true and last_modified after --since
  rsvp_change — responseStatus.time after --since

Pair with 'sync' for an end-to-end "what changed this week" view.`,
		Example: strings.Trim(`
  outlook-calendar-pp-cli review --since 7d --json
  outlook-calendar-pp-cli review --since 2026-05-03 --json
`, "\n"),
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

			now := time.Now()
			anchor, err := resolveSince(since, now, db)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", since, err))
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data, synced_at, is_cancelled, created_date_time, last_modified_date_time FROM events`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			type changeRow struct {
				ID           string `json:"id"`
				Subject      string `json:"subject"`
				Start        string `json:"start"`
				Reason       string `json:"reason"`
				Detail       string `json:"detail,omitempty"`
				LastModified string `json:"last_modified,omitempty"`
				WebLink      string `json:"web_link,omitempty"`
			}

			type result struct {
				Since       string      `json:"since"`
				Added       []changeRow `json:"added"`
				Rescheduled []changeRow `json:"rescheduled"`
				Cancelled   []changeRow `json:"cancelled"`
				RSVPChanged []changeRow `json:"rsvp_changed"`
			}
			out := result{Since: anchor.UTC().Format(time.RFC3339)}

			for rows.Next() {
				var (
					id        string
					data      []byte
					synced    string
					cancelled int
					created   string
					lastMod   string
				)
				if err := rows.Scan(&id, &data, &synced, &cancelled, &created, &lastMod); err != nil {
					return apiErr(err)
				}
				ev, err := parseGraphEvent(data)
				if err != nil {
					continue
				}
				row := changeRow{
					ID:           id,
					Subject:      ev.Subject,
					Start:        ev.Start.DateTime,
					LastModified: lastMod,
					WebLink:      ev.WebLink,
				}
				createdTime := parseAnyTime(created)
				modTime := parseAnyTime(lastMod)
				rsvpTime := parseAnyTime(ev.ResponseStatus.Time)

				if cancelled == 1 && modTime.After(anchor) {
					row.Reason = "cancelled"
					out.Cancelled = append(out.Cancelled, row)
					continue
				}
				if !createdTime.IsZero() && createdTime.After(anchor) {
					row.Reason = "added"
					out.Added = append(out.Added, row)
					continue
				}
				if !modTime.IsZero() && modTime.After(anchor) {
					row.Reason = "rescheduled"
					row.Detail = "last_modified=" + lastMod
					out.Rescheduled = append(out.Rescheduled, row)
				}
				if !rsvpTime.IsZero() && rsvpTime.After(anchor) {
					rsvp := changeRow{
						ID:      id,
						Subject: ev.Subject,
						Start:   ev.Start.DateTime,
						Reason:  "rsvp_change",
						Detail:  "response=" + ev.ResponseStatus.Response,
					}
					out.RSVPChanged = append(out.RSVPChanged, rsvp)
				}
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			sortByStart := func(rs []changeRow) {
				sort.Slice(rs, func(i, j int) bool { return rs[i].Start < rs[j].Start })
			}
			sortByStart(out.Added)
			sortByStart(out.Rescheduled)
			sortByStart(out.Cancelled)
			sortByStart(out.RSVPChanged)

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, out, flags)
			}
			fmt.Fprintf(w, "Changes since %s:\n", out.Since)
			fmt.Fprintf(w, "  Added:        %d\n", len(out.Added))
			fmt.Fprintf(w, "  Rescheduled:  %d\n", len(out.Rescheduled))
			fmt.Fprintf(w, "  Cancelled:    %d\n", len(out.Cancelled))
			fmt.Fprintf(w, "  RSVP changed: %d\n", len(out.RSVPChanged))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "7d", "Look back N days, an ISO 8601 timestamp, 'last-sync', or 'today'")
	return cmd
}

func resolveSince(s string, anchor time.Time, db *store.Store) (time.Time, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "today":
		return time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location()), nil
	case "last-sync":
		// PATCH: --since last-sync reads store.GetLastSyncedAt("events") instead of hardcoding now-24h.
		if db != nil {
			if ts := db.GetLastSyncedAt("events"); ts != "" {
				if t := parseAnyTime(ts); !t.IsZero() {
					return t, nil
				}
			}
		}
		return anchor.Add(-24 * time.Hour), nil
	}
	if days, err := parseRelativeDays(s); err == nil {
		return anchor.AddDate(0, 0, -days), nil
	}
	return parseHumanTime(s, anchor)
}

func parseAnyTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.9999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
