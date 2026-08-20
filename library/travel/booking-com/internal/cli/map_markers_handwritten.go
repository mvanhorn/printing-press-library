// Hand-written replacement for newMapPromotedCmd. The generated version POSTs
// form-shaped params to /dml/graphql; booking.com's GraphQL endpoint requires
// a proper {operationName, query, variables} envelope plus the
// X-Booking-CSRF-Token header. This file wires that.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"

	"github.com/spf13/cobra"
)

func newMapMarkersHandwrittenCmd(flags *rootFlags) *cobra.Command {
	var destID int
	var checkin string
	var checkout string
	var adults int
	var rooms int
	var currency string

	cmd := &cobra.Command{
		Use:   "map",
		Short: "Map-marker data for a destination via Booking.com's internal GraphQL endpoint.",
		Long: "Fetch per-property latitude, longitude, summary price, and rating for every hotel " +
			"booking.com places on the map view of a destination. Requires a booking.com destination " +
			"id (look one up with 'destinations search <query>'). The request is a real " +
			"MapMarkersDesktop GraphQL operation against /dml/graphql, not the public search HTML, " +
			"so coordinates are accurate.",
		Example: "  booking-com-pp-cli map --dest-id -1456928 --checkin 2026-06-20 --checkout 2026-06-23 --adults 2 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "map.markers",
			"pp:method":     "POST",
			"pp:path":       "/dml/graphql",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if destID == 0 {
				return fmt.Errorf("--dest-id is required (run 'booking-com-pp-cli destinations search <query>' to look up an id)")
			}
			if checkin == "" || checkout == "" {
				return fmt.Errorf("--checkin and --checkout are required (YYYY-MM-DD)")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: fetch the search-results HTML to grab the live
			// b_csrf_token. Booking.com rotates this token on every
			// page load.
			csrfPath := "/searchresults.html"
			csrfParams := map[string]string{
				"ss":           fmt.Sprintf("dest_%d", destID),
				"dest_id":      fmt.Sprintf("%d", destID),
				"dest_type":    "city",
				"checkin":      checkin,
				"checkout":     checkout,
				"group_adults": fmt.Sprintf("%d", adults),
				"no_rooms":     fmt.Sprintf("%d", rooms),
			}
			htmlData, err := c.Get(csrfPath, csrfParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			csrf := booking.ExtractCSRFToken([]byte(htmlData))
			if csrf == "" {
				return fmt.Errorf("could not extract b_csrf_token from booking.com search HTML (booking.com may have shipped an anti-bot challenge — retry with 'doctor' to verify auth)")
			}

			// Step 2: build the GraphQL request body from the captured
			// operation and the user's params.
			body, err := booking.BuildMapMarkersRequest(booking.MapMarkersOptions{
				DestID:   destID,
				DestType: "CITY",
				Checkin:  checkin,
				Checkout: checkout,
				Adults:   adults,
				Rooms:    rooms,
				Currency: currency,
			})
			if err != nil {
				return err
			}

			// Step 3: POST to /dml/graphql with CSRF header. Use the
			// PostQuery variant so verify-mode does not short-circuit
			// the request (GraphQL queries ride a POST but are reads).
			data, _, err := c.PostQueryWithParamsAndHeaders(
				"/dml/graphql",
				map[string]string{"lang": "en-us"},
				json.RawMessage(body),
				booking.GraphQLHeaders(csrf),
			)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Step 4: parse the GraphQL response into the existing
			// MapMarker shape. ParseMapMarkers walks
			// data.searchQueries.search.results.
			data = extractResponseData(data)
			parsed, err := booking.ParseMapMarkers(data)
			if err != nil {
				return err
			}
			data = parsed

			prov := attachFreshness(DataProvenance{Source: "live"}, flags)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&destID, "dest-id", 0, "Booking.com destination id (use 'destinations search' to look one up). Required.")
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date YYYY-MM-DD. Required.")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date YYYY-MM-DD. Required.")
	cmd.Flags().IntVar(&adults, "adults", 2, "Adult guests.")
	cmd.Flags().IntVar(&rooms, "rooms", 1, "Rooms requested.")
	cmd.Flags().StringVar(&currency, "currency", "USD", "ISO currency code for prices.")
	return cmd
}
