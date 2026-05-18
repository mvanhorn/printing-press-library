// Hand-built (Phase 3): per-attendee co-occurrence — count events shared
// with a person and the most-recent N.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newWithCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var recent int

	cmd := &cobra.Command{
		Use:   "with [email]",
		Short: "How often have I met with this person, and when did I see them last? Counts and recent N events from local store",
		Example: strings.Trim(`
  outlook-calendar-pp-cli with alice@example.com --since 90d --json
  outlook-calendar-pp-cli with alice@example.com --since 365d --recent 10 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.ToLower(strings.TrimSpace(args[0]))
			if needle == "" {
				return usageErr(fmt.Errorf("provide an email address as the positional argument"))
			}
			days, err := parseRelativeDays(since)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", since, err))
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
			start := now.AddDate(0, 0, -days)
			events, err := loadEvents(cmd.Context(), db.DB(), start, now.AddDate(0, 0, 365))
			if err != nil {
				return apiErr(err)
			}

			type recentRow struct {
				ID       string `json:"id"`
				Subject  string `json:"subject"`
				Start    string `json:"start"`
				Location string `json:"location"`
				Response string `json:"response"`
			}
			type result struct {
				Email      string      `json:"email"`
				Count      int         `json:"count"`
				LastSeen   string      `json:"last_seen,omitempty"`
				FirstSeen  string      `json:"first_seen,omitempty"`
				WindowDays int         `json:"window_days"`
				Recent     []recentRow `json:"recent"`
			}

			matches := []recentRow{}
			lastSeen := ""
			firstSeen := ""
			for _, ev := range events {
				match := false
				if strings.ToLower(ev.Organizer.EmailAddress.Address) == needle {
					match = true
				} else {
					for _, a := range ev.Attendees {
						if a.Email == needle {
							match = true
							break
						}
					}
				}
				if !match {
					continue
				}
				row := recentRow{
					ID:       ev.ID,
					Subject:  ev.Subject,
					Start:    ev.Start.DateTime,
					Location: ev.Location,
					Response: ev.ResponseStatus.Response,
				}
				matches = append(matches, row)
				if firstSeen == "" || row.Start < firstSeen {
					firstSeen = row.Start
				}
				if row.Start > lastSeen {
					lastSeen = row.Start
				}
			}
			// PATCH: capture total match count before --recent truncation so Count reports all shared events, not the displayed slice.
			totalCount := len(matches)
			sort.Slice(matches, func(i, j int) bool { return matches[i].Start > matches[j].Start })
			if recent > 0 && len(matches) > recent {
				matches = matches[:recent]
			}

			out := result{
				Email:      needle,
				Count:      totalCount,
				LastSeen:   lastSeen,
				FirstSeen:  firstSeen,
				WindowDays: days,
				Recent:     matches,
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, out, flags)
			}
			fmt.Fprintf(w, "%s — %d shared event(s) in the last %d days\n", needle, out.Count, days)
			if out.LastSeen != "" {
				fmt.Fprintf(w, "Last seen: %s\n", out.LastSeen)
			}
			for _, r := range out.Recent {
				fmt.Fprintf(w, "  %s  %s\n", r.Start, r.Subject)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "90d", "Look back N days (e.g. 30d, 90d, 365d)")
	cmd.Flags().IntVar(&recent, "recent", 10, "Maximum number of recent events to include in output (0 = all)")
	return cmd
}
