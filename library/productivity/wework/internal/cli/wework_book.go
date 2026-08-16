// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) headless desk booking for WeWork-owned locations.
// Given a city + location (id or name) + date, resolves the full booking chain
// (get-locations-by-geo -> get-spaces WeWorkSpaceID -> inventory-details SpaceID
// -> get-user-cards CardUuid) and posts the create. Defaults to a preview;
// --confirm places the real (paid) booking.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/client"
	"github.com/spf13/cobra"
)

func newBookCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagLocationID, flagLocation, flagDate, flagStart, flagEnd string
	var flagConfirm bool
	cmd := &cobra.Command{
		Use:   "book",
		Short: "Book a desk at a WeWork-owned location (headless; --confirm to charge)",
		Long: "Books a day-desk end to end from a city + location + date — no browser. Resolves the\n" +
			"location's booking identifiers automatically (see `locations`). Prints a preview by\n" +
			"default; pass --confirm to place the REAL booking (charges your saved card).",
		Example: strings.Trim(`
  wework-pp-cli book --city "Austin, TX" --location "Barton Springs" --date 2026-08-18
  wework-pp-cli book --city "Austin, TX" --location-id e0317ab1-39a8-4024-ae12-6260b5470295 --date 2026-08-18 --confirm`, "\n"),
		Annotations: map[string]string{}, // mutating: not read-only
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagCity) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--city is required"))
			}
			if flagLocationID == "" && strings.TrimSpace(flagLocation) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide --location-id or --location (name)"))
			}
			date := strings.TrimSpace(flagDate)
			if date == "" {
				date = todayLocalDate()
			}
			if flagStart == "" {
				flagStart = "08:30"
			}
			if flagEnd == "" {
				flagEnd = "17:00"
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			lat, lng, matchedCity, err := resolveCityGeo(ctx, c, flagCity)
			if err != nil {
				return err
			}
			buildings, err := resolveWeworkBuildings(ctx, c, lat, lng, cityNameOnly(matchedCity), date)
			if err != nil {
				return err
			}
			b, err := pickBuilding(buildings, flagLocationID, flagLocation)
			if err != nil {
				return err
			}
			if !b.Bookable || b.WeWorkSpaceID == "" {
				return fmt.Errorf("location %q has no bookable desk for %s (WeWorkSpaceID unresolved)", b.Name, date)
			}

			// StartTime/EndTime in the building's timezone -> UTC.
			startUTC, endUTC, utcOffset, err := localToUTC(date, flagStart, flagEnd, b.TimeZone)
			if err != nil {
				return err
			}

			// SpaceID (kubeSpaceId): resolved during enrichment; fall back to a
			// direct inventory-details lookup if not already populated.
			spaceID := b.SpaceID
			if spaceID == "" {
				spaceID, err = resolveSpaceID(ctx, c, b, date, utcOffset)
				if err != nil {
					return err
				}
			}
			// CardUuid via get-user-cards (default card).
			cardUUID, err := resolveDefaultCard(ctx, c)
			if err != nil {
				return err
			}

			payload := buildBookingPayload(b, spaceID, cardUUID, startUTC, endUTC, utcOffset, date, flagStart, flagEnd)

			w := cmd.OutOrStdout()
			if !flagConfirm {
				out := map[string]any{
					"preview": true, "location": b.Name, "locationId": b.LocationID, "date": date,
					"start": flagStart, "end": flagEnd, "price": b.PriceAmount, "currency": orDefault(b.Currency, "USD"),
					"payload": payload,
				}
				if flags.asJSON {
					return printJSONFiltered(w, out, flags)
				}
				fmt.Fprintf(w, "Preview: %s on %s, %s–%s — $%.0f\n", b.Name, date, flagStart, flagEnd, b.PriceAmount)
				fmt.Fprintf(w, "  locationId: %s\n  weWorkSpaceId: %s\n  spaceId: %s\n", b.LocationID, b.WeWorkSpaceID, spaceID)
				fmt.Fprintf(w, "\nThis is a preview. Re-run with --confirm to place the real booking (charges your saved card).\n")
				return nil
			}

			raw, _, err := c.Post(ctx, "/common-booking/", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				BookingStatus string `json:"BookingStatus"`
				Errors        []any  `json:"Errors"`
				ReservationID string `json:"ReservationID"`
				WeworkUUID    string `json:"WeworkUUID"`
			}
			_ = json.Unmarshal(raw, &resp)
			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"booked": resp.BookingStatus == "BookingSuccess", "status": resp.BookingStatus,
					"reservationId": resp.ReservationID, "location": b.Name, "date": date, "price": b.PriceAmount,
				}, flags)
			}
			if resp.BookingStatus != "BookingSuccess" {
				return fmt.Errorf("booking failed: status=%q errors=%v", resp.BookingStatus, resp.Errors)
			}
			fmt.Fprintf(w, "Booked: %s on %s, %s–%s. Reservation %s. Charged $%.0f.\n", b.Name, date, flagStart, flagEnd, resp.ReservationID, b.PriceAmount)
			fmt.Fprintf(w, "Cancel with: wework-pp-cli cancel --reservation-id %s --confirm\n", resp.ReservationID)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City of the location, e.g. \"Austin, TX\" (required)")
	cmd.Flags().StringVar(&flagLocationID, "location-id", "", "Exact building locationId (from `locations`)")
	cmd.Flags().StringVar(&flagLocation, "location", "", "Location name substring, e.g. \"Barton Springs\"")
	cmd.Flags().StringVar(&flagDate, "date", "", "Booking date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&flagStart, "start", "", "Start time HH:MM local (default 08:30)")
	cmd.Flags().StringVar(&flagEnd, "end", "", "End time HH:MM local (default 17:00)")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Place the REAL booking (charges your card). Without it, preview only.")
	return cmd
}

