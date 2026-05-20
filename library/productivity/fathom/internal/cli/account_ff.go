package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newAccountCmd(flags *rootFlags) *cobra.Command {
	var domain string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "account [domain]",
		Short: "Account relationship history — all meetings, topics, and action items for a company",
		Long: `View a complete, domain-keyed history of all interactions with a company:
every meeting, topics discussed (keyword frequency), open and closed action
items, and meeting cadence. Fathom's API is recording-centric; this command
gives you an account-centric lens.

Run 'sync --full' first to populate the local store.`,
		Example: strings.Trim(`
  fathom-pp-cli account acme.com
  fathom-pp-cli account --domain stripe.com
  fathom-pp-cli account notion.so --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && domain == "" {
				domain = args[0]
			}
			if domain == "" {
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

			var matched []fathomMeeting
			for _, m := range meetings {
				for _, inv := range m.CalendarInvitees {
					if strings.EqualFold(inv.EmailDomain, domain) {
						matched = append(matched, m)
						break
					}
				}
			}

			sort.Slice(matched, func(i, j int) bool {
				return matched[i].CreatedAt < matched[j].CreatedAt
			})

			// Aggregate action items and topics from AI summaries
			topicCounts := map[string]int{}
			var openActions, closedActions []string
			contactSet := map[string]string{} // email -> name

			for _, m := range matched {
				// Extract topics from Fathom's AI-generated summary sections rather
				// than doing raw word frequency (which produces stop words).
				if m.DefaultSummary != nil && m.DefaultSummary.MarkdownFormatted != nil {
					for _, topic := range extractSummaryTopics(*m.DefaultSummary.MarkdownFormatted) {
						topicCounts[topic]++
					}
				}
				for _, ai := range m.ActionItems {
					if ai.Completed {
						closedActions = append(closedActions, ai.Description)
					} else {
						openActions = append(openActions, ai.Description)
					}
				}
				for _, inv := range m.CalendarInvitees {
					if strings.EqualFold(inv.EmailDomain, domain) {
						contactSet[inv.Email] = inv.Name
					}
				}
			}

			// Top 10 topics by frequency
			type kv struct{ K string; V int }
			var topicList []kv
			for k, v := range topicCounts {
				topicList = append(topicList, kv{k, v})
			}
			sort.Slice(topicList, func(i, j int) bool { return topicList[i].V > topicList[j].V })
			if len(topicList) > 10 {
				topicList = topicList[:10]
			}

			type meetingSummary struct {
				Title   string `json:"title"`
				Date    string `json:"date"`
				URL     string `json:"url"`
				OpenAI  int    `json:"open_action_items"`
			}

			var meetList []meetingSummary
			for _, m := range matched {
				open := 0
				for _, ai := range m.ActionItems {
					if !ai.Completed {
						open++
					}
				}
				meetList = append(meetList, meetingSummary{
					Title:  m.meetingTitle(),
					Date:   func() string { if len(m.CreatedAt) >= 10 { return m.CreatedAt[:10] }; return m.CreatedAt }(), // PATCH(created-at-guard): guard against empty/short CreatedAt
					URL:    m.ShareURL,
					OpenAI: open,
				})
			}

			var contacts []string
			for email, name := range contactSet {
				contacts = append(contacts, fmt.Sprintf("%s <%s>", name, email))
			}
			sort.Strings(contacts)

			var topTopics []string
			for _, kv := range topicList {
				topTopics = append(topTopics, kv.K)
			}

			type accountResult struct {
				Domain        string         `json:"domain"`
				TotalMeetings int            `json:"total_meetings"`
				Contacts      []string       `json:"contacts"`
				TopTopics     []string       `json:"top_topics"`
				OpenActions   []string       `json:"open_action_items"`
				ClosedActions []string       `json:"closed_action_items"`
				Meetings      []meetingSummary `json:"meetings"`
			}

			result := accountResult{
				Domain:        domain,
				TotalMeetings: len(matched),
				Contacts:      contacts,
				TopTopics:     topTopics,
				OpenActions:   openActions,
				ClosedActions: closedActions,
				Meetings:      meetList,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if len(matched) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No meetings found with domain %s.\n", domain)
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'fathom-pp-cli sync --full' if the store is empty.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Account: %s  (%d meetings)\n\n", domain, len(matched))
			fmt.Fprintln(cmd.OutOrStdout(), "Contacts:")
			for _, c := range contacts {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", c)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Top topics discussed:")
			for _, t := range topTopics {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", t)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if len(openActions) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Open action items (%d):\n", len(openActions))
				for _, ai := range openActions {
					fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", ai)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Meeting history (oldest first):")
			for _, m := range meetList {
				openLabel := ""
				if m.OpenAI > 0 {
					openLabel = fmt.Sprintf("  [%d open action items]", m.OpenAI)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s%s\n", m.Date, m.Title, openLabel)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Company email domain (e.g. acme.com)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
