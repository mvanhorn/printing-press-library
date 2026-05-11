package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

// PATCH(novel-commands): cross-meeting intelligence commands reading from local SQLite store.
func newCommitmentsCmd(flags *rootFlags) *cobra.Command {
	var assignee string
	var since string
	var includeCompleted bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "commitments",
		Short: "Cross-meeting action item tracker — see every open commitment across all calls",
		Long: `Query all action items from synced meetings and surface what was promised,
by whom, and whether it's done. Groups by meeting, filterable by assignee
email. Run 'sync' first to populate the local store.`,
		Example: strings.Trim(`
  fathom-pp-cli commitments
  fathom-pp-cli commitments --assignee me
  fathom-pp-cli commitments --since 30d --json
  fathom-pp-cli commitments --include-completed --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff, err := parseSince(since)
			if err != nil {
				return err
			}

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			type commitment struct {
				MeetingTitle string  `json:"meeting_title"`
				MeetingDate  string  `json:"meeting_date"`
				MeetingURL   string  `json:"meeting_url"`
				Description  string  `json:"description"`
				AssigneeName string  `json:"assignee_name"`
				AssigneeEmail string `json:"assignee_email"`
				Completed    bool    `json:"completed"`
				Timestamp    string  `json:"timestamp"`
				PlaybackURL  string  `json:"playback_url"`
			}

			results := make([]commitment, 0)
			for _, m := range meetings {
				if len(m.ActionItems) == 0 {
					continue
				}
				// Apply date filter
				if !cutoff.IsZero() {
					t, err := parseFlexTime(m.CreatedAt)
					if err != nil || t.Before(cutoff) {
						continue
					}
				}
				for _, ai := range m.ActionItems {
					if !includeCompleted && ai.Completed {
						continue
					}
					// Apply assignee filter
					if assignee != "" && assignee != "me" {
						if ai.Assignee == nil || !strings.EqualFold(ai.Assignee.Email, assignee) {
							continue
						}
					}
					c := commitment{
						MeetingTitle: m.meetingTitle(),
						MeetingDate:  m.CreatedAt,
						MeetingURL:   m.ShareURL,
						Description:  ai.Description,
						Completed:    ai.Completed,
						Timestamp:    ai.RecordingTimestamp,
						PlaybackURL:  ai.RecordingPlayback,
					}
					if ai.Assignee != nil {
						c.AssigneeName = ai.Assignee.Name
						c.AssigneeEmail = ai.Assignee.Email
					}
					results = append(results, c)
				}
			}

			// Sort by meeting date descending
			sort.Slice(results, func(i, j int) bool {
				return results[i].MeetingDate > results[j].MeetingDate
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			// Human output
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open commitments found. Run 'fathom-pp-cli sync --full' if the store is empty.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Open commitments: %d\n\n", len(results))
			for _, c := range results {
				status := "[ ]"
				if c.Completed {
					status = "[x]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", status, c.Description)
				if c.AssigneeName != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    Assignee: %s <%s>\n", c.AssigneeName, c.AssigneeEmail)
				}
				dateStr := c.MeetingDate
				if len(dateStr) > 10 {
					dateStr = dateStr[:10]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "    Meeting:  %s (%s)\n", c.MeetingTitle, dateStr)
				if c.PlaybackURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    Watch:    %s\n", c.PlaybackURL)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee email (use 'me' for the recorded_by user)")
	cmd.Flags().StringVar(&since, "since", "", "Only include meetings since this duration (e.g. 30d, 4w, 3m)")
	cmd.Flags().BoolVar(&includeCompleted, "include-completed", false, "Include completed action items (default: open only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.fathom-pp-cli/data.db)")
	return cmd
}