func pickBuilding(buildings []weworkBuilding, id, name string) (weworkBuilding, error) {
	if id != "" {
		for _, b := range buildings {
			if b.LocationID == id {
				return b, nil
			}
		}
		return weworkBuilding{}, fmt.Errorf("no location with id %q in this city (list them with `locations`)", id)
	}
	n := strings.ToLower(strings.TrimSpace(name))
	var matches []weworkBuilding
	for _, b := range buildings {
		if strings.Contains(strings.ToLower(b.Name+" "+b.Line1), n) {
			matches = append(matches, b)
		}
	}
	switch len(matches) {
	case 0:
		return weworkBuilding{}, fmt.Errorf("no location matched %q (list them with `locations --city ... --filter %s`)", name, name)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return weworkBuilding{}, fmt.Errorf("%q matched %d locations (%s) — narrow it or use --location-id", name, len(matches), strings.Join(names, "; "))
	}
}

func resolveSpaceID(ctx context.Context, c *client.Client, b weworkBuilding, date, utcOffset string) (string, error) {
	start, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("bad date %q: %w", date, err)
	}
	params := map[string]string{
		"propertyType": formatCLIParamValue(b.LocationType), "propertyId": b.LocationID, "spaceType": "0",
		"startDate": start.Format("01/02/2006"), "endDate": "", "duration": "0", "roomTypeFilter": "",
		"locationOffset": utcOffset, "capacity": "0", "limit": "0", "offset": "0", "floorId": "0",
		"spaceId": b.WeWorkSpaceID, "useInventoryUuid": "true", "platFormType": "1", "applicationType": "WorkplaceOne",
	}
	raw, err := c.Get(ctx, "/common-booking/inventory-details", params)
	if err != nil {
		return "", classifyAPIError(err, nil)
	}
	var inv struct {
		KubeSpaceID json.Number `json:"kubeSpaceId"`
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return "", fmt.Errorf("parsing inventory-details: %w", err)
	}
	if inv.KubeSpaceID.String() == "" || inv.KubeSpaceID.String() == "0" {
		return "", fmt.Errorf("could not resolve SpaceID (kubeSpaceId) for %s", b.Name)
	}
	return inv.KubeSpaceID.String(), nil
}

