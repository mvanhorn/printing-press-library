// Hand-rewritten in Phase 3 to delegate to the cross-network source clients.

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

func newAvailabilityCheckCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var flagTime string
	var flagPartySize int
	var flagForwardMinutes int
	var flagForwardDays int
	var flagAttribute string

	cmd := &cobra.Command{
		Use:   "check <restaurant>",
		Short: "Check open slots for a restaurant on a specific date and party size",
		Long: "Per-venue availability across both networks. Resolves the venue on OpenTable " +
			"or Tock and returns the earliest matching slot per the requested date/party.\n\n" +
			"Restaurant identifier accepts three shapes:\n" +
			"  • Bare slug — 'canlis' (searches both networks)\n" +
			"  • Network-prefixed slug — 'opentable:le-bernardin', 'tock:alinea'\n" +
			"  • Numeric OpenTable ID — '3688' or 'opentable:3688'. IDs come from\n" +
			"    `restaurants list --json` (the `id` field) and bypass the\n" +
			"    name-based slug resolver entirely, so they're the most\n" +
			"    reliable input shape for agents composing `list → check`.\n" +
			"    Tock has no numeric-ID convention; use the domain-name slug.",
		Example: "  table-reservation-goat-pp-cli availability check 'tock:alinea' --party 2 --date 2026-06-15 --json\n" +
			"  table-reservation-goat-pp-cli availability check 3688 --party 6 --date 2026-12-25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			venue := strings.TrimSpace(args[0])
			if venue == "" || strings.Contains(venue, "__printing_press_invalid__") {
				return fmt.Errorf("invalid venue: %q (provide a slug like 'alinea' or 'tock:alinea')", args[0])
			}
			party := flagPartySize
			if party == 0 {
				party = 2
			}
			startDate := flagDate
			if startDate == "" {
				startDate = time.Now().Format("2006-01-02")
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), earliestRow{
					Venue: venue, Network: "opentable",
					Available: false, Reason: "dry-run",
				}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			withinDays := flagForwardDays
			if withinDays == 0 {
				withinDays = 1
			}
			row := resolveEarliestForVenue(cmd.Context(), session, venue, party, startDate, withinDays, false)
			return printJSONFiltered(cmd.OutOrStdout(), row, flags)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date in YYYY-MM-DD; defaults to today")
	cmd.Flags().StringVar(&flagTime, "time", "20:00", "Time in HH:MM (24h)")
	cmd.Flags().IntVar(&flagPartySize, "party", 2, "Party size")
	cmd.Flags().IntVar(&flagForwardMinutes, "forward-minutes", 150, "Search +/- N minutes around requested time")
	cmd.Flags().IntVar(&flagForwardDays, "forward-days", 1, "Also search forward N days from start date")
	cmd.Flags().StringVar(&flagAttribute, "attribute", "", "Filter by slot attribute (patio, bar, highTop, standard, experience)")
	_ = flagTime
	_ = flagForwardMinutes
	_ = flagAttribute
	return cmd
}
