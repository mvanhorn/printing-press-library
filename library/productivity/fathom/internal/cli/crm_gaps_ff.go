package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newCRMGapsCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "crm-gaps",
		Short: "CRM gap audit — find CRM-matched meetings where no action items were logged",
		Long: `Surface meetings that were matched to CRM contacts or deals but had no
action items recorded. These are calls that touched active accounts or
opportunities but left no paper trail — a common sales hygiene gap.

Requires meetings to have been synced with --include-crm-matches and
--include-action-items. Run 'sync --full' first.`,
		Example: strings.Trim(`
  fathom-pp-cli crm-gaps
  fathom-pp-cli crm-gaps --since 30d
  fathom-pp-cli crm-gaps --agent`, "\n"),
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

			type crmGapEntry struct {
				RecordingID  int64    `json:"recording_id"`
				Title        string   `json:"title"`
				Date         string   `json:"date"`
				URL          string   `json:"url"`
				CRMContacts  []string `json:"crm_contacts"`
				CRMDeals     []string `json:"crm_deals"`
				CRMCompanies []string `json:"crm_companies"`
			}

			results := make([]crmGapEntry, 0)
			for _, m := range meetings {
				if !cutoff.IsZero() {
					t, err := parseFlexTime(m.CreatedAt)
					if err != nil || t.Before(cutoff) {
						continue
					}
				}
				// Must have CRM matches
				if m.CRMMatches == nil {
					continue
				}
				hasCRM := len(m.CRMMatches.Contacts) > 0 || len(m.CRMMatches.Deals) > 0 || len(m.CRMMatches.Companies) > 0
				if !hasCRM {
					continue
				}
				// Must have NO action items (the gap)
				if len(m.ActionItems) > 0 {
					continue
				}

				var contacts, deals, companies []string
				for _, c := range m.CRMMatches.Contacts {
					contacts = append(contacts, fmt.Sprintf("%s <%s>", c.Name, c.Email))
				}
				for _, d := range m.CRMMatches.Deals {
					deals = append(deals, d.Name)
				}
				for _, co := range m.CRMMatches.Companies {
					companies = append(companies, co.Name)
				}

				results = append(results, crmGapEntry{
					RecordingID:  m.RecordingID,
					Title:        m.meetingTitle(),
					Date:         func() string { if len(m.CreatedAt) >= 10 { return m.CreatedAt[:10] }; return m.CreatedAt }(), // PATCH(created-at-guard): guard against empty/short CreatedAt
					URL:          m.ShareURL,
					CRMContacts:  contacts,
					CRMDeals:     deals,
					CRMCompanies: companies,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].Date > results[j].Date
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No CRM gaps found — all CRM-matched meetings have action items.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "CRM-matched meetings with no action items: %d\n\n", len(results))
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", r.Date, r.Title)
				if len(r.CRMDeals) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  Deals:    %s\n", strings.Join(r.CRMDeals, ", "))
				}
				if len(r.CRMCompanies) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  Companies: %s\n", strings.Join(r.CRMCompanies, ", "))
				}
				if len(r.CRMContacts) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  Contacts: %s\n", strings.Join(r.CRMContacts, ", "))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Watch:    %s\n", r.URL)
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only include meetings since this duration (e.g. 30d, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
