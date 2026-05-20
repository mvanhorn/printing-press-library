package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var domain string
	var months int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Engagement velocity — track whether your meeting cadence with a company is accelerating or stalling",
		Long: `Group meetings with a specific external domain by calendar month and
show whether your engagement cadence is increasing, stable, or declining.
A useful pipeline health signal before deals go cold.

Run 'sync --full' first to populate the local store.`,
		Example: strings.Trim(`
  fathom-pp-cli velocity --domain acme.com
  fathom-pp-cli velocity --domain stripe.com --months 6
  fathom-pp-cli velocity --domain notion.so --agent`, "\n"),
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

			if months <= 0 {
				months = 6
			}
			cutoff, _ := parseSince(fmt.Sprintf("%dd", months*30))

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			// Count meetings per month for this domain
			monthCounts := map[string]int{}
			for _, m := range meetings {
				t, err := parseFlexTime(m.CreatedAt)
				if err != nil || (!cutoff.IsZero() && t.Before(cutoff)) {
					continue
				}
				found := false
				for _, inv := range m.CalendarInvitees {
					if strings.EqualFold(inv.EmailDomain, domain) {
						found = true
						break
					}
				}
				if found {
					monthCounts[calMonth(t)]++
				}
			}

			// Build sorted months list (include zero-count months in range)
			type monthEntry struct {
				Month string `json:"month"`
				Count int    `json:"count"`
			}

			var entries []monthEntry
			for m, c := range monthCounts {
				entries = append(entries, monthEntry{Month: m, Count: c})
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Month < entries[j].Month
			})

			// Compute trend: compare first half vs second half
			trend := "stable"
			if len(entries) >= 2 {
				mid := len(entries) / 2
				var firstSum, secondSum int
				for _, e := range entries[:mid] {
					firstSum += e.Count
				}
				for _, e := range entries[mid:] {
					secondSum += e.Count
				}
				if secondSum > firstSum {
					trend = "accelerating"
				} else if secondSum < firstSum {
					trend = "stalling"
				}
			}
			if len(entries) == 0 {
				trend = "no-data"
			}

			type velocityResult struct {
				Domain  string       `json:"domain"`
				Trend   string       `json:"trend"`
				Months  []monthEntry `json:"months"`
				Total   int          `json:"total_meetings"`
			}

			total := 0
			for _, e := range entries {
				total += e.Count
			}

			result := velocityResult{
				Domain: domain,
				Trend:  trend,
				Months: entries,
				Total:  total,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Human output
			trendIcon := "→"
			switch trend {
			case "accelerating":
				trendIcon = "↑"
			case "stalling":
				trendIcon = "↓"
			case "no-data":
				trendIcon = "?"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Engagement velocity: %s  %s %s\n\n", domain, trendIcon, trend)
			if len(entries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No meetings found with %s in the last %d months.\n", domain, months)
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'fathom-pp-cli sync --full' if the store is empty.")
				return nil
			}
			maxCount := 0
			for _, e := range entries {
				if e.Count > maxCount {
					maxCount = e.Count
				}
			}
			for _, e := range entries {
				bar := strings.Repeat("█", e.Count)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s %d\n", e.Month, bar, e.Count)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d meetings over %d months\n", total, len(entries))
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "External participant email domain (e.g. acme.com)")
	cmd.Flags().IntVar(&months, "months", 6, "Number of months to look back")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
