// Hand-rewritten in Phase 3 to delegate to the cross-network source clients.

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.
// PATCH: location-native-redesign — U7 wires `availability check` through
// the typed ResolveLocation pipeline. The --location flag supersedes the
// untyped (and previously absent on this command) location hint; --metro
// is added as a deprecated legacy alias to match `restaurants list`'s
// R12 backward-compat contract. The numeric-ID short-circuit is exempt
// from the post-filter hard-reject: an agent that passes a numeric OT
// ID knew exactly which venue it wanted, so out-of-radius numeric IDs
// get a soft-demote (LocationWarning) rather than a drop.

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
	var flagLocation string
	var flagAcceptAmbiguous bool
	var flagMetro string

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
			"    Tock has no numeric-ID convention; use the domain-name slug.\n\n" +
			"Use `--location <city>` to anchor the OT Autocomplete fallback on " +
			"a specific metro centroid (e.g., `--location 'seattle'` for a " +
			"`canlis` query). Without `--location`, the resolver anchors on " +
			"NYC. Numeric-ID inputs are exempt from the radius hard-reject — " +
			"out-of-radius numeric IDs return with a `location_warning` " +
			"rather than being dropped.",
		Example: "  table-reservation-goat-pp-cli availability check 'tock:alinea' --party 2 --date 2026-06-15 --json\n" +
			"  table-reservation-goat-pp-cli availability check 'canlis' --location 'seattle' --party 4 --agent\n" +
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
			withinDays := flagForwardDays
			if withinDays == 0 {
				withinDays = 1
			}

			// Resolve --location / --metro into a typed GeoContext (or a
			// disambiguation envelope) before any provider call. The
			// resolved GeoContext flows into resolveEarliestForVenue and
			// drives the OT Autocomplete coordinate hint in place of the
			// hardcoded NYC fallback.
			gc, envelope, locationErr, acceptedAmbiguous := resolveLocationFlags(
				cmd.ErrOrStderr(),
				flagLocation,
				flagMetro,
				flagAcceptAmbiguous,
			)
			if locationErr != nil {
				return locationErr
			}
			if envelope != nil {
				// Disambiguation envelope replaces the result row entirely.
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			if dryRunOK(flags) {
				row := earliestRow{
					Venue: venue, Network: "opentable",
					Available: false, Reason: "dry-run",
				}
				row = applyGeoToVenueRow(row, gc, acceptedAmbiguous, venue)
				return printJSONFiltered(cmd.OutOrStdout(), row, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			row := resolveEarliestForVenue(cmd.Context(), session, venue, party, startDate, withinDays, false, gc)
			row = applyGeoToVenueRow(row, gc, acceptedAmbiguous, venue)
			return printJSONFiltered(cmd.OutOrStdout(), row, flags)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date in YYYY-MM-DD; defaults to today")
	cmd.Flags().StringVar(&flagTime, "time", "20:00", "Time in HH:MM (24h)")
	cmd.Flags().IntVar(&flagPartySize, "party", 2, "Party size")
	cmd.Flags().IntVar(&flagForwardMinutes, "forward-minutes", 150, "Search +/- N minutes around requested time")
	cmd.Flags().IntVar(&flagForwardDays, "forward-days", 1, "Also search forward N days from start date")
	cmd.Flags().StringVar(&flagAttribute, "attribute", "", "Filter by slot attribute (patio, bar, highTop, standard, experience)")
	cmd.Flags().StringVar(&flagLocation, "location", "",
		"Free-form location: 'bellevue, wa', 'seattle', '47.6,-122.3', or 'seattle metro'. "+
			"Anchors the OT Autocomplete fallback on the resolved centroid; numeric-ID "+
			"inputs are exempt from the radius hard-reject and instead receive a "+
			"location_warning when out-of-radius.")
	cmd.Flags().BoolVar(&flagAcceptAmbiguous, "batch-accept-ambiguous", false,
		"BATCH-ONLY escape hatch: when --location is ambiguous, force-pick the top "+
			"candidate instead of returning a disambiguation envelope. Interactive "+
			"agents must NOT set this — it defeats the disambiguation contract.")
	cmd.Flags().StringVar(&flagMetro, "metro", "",
		"Metro slug (e.g., chicago, seattle). DEPRECATED — use --location <city>. "+
			"Implicit --batch-accept-ambiguous is canonical-only: a single-hit registry lookup "+
			"preserves the legacy result shape; ambiguous or unknown values return the standard "+
			"disambiguation envelope just like --location would.")
	_ = flagTime
	_ = flagForwardMinutes
	_ = flagAttribute
	return cmd
}
