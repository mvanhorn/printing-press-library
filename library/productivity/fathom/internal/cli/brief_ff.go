package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newBriefCmd(flags *rootFlags) *cobra.Command {
	var domain string
	var email string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Pre-call account brief — full meeting history with a person or company",
		Long: `Pull all prior meetings with a specific person (by email) or company (by
domain) from the local store. Shows chronological history with topics
discussed, open action items, and last contact date.

Run 'sync --full' first to populate the local store.`,
		Example: strings.Trim(`
  fathom-pp-cli brief --domain acme.com
  fathom-pp-cli brief --email jane@acme.com
  fathom-pp-cli brief --domain stripe.com --limit 5 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && domain == "" && email == "" {
				arg := args[0]
				if strings.Contains(arg, "@") {
					email = arg
				} else {
					domain = arg
				}
			}
			if domain == "" && email == "" {
				return cmd.Help()
			}
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

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			// Filter meetings by participant email or domain
			var matched []fathomMeeting
			for _, m := range meetings {
				for _, inv := range m.CalendarInvitees {
					if domain != "" && strings.EqualFold(inv.EmailDomain, domain) {
						matched = append(matched, m)
						break
					}
					if email != "" && strings.EqualFold(inv.Email, email) {
						matched = append(matched, m)
						break
					}
				}
			}

			// Sort by date descending (most recent first)
			sort.Slice(matched, func(i, j int) bool {
				return matched[i].CreatedAt > matched[j].CreatedAt
			})

			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}

			type meetingBrief struct {
				Title       string   `json:"title"`
				Date        string   `json:"date"`
				URL         string   `json:"url"`
				Participants []string `json:"participants"`
				SummarySnippet *string `json:"summary_snippet,omitempty"`
				OpenActions  []string `json:"open_action_items"`
			}

			results := make([]meetingBrief, 0)
			for _, m := range matched {
				var participants []string
				for _, inv := range m.CalendarInvitees {
					participants = append(participants, fmt.Sprintf("%s <%s>", inv.Name, inv.Email))
				}
				var open []string
				for _, ai := range m.ActionItems {
					if !ai.Completed {
						open = append(open, ai.Description)
					}
				}
				var snippet *string
				if m.DefaultSummary != nil && m.DefaultSummary.MarkdownFormatted != nil {
					s := *m.DefaultSummary.MarkdownFormatted
					if len(s) > 400 {
						s = s[:400] + "…"
					}
					snippet = &s
				}
				results = append(results, meetingBrief{
					Title:          m.meetingTitle(),
					Date:           func() string { if len(m.CreatedAt) >= 10 { return m.CreatedAt[:10] }; return m.CreatedAt }(), // PATCH(created-at-guard): guard against empty/short CreatedAt
					URL:            m.ShareURL,
					Participants:   participants,
					SummarySnippet: snippet,
					OpenActions:    open,
				})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			// Human output
			target := domain
			if email != "" {
				target = email
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No meetings found with %s. Run 'fathom-pp-cli sync --full' if the store is empty.\n", target)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Meeting history with %s (%d meetings)\n\n", target, len(results))
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "── %s  %s\n", r.Date, r.Title)
				fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", r.URL)
				if r.SummarySnippet != nil {
					lines := strings.Split(*r.SummarySnippet, "\n")
					for _, line := range lines[:min(3, len(lines))] {
						if strings.TrimSpace(line) != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", line)
						}
					}
				}
				if len(r.OpenActions) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "   Open action items:")
					for _, ai := range r.OpenActions {
						fmt.Fprintf(cmd.OutOrStdout(), "     • %s\n", ai)
					}
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Filter by participant email domain (e.g. acme.com)")
	cmd.Flags().StringVar(&email, "email", "", "Filter by specific participant email")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of meetings to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