func resolveDefaultCard(ctx context.Context, c *client.Client) (string, error) {
	raw, err := c.Get(ctx, "/payments/get-user-cards", nil)
	if err != nil {
		return "", classifyAPIError(err, nil)
	}
	// Response is wrapped as {"data": [...]}; also tolerate a bare array or a
	// {"cards": [...]} wrapper.
	type card struct {
		UUID      string `json:"uuid"`
		IsDefault bool   `json:"is_default"`
		IsValid   bool   `json:"is_valid"`
	}
	var cards []card
	if json.Unmarshal(raw, &cards) != nil || len(cards) == 0 {
		var wrap struct {
			Data  []card `json:"data"`
			Cards []card `json:"cards"`
		}
		if json.Unmarshal(raw, &wrap) == nil {
			if len(wrap.Data) > 0 {
				cards = wrap.Data
			} else if len(wrap.Cards) > 0 {
				cards = wrap.Cards
			}
		}
	}
	if len(cards) == 0 {
		return "", fmt.Errorf("no saved payment card found (add one at members.wework.com)")
	}
	for _, c := range cards {
		if c.IsDefault && c.UUID != "" {
			return c.UUID, nil
		}
	}
	return cards[0].UUID, nil
}

// localToUTC converts a date + local HH:MM start/end in the building's IANA
// timezone to UTC RFC3339 strings and the offset (e.g. "-05:00").
func localToUTC(date, start, end, tz string) (startUTC, endUTC, offset string, err error) {
	loc, lerr := time.LoadLocation(tz)
	if lerr != nil || tz == "" {
		loc = time.UTC
	}
	parse := func(hhmm string) (time.Time, error) {
		return time.ParseInLocation("2006-01-02 15:04", date+" "+hhmm, loc)
	}
	st, err := parse(start)
	if err != nil {
		return "", "", "", fmt.Errorf("bad --start %q (want HH:MM): %w", start, err)
	}
	et, err := parse(end)
	if err != nil {
		return "", "", "", fmt.Errorf("bad --end %q (want HH:MM): %w", end, err)
	}
	_, offSec := st.Zone()
	sign := "+"
	if offSec < 0 {
		sign = "-"
		offSec = -offSec
	}
	offset = fmt.Sprintf("%s%02d:%02d", sign, offSec/3600, (offSec%3600)/60)
	return st.UTC().Format("2006-01-02T15:04:05Z"), et.UTC().Format("2006-01-02T15:04:05Z"), offset, nil
}

func buildBookingPayload(b weworkBuilding, spaceID, cardUUID, startUTC, endUTC, utcOffset, date, start, end string) map[string]any {
	tzName := "GMT " + utcOffset
	dayFmt := ""
	if t, err := time.Parse("2006-01-02", date); err == nil {
		dayFmt = t.Format("Monday, January 2")
	}
	return map[string]any{
		"ApplicationType":      "WorkplaceOne",
		"SpaceType":            4,
		"SpaceTypeID":          b.SpaceTypeID,
		"ReservationID":        "",
		"TriggerCalendarEvent": true,
		"LocationType":         b.LocationType,
		"UTCOffset":            utcOffset,
		"Currency":             orDefault(b.Currency, "USD"),
		"CardUuid":             cardUUID,
		"CreditCharged":        0,
		"LocationID":           b.LocationID,
		"SpaceID":              spaceID,
		"WeWorkSpaceID":        b.WeWorkSpaceID,
		"StartTime":            startUTC,
		"EndTime":              endUTC,
		"PlatFormTypeEnum":     1,
		"Notes": map[string]any{
			"locationAddress": b.Line1, "locationCity": b.City, "locationState": b.State,
			"locationCountry": orDefault(b.Country, "USA"), "locationName": b.Name,
		},
		"MailData": map[string]any{
			"dayFormatted": dayFmt, "startTimeFormatted": start, "endTimeFormatted": end,
			"locationAddress": b.Line1, "locationName": b.Name, "locationCity": b.City,
			"locationState": b.State, "locationCountry": orDefault(b.Country, "USA"),
			"creditsUsed": fmt.Sprintf("%.0f", b.PriceAmount), "Capacity": "1",
			"TimezoneUsed": tzName, "TimezoneIana": b.TimeZone,
			"startDateTime": date + " " + start, "endDateTime": date + " " + end,
			"workspaceType": "SharedWorkspace",
		},
	}
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
