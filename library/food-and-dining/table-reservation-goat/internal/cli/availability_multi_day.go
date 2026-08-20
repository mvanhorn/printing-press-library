// Hand-rewritten to delegate to the cross-network source clients.
// The generated scaffold called `client.Get("/availability/multi-day", params)`
// against opentable.com which doesn't exist as a REST endpoint; multi-day
// availability is built from per-day source-client calls (the OT GraphQL
// gateway is `forwardDays:0` only, so multi-day scans loop here just like
// `earliest.go` does).

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.
// PATCH: location-native-redesign — U7 wires `availability multi-day` to
// the typed ResolveLocation pipeline alongside `availability check`. The
// location context applies once at resolve time (not per-day), so the
// LocationResolved / LocationWarning annotations sit on the top-level
// multiDayResponse rather than echoing per-row.

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

type multiDayDayResult struct {
	Date   string      `json:"date"`
	Result earliestRow `json:"result"`
}

type multiDayResponse struct {
	Venue     string              `json:"venue"`
	Party     int                 `json:"party"`
	StartDate string              `json:"start_date"`
	Days      int                 `json:"days"`
	Results   []multiDayDayResult `json:"results"`
	// LocationResolved / LocationWarning are the U7 typed-resolution
	// annotations. They sit at the top level (not per-day) because the
	// location applies to the venue resolution once, not to each day's
	// availability fetch. Both are omitempty so the no-constraint path
	// preserves the pre-U7 JSON shape.
	LocationResolved *LocationResolvedField `json:"location_resolved,omitempty"`
	LocationWarning  *LocationWarningField  `json:"location_warning,omitempty"`
	QueriedAt        string                 `json:"queried_at"`
}

func newAvailabilityMultiDayCmd(flags *rootFlags) *cobra.Command {
	var flagStartDate string
	var flagDays int
	var flagPartySize int
	var flagLocation string
	var flagAcceptAmbiguous bool
	var flagMetro string

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

			// Resolve --location / --metro once before any per-day fetch.
			// The resolved GeoContext flows into resolveEarliestForVenue
			// (anchoring the OT Autocomplete coordinate hint) and into
			// applyGeoToVenueRow (numeric-ID exemption + decoration).
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
				// Disambiguation envelope replaces the multi-day matrix
				// entirely — the caller must pick a location before the
				// per-day fetch makes sense.
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			if dryRunOK(flags) {
				dryRow := earliestRow{Venue: venue, Network: "opentable", Available: false, Reason: "dry-run"}
				// Apply decoration so the dry-run shape includes the
				// location_resolved annotation just like the live path.
				// The numeric-ID exemption branch is a no-op on dry-run
				// (no lat/lng populated).
				dryRow = applyGeoToVenueRow(dryRow, gc, acceptedAmbiguous, venue)
				resp := multiDayResponse{
					Venue: venue, Party: party, StartDate: startDate, Days: days,
					Results: []multiDayDayResult{{
						Date:   startDate,
						Result: dryRow,
					}},
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}
				resp.LocationResolved = dryRow.LocationResolved
				resp.LocationWarning = dryRow.LocationWarning
				// Strip the per-row decoration once it has been promoted
				// to the top-level so the per-day result stays compact.
				resp.Results[0].Result.LocationResolved = nil
				resp.Results[0].Result.LocationWarning = nil
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			rows := make([]multiDayDayResult, 0, days)
			for d := 0; d < days; d++ {
				dayStr := start.AddDate(0, 0, d).Format("2006-01-02")
				row := resolveEarliestForVenue(cmd.Context(), session, venue, party, dayStr, 1, flags.noCache, gc)
				// Per-row location decoration is dropped — the location
				// context applies once at resolve time, not per-day. The
				// numeric-ID warning is computed once below from the first
				// row that carries lat/lng.
				row.LocationResolved = nil
				row.LocationWarning = nil
				rows = append(rows, multiDayDayResult{Date: dayStr, Result: row})
			}
			resp := multiDayResponse{
				Venue: venue, Party: party, StartDate: startDate, Days: days,
				Results: rows, QueriedAt: time.Now().UTC().Format(time.RFC3339),
			}
			// Decorate once at the top level. Use a representative row
			// (the first day's result) to drive the numeric-ID
			// exemption's distance check — all days resolve the same
			// venue, so lat/lng is consistent across rows.
			var probe earliestRow
			if len(rows) > 0 {
				probe = rows[0].Result
			} else {
				probe = earliestRow{Venue: venue}
			}
			probe = applyGeoToVenueRow(probe, gc, acceptedAmbiguous, venue)
			resp.LocationResolved = probe.LocationResolved
			resp.LocationWarning = probe.LocationWarning
			return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
		},
	}
	cmd.Flags().StringVar(&flagStartDate, "start-date", "", "Start of date range (YYYY-MM-DD)")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Number of days to scan (default 7, max 14)")
	cmd.Flags().IntVar(&flagPartySize, "party", 2, "Party size (default 2)")
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
	return cmd
}
