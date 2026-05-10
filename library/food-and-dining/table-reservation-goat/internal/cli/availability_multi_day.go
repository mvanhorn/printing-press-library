// Hand-rewritten to delegate to the cross-network source clients.
// The generated scaffold called `client.Get("/availability/multi-day", params)`
// against opentable.com which doesn't exist as a REST endpoint; multi-day
// availability is built from per-day source-client calls (the OT GraphQL
// gateway is `forwardDays:0` only, so multi-day scans loop here just like
// `earliest.go` does).

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

type multiDayDayResult struct {
	Date    string      `json:"date"`
	Result  earliestRow `json:"result"`
}

type multiDayResponse struct {
	Venue     string              `json:"venue"`
	Party     int                 `json:"party"`
	StartDate string              `json:"start_date"`
	Days      int                 `json:"days"`
	Results   []multiDayDayResult `json:"results"`
	QueriedAt string              `json:"queried_at"`
}

func newAvailabilityMultiDayCmd(flags *rootFlags) *cobra.Command {
	var flagStartDate string
	var flagDays int
	var flagPartySize int

	cmd := &cobra.Command{
		Use:         "multi-day <restaurant>",
		Short:       "Multi-day availability for a single restaurant — per-day earliest-slot matrix",
		Example:     "  table-reservation-goat-pp-cli availability multi-day 'tock:canlis' --start-date 2026-05-15 --days 7 --party 2",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			venue := strings.TrimSpace(args[0])
			if venue == "" || strings.Contains(venue, "__printing_press_invalid__") {
				return fmt.Errorf("invalid venue: %q (provide a slug like 'canlis' or 'tock:canlis')", args[0])
			}
			if flagPartySize <= 0 {
				return fmt.Errorf("invalid --party %d: must be a positive integer", flagPartySize)
			}
			if flagDays <= 0 || flagDays > 14 {
				return fmt.Errorf("invalid --days %d: must be in [1, 14]", flagDays)
			}
			startDate := flagStartDate
			if startDate == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"start-date\" not set")
			}
			if startDate == "" {
				startDate = time.Now().UTC().Format("2006-01-02")
			}
			party := flagPartySize
			days := flagDays
			start, err := time.Parse("2006-01-02", startDate)
			if err != nil {
				return fmt.Errorf("invalid --start-date %q: %w", startDate, err)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), multiDayResponse{
					Venue: venue, Party: party, StartDate: startDate, Days: days,
					Results: []multiDayDayResult{{
						Date:   startDate,
						Result: earliestRow{Venue: venue, Network: "opentable", Available: false, Reason: "dry-run"},
					}},
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			rows := make([]multiDayDayResult, 0, days)
			for d := 0; d < days; d++ {
				dayStr := start.AddDate(0, 0, d).Format("2006-01-02")
				row := resolveEarliestForVenue(cmd.Context(), session, venue, party, dayStr, 1, flags.noCache)
				rows = append(rows, multiDayDayResult{Date: dayStr, Result: row})
			}
			return printJSONFiltered(cmd.OutOrStdout(), multiDayResponse{
				Venue: venue, Party: party, StartDate: startDate, Days: days,
				Results: rows, QueriedAt: time.Now().UTC().Format(time.RFC3339),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagStartDate, "start-date", "", "Start of date range (YYYY-MM-DD)")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Number of days to scan (default 7, max 14)")
	cmd.Flags().IntVar(&flagPartySize, "party", 2, "Party size (default 2)")
	return cmd
}
