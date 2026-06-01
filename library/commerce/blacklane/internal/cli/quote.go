// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel features (not generator output).
// PATCH: friendly quote engine — geocode via OSM Nominatim, POST /prices, render
// vehicle classes; plus compare/fit/trip transcendence commands built on it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// ---- Location resolution (OpenStreetMap Nominatim, free, no API key) ----

type geoPoint struct {
	Address     string  `json:"address"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	PlaceID     string  `json:"placeId,omitempty"`     // Blacklane placeId (when resolved natively)
	AirportIata string  `json:"airportIata,omitempty"` // IATA code for airports
}

// geocode resolves a free-text address to coordinates via OSM Nominatim.
func geocode(query string, timeout time.Duration) (geoPoint, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	endpoint := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return geoPoint{}, err
	}
	// Nominatim requires a descriptive User-Agent.
	req.Header.Set("User-Agent", "blacklane-pp-cli (https://github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return geoPoint{}, fmt.Errorf("geocoding %q: %w", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return geoPoint{}, fmt.Errorf("geocoding %q: OpenStreetMap returned HTTP %d", query, resp.StatusCode)
	}
	var hits []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil {
		return geoPoint{}, fmt.Errorf("geocoding %q: %w", query, err)
	}
	if len(hits) == 0 {
		return geoPoint{}, fmt.Errorf("no location found for %q — try a more specific address, or pass coordinates with --pickup-lat/--pickup-lng", query)
	}
	lat, errLat := strconv.ParseFloat(hits[0].Lat, 64)
	lng, errLng := strconv.ParseFloat(hits[0].Lon, 64)
	if errLat != nil || errLng != nil {
		return geoPoint{}, fmt.Errorf("geocoding %q: invalid coordinates in OpenStreetMap response", query)
	}
	return geoPoint{Address: hits[0].DisplayName, Latitude: lat, Longitude: lng}, nil
}

// resolveLocation turns an address string (or explicit lat/lng) into a geoPoint.
// coordsProvided must be true only when BOTH coordinates were explicitly given
// (the caller validates that), so lat=0 / lng=0 — the equator and prime meridian
// — are honored rather than mistaken for "unset".
func resolveLocation(label string, lat, lng float64, coordsProvided bool, timeout time.Duration) (geoPoint, error) {
	if coordsProvided {
		addr := label
		if addr == "" {
			addr = fmt.Sprintf("%.5f,%.5f", lat, lng)
		}
		return geoPoint{Address: addr, Latitude: lat, Longitude: lng}, nil
	}
	if strings.TrimSpace(label) == "" {
		return geoPoint{}, fmt.Errorf("empty location")
	}
	// Prefer Blacklane-native resolution — it returns the placeId + airportIata
	// that /prices uses for accurate fares. Fall back to OpenStreetMap if it
	// finds nothing or errors.
	if g, err := blacklaneResolve(label, timeout); err == nil {
		return g, nil
	}
	return geocode(label, timeout)
}

// ---- /prices request + response shapes ----

type pricesLocation struct {
	Address     string  `json:"address"`
	AirportIata *string `json:"airportIata"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	PlaceID     *string `json:"placeId"`
}

func locToPrices(g geoPoint) pricesLocation {
	var iata, pid *string
	if g.AirportIata != "" {
		iata = &g.AirportIata
	}
	if g.PlaceID != "" {
		pid = &g.PlaceID
	}
	return pricesLocation{Address: g.Address, AirportIata: iata, Latitude: g.Latitude, Longitude: g.Longitude, PlaceID: pid}
}

// quotePackage is the flattened, agent-friendly view of one priced vehicle class.
type quotePackage struct {
	PackageSlug string   `json:"packageSlug"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle"`
	Models      []string `json:"models,omitempty"`
	GrossAmount string   `json:"grossAmount"`
	Currency    string   `json:"currency"`
	MaxSeats    int      `json:"maxSeats"`
	MaxLuggage  int      `json:"maxLuggage"`
}

type quoteResult struct {
	ServiceType     string         `json:"serviceType"`
	DepartAt        string         `json:"departAt"`
	DurationSeconds int            `json:"durationSeconds,omitempty"`
	Pickup          geoPoint       `json:"pickup"`
	Dropoff         *geoPoint      `json:"dropoff,omitempty"`
	Packages        []quotePackage `json:"packages"`
	EstimatedKM     float64        `json:"estimatedDistanceKm,omitempty"`
	IncludedKM      float64        `json:"includedDistanceKm,omitempty"`
}

// doQuote builds the /prices request, calls the API, and flattens the response.
func doQuote(flags *rootFlags, serviceType, departAt string, durationSecs int, pickup geoPoint, dropoff *geoPoint) (*quoteResult, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"serviceCategory":  "prebooked",
		"serviceType":      serviceType,
		"departAt":         departAt,
		"pickup":           locToPrices(pickup),
		"featureFlags":     []any{},
		"voucherParameter": map[string]any{"autoApplyPromotion": true},
	}
	if serviceType == "hourly" {
		body["duration"] = durationSecs
	} else if dropoff != nil {
		body["dropoff"] = locToPrices(*dropoff)
	}

	data, _, err := c.Post(context.Background(), "/prices", body)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}

	var raw struct {
		Packages []struct {
			PackageSlug string   `json:"packageSlug"`
			Title       string   `json:"title"`
			Subtitle    string   `json:"subtitle"`
			Models      []string `json:"models"`
			Currency    string   `json:"currency"`
			Price       struct {
				Totals struct {
					GrossAmount string `json:"grossAmount"`
				} `json:"totals"`
			} `json:"price"`
			Settings struct {
				Seats struct {
					Maximum int `json:"maximum"`
				} `json:"seats"`
				Luggages struct {
					Cabin   int `json:"cabin"`
					Checked int `json:"checked"`
				} `json:"luggages"`
			} `json:"settings"`
		} `json:"packages"`
		Meta struct {
			EstimatedDistance float64 `json:"estimatedDistance"`
			IncludedDistance  float64 `json:"includedDistance"`
		} `json:"meta"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing quote response: %w", err)
	}
	res := &quoteResult{
		ServiceType:     serviceType,
		DepartAt:        departAt,
		DurationSeconds: durationSecs,
		Pickup:          pickup,
		Dropoff:         dropoff,
		EstimatedKM:     raw.Meta.EstimatedDistance / 1000.0,
		IncludedKM:      raw.Meta.IncludedDistance / 1000.0,
	}
	for _, p := range raw.Packages {
		res.Packages = append(res.Packages, quotePackage{
			PackageSlug: p.PackageSlug,
			Title:       p.Title,
			Subtitle:    p.Subtitle,
			// PATCH: trim whitespace from each model string. The upstream
			// /prices "models" array carries leading spaces (e.g. " BMW 5
			// Series"), which surfaced as ugly leading-space entries in
			// quote/fit/compare/trip output.
			Models:      trimModels(p.Models),
			GrossAmount: p.Price.Totals.GrossAmount,
			Currency:    p.Currency,
			MaxSeats:    p.Settings.Seats.Maximum,
			MaxLuggage:  p.Settings.Luggages.Cabin + p.Settings.Luggages.Checked,
		})
	}
	if len(res.Packages) == 0 {
		msg := "no vehicle classes available for this route/time"
		if len(raw.Errors) > 0 {
			msg = raw.Errors[0].Message
		}
		return res, fmt.Errorf("%s", msg)
	}
	// Cheapest first.
	sort.SliceStable(res.Packages, func(i, j int) bool {
		return amountFloat(res.Packages[i].GrossAmount) < amountFloat(res.Packages[j].GrossAmount)
	})
	return res, nil
}

func amountFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	return f
}

// hourlySeconds converts an --hourly hours flag to the seconds the /prices API
// expects, enforcing Blacklane's 2-hour minimum for by-the-hour bookings.
func hourlySeconds(hours int) (int, error) {
	if hours < 2 {
		return 0, fmt.Errorf("hourly bookings must be at least 2 hours (--hourly 2)")
	}
	return hours * 3600, nil
}

// normalizeDepartAt accepts YYYY-MM-DDTHH:MM or ...:SS and returns ...:SS.
func normalizeDepartAt(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("--at is required (e.g. --at 2026-06-25T15:00)")
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02T15:04:05"), nil
		}
	}
	return "", fmt.Errorf("invalid --at %q (use YYYY-MM-DDTHH:MM, e.g. 2026-06-25T15:00)", s)
}

func renderQuoteTable(cmd *cobra.Command, r *quoteResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	svc := r.ServiceType
	if svc != "" {
		svc = strings.ToUpper(svc[:1]) + svc[1:]
	}
	hdr := fmt.Sprintf("%s quote — %s", svc, r.DepartAt)
	if r.ServiceType == "hourly" {
		hdr += fmt.Sprintf(" (%dh)", r.DurationSeconds/3600)
	}
	fmt.Fprintln(cmd.OutOrStdout(), hdr)
	fmt.Fprintln(w, "CLASS\tPRICE\tSEATS\tBAGS\tVEHICLES")
	for _, p := range r.Packages {
		models := strings.Join(p.Models, ", ")
		if len(models) > 40 {
			models = models[:37] + "…"
		}
		fmt.Fprintf(w, "%s\t%s %s\t%d\t%d\t%s\n", p.Title, p.GrossAmount, p.Currency, p.MaxSeats, p.MaxLuggage, models)
	}
	w.Flush()
}

func emitQuote(cmd *cobra.Command, flags *rootFlags, r *quoteResult) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), r, flags)
	}
	renderQuoteTable(cmd, r)
	return nil
}

// ---- quote command ----

func newQuoteCmd(flags *rootFlags) *cobra.Command {
	var at string
	var hourly int
	var pLat, pLng, dLat, dLng float64

	cmd := &cobra.Command{
		Use:   "quote <pickup> [dropoff]",
		Short: "Quote a chauffeur ride (transfer or hourly) by address",
		Long: "Get upfront, fixed-price chauffeur quotes across vehicle classes.\n" +
			"Transfer: provide a pickup and dropoff. Hourly: provide a pickup and --hourly <hours>.\n" +
			"Addresses are resolved to coordinates via OpenStreetMap (no API key). No booking is ever made.",
		Example: strings.Trim(`
  blacklane-pp-cli quote "San Francisco Airport" "Union Square San Francisco" --at 2026-06-25T15:00
  blacklane-pp-cli quote "Union Square San Francisco" --hourly 3 --at 2026-06-25T15:00
  blacklane-pp-cli quote "JFK" "Times Square NYC" --at 2026-06-20T15:00 --agent --select packages.packageSlug,packages.grossAmount`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			departAt, err := normalizeDepartAt(at)
			if err != nil {
				return err
			}
			// Coordinate flags must be supplied in pairs (lat=0/lng=0 are valid
			// real coordinates, so we key off whether the flags were set).
			pickupCoords := cmd.Flags().Changed("pickup-lat") || cmd.Flags().Changed("pickup-lng")
			if pickupCoords && !(cmd.Flags().Changed("pickup-lat") && cmd.Flags().Changed("pickup-lng")) {
				return fmt.Errorf("provide both --pickup-lat and --pickup-lng together")
			}
			dropoffCoords := cmd.Flags().Changed("dropoff-lat") || cmd.Flags().Changed("dropoff-lng")
			if dropoffCoords && !(cmd.Flags().Changed("dropoff-lat") && cmd.Flags().Changed("dropoff-lng")) {
				return fmt.Errorf("provide both --dropoff-lat and --dropoff-lng together")
			}
			pickup, err := resolveLocation(args[0], pLat, pLng, pickupCoords, flags.timeout)
			if err != nil {
				return err
			}
			if hourly > 0 {
				secs, err := hourlySeconds(hourly)
				if err != nil {
					return err
				}
				if dryRunOK(flags) {
					return nil
				}
				r, err := doQuote(flags, "hourly", departAt, secs, pickup, nil)
				if err != nil {
					return err
				}
				return emitQuote(cmd, flags, r)
			}
			if len(args) < 2 && !dropoffCoords {
				return fmt.Errorf("transfer quotes need a dropoff: quote <pickup> <dropoff> --at <time> (or use --hourly <hours>)")
			}
			dropLabel := ""
			if len(args) >= 2 {
				dropLabel = args[1]
			}
			dropoff, err := resolveLocation(dropLabel, dLat, dLng, dropoffCoords, flags.timeout)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			r, err := doQuote(flags, "transfer", departAt, 0, pickup, &dropoff)
			if err != nil {
				return err
			}
			return emitQuote(cmd, flags, r)
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "Pickup datetime, e.g. 2026-06-25T15:00 (required)")
	cmd.Flags().IntVar(&hourly, "hourly", 0, "Hourly booking: number of hours (min 2). Omit dropoff.")
	cmd.Flags().Float64Var(&pLat, "pickup-lat", 0, "Pickup latitude (skip geocoding)")
	cmd.Flags().Float64Var(&pLng, "pickup-lng", 0, "Pickup longitude (skip geocoding)")
	cmd.Flags().Float64Var(&dLat, "dropoff-lat", 0, "Dropoff latitude (skip geocoding)")
	cmd.Flags().Float64Var(&dLng, "dropoff-lng", 0, "Dropoff longitude (skip geocoding)")
	return cmd
}

// ---- compare command ----

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var dates string
	var hourly int

	cmd := &cobra.Command{
		Use:   "compare <pickup> <dropoff>",
		Short: "Quote one route across several dates/times to find the cheapest",
		Long:  "Fan out the same route across multiple departure times and rank by lowest fare.\nDates are compared on the cheapest available vehicle class.",
		Example: strings.Trim(`
  blacklane-pp-cli compare "JFK" "Times Square NYC" --dates 2026-06-20T15:00,2026-06-21T15:00,2026-06-22T09:00
  blacklane-pp-cli compare "Union Square SF" --hourly 3 --dates 2026-06-20T09:00,2026-06-21T09:00 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			rawDates := splitCSV(dates)
			if len(rawDates) == 0 {
				return fmt.Errorf("--dates is required (comma-separated, e.g. --dates 2026-06-20T15:00,2026-06-21T15:00)")
			}
			pickup, err := resolveLocation(args[0], 0, 0, false, flags.timeout)
			if err != nil {
				return err
			}
			var dropoff *geoPoint
			if hourly > 0 {
				if _, err := hourlySeconds(hourly); err != nil {
					return err
				}
			} else {
				if len(args) < 2 {
					return fmt.Errorf("transfer compare needs a dropoff (or use --hourly <hours>)")
				}
				d, err := resolveLocation(args[1], 0, 0, false, flags.timeout)
				if err != nil {
					return err
				}
				dropoff = &d
			}
			if dryRunOK(flags) {
				return nil
			}
			type row struct {
				DepartAt    string `json:"departAt"`
				Cheapest    string `json:"cheapestClass"`
				GrossAmount string `json:"grossAmount"`
				Currency    string `json:"currency"`
				Error       string `json:"error,omitempty"`
			}
			var rows []row
			for _, d := range rawDates {
				departAt, err := normalizeDepartAt(d)
				if err != nil {
					rows = append(rows, row{DepartAt: d, Error: err.Error()})
					continue
				}
				st, secs := "transfer", 0
				if hourly > 0 {
					st, secs = "hourly", hourly*3600
				}
				r, err := doQuote(flags, st, departAt, secs, pickup, dropoff)
				if err != nil {
					rows = append(rows, row{DepartAt: departAt, Error: err.Error()})
					continue
				}
				cheapest := r.Packages[0]
				rows = append(rows, row{DepartAt: departAt, Cheapest: cheapest.Title, GrossAmount: cheapest.GrossAmount, Currency: cheapest.Currency})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Error != "" || rows[j].Error != "" {
					return rows[i].Error == "" && rows[j].Error != ""
				}
				return amountFloat(rows[i].GrossAmount) < amountFloat(rows[j].GrossAmount)
			})
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitDomainList(cmd, flags, rows)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DEPART\tCHEAPEST\tPRICE")
			for _, r := range rows {
				if r.Error != "" {
					fmt.Fprintf(w, "%s\t(unavailable)\t%s\n", r.DepartAt, r.Error)
					continue
				}
				fmt.Fprintf(w, "%s\t%s\t%s %s\n", r.DepartAt, r.Cheapest, r.GrossAmount, r.Currency)
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&dates, "dates", "", "Comma-separated pickup datetimes (required)")
	cmd.Flags().IntVar(&hourly, "hourly", 0, "Compare hourly bookings of N hours instead of transfers")
	return cmd
}

// ---- fit command ----

func newNovelFitCmd(flags *rootFlags) *cobra.Command {
	var at string
	var pax, bags, hourly int

	cmd := &cobra.Command{
		Use:   "fit <pickup> [dropoff]",
		Short: "Recommend the cheapest vehicle class that fits your party",
		Long:  "Quote a route, then pick the lowest-priced class whose seat and luggage capacity covers --pax and --bags.",
		Example: strings.Trim(`
  blacklane-pp-cli fit "JFK" "Times Square NYC" --pax 3 --bags 4 --at 2026-06-20T15:00
  blacklane-pp-cli fit "Union Square SF" --hourly 3 --pax 2 --bags 2 --at 2026-06-20T09:00 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			departAt, err := normalizeDepartAt(at)
			if err != nil {
				return err
			}
			pickup, err := resolveLocation(args[0], 0, 0, false, flags.timeout)
			if err != nil {
				return err
			}
			var dropoff *geoPoint
			st, secs := "transfer", 0
			if hourly > 0 {
				s, err := hourlySeconds(hourly)
				if err != nil {
					return err
				}
				st, secs = "hourly", s
			} else {
				if len(args) < 2 {
					return fmt.Errorf("transfer fit needs a dropoff (or use --hourly <hours>)")
				}
				d, err := resolveLocation(args[1], 0, 0, false, flags.timeout)
				if err != nil {
					return err
				}
				dropoff = &d
			}
			if dryRunOK(flags) {
				return nil
			}
			r, err := doQuote(flags, st, departAt, secs, pickup, dropoff)
			if err != nil {
				return err
			}
			var best *quotePackage
			for i := range r.Packages {
				p := r.Packages[i]
				if (pax == 0 || p.MaxSeats >= pax) && (bags == 0 || p.MaxLuggage >= bags) {
					best = &r.Packages[i]
					break // packages are sorted cheapest-first
				}
			}
			if best == nil {
				return fmt.Errorf("no vehicle class fits %d passengers and %d bags on this route", pax, bags)
			}
			out := map[string]any{
				"recommended":  best,
				"forParty":     map[string]int{"pax": pax, "bags": bags},
				"departAt":     departAt,
				"alternatives": r.Packages,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Best fit for %d pax / %d bags: %s — %s %s (seats %d, bags %d)\n",
				pax, bags, best.Title, best.GrossAmount, best.Currency, best.MaxSeats, best.MaxLuggage)
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "Pickup datetime (required)")
	cmd.Flags().IntVar(&pax, "pax", 0, "Number of passengers that must fit")
	cmd.Flags().IntVar(&bags, "bags", 0, "Number of bags that must fit")
	cmd.Flags().IntVar(&hourly, "hourly", 0, "Hourly booking of N hours instead of a transfer")
	return cmd
}

// ---- trip command (multi-leg) ----

func newNovelTripCmd(flags *rootFlags) *cobra.Command {
	var at string
	var legs []string

	cmd := &cobra.Command{
		Use:   "trip",
		Short: "Quote a multi-leg journey and total the cheapest-class fares",
		Long:  "Quote a sequence of transfer legs (each 'From>To') departing at --at and sum the cheapest class per leg.",
		Example: strings.Trim(`
  blacklane-pp-cli trip --leg "JFK>Times Square NYC" --leg "Times Square NYC>LaGuardia" --at 2026-06-20T09:00
  blacklane-pp-cli trip --leg "SFO>Union Square SF" --at 2026-06-20T09:00 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(legs) == 0 {
				return cmd.Help()
			}
			departAt, err := normalizeDepartAt(at)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			type legRow struct {
				Leg         string `json:"leg"`
				Cheapest    string `json:"cheapestClass"`
				GrossAmount string `json:"grossAmount"`
				Currency    string `json:"currency"`
				Error       string `json:"error,omitempty"`
			}
			var rows []legRow
			totals := map[string]float64{} // per-currency; Blacklane prices each leg in local currency
			for _, leg := range legs {
				parts := strings.SplitN(leg, ">", 2)
				if len(parts) != 2 {
					rows = append(rows, legRow{Leg: leg, Error: "expected 'From>To'"})
					continue
				}
				pu, err := resolveLocation(strings.TrimSpace(parts[0]), 0, 0, false, flags.timeout)
				if err != nil {
					rows = append(rows, legRow{Leg: leg, Error: err.Error()})
					continue
				}
				du, err := resolveLocation(strings.TrimSpace(parts[1]), 0, 0, false, flags.timeout)
				if err != nil {
					rows = append(rows, legRow{Leg: leg, Error: err.Error()})
					continue
				}
				r, err := doQuote(flags, "transfer", departAt, 0, pu, &du)
				if err != nil {
					rows = append(rows, legRow{Leg: leg, Error: err.Error()})
					continue
				}
				c := r.Packages[0]
				rows = append(rows, legRow{Leg: leg, Cheapest: c.Title, GrossAmount: c.GrossAmount, Currency: c.Currency})
				totals[c.Currency] += amountFloat(c.GrossAmount)
			}
			type currencyTotal struct {
				Currency string `json:"currency"`
				Total    string `json:"total"`
			}
			var totalsList []currencyTotal
			for cur, amt := range totals {
				totalsList = append(totalsList, currencyTotal{Currency: cur, Total: fmt.Sprintf("%.2f", amt)})
			}
			out := map[string]any{
				"legs":     rows,
				"totals":   totalsList,
				"departAt": departAt,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "LEG\tCLASS\tPRICE")
			for _, r := range rows {
				if r.Error != "" {
					fmt.Fprintf(w, "%s\t(unavailable)\t%s\n", r.Leg, r.Error)
					continue
				}
				fmt.Fprintf(w, "%s\t%s\t%s %s\n", r.Leg, r.Cheapest, r.GrossAmount, r.Currency)
			}
			w.Flush()
			for cur, amt := range totals {
				fmt.Fprintf(cmd.OutOrStdout(), "TOTAL (%s): %.2f\n", cur, amt)
			}
			if len(totals) > 1 {
				fmt.Fprintln(cmd.OutOrStdout(), "(legs span multiple currencies — totalled per currency)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "Pickup datetime for all legs (required)")
	cmd.Flags().StringArrayVar(&legs, "leg", nil, "A leg as 'From>To' (repeatable)")
	return cmd
}

// emitDomainList prints a curated list/object that is already lean. It honors
// --select and --csv but skips the generic --compact allowlist, which would
// otherwise strip every field from these domain rows (their keys aren't in the
// id/name/status allowlist compactListFields keeps).
func emitDomainList(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data := json.RawMessage(raw)
	if flags.csv {
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	}
	return printOutput(cmd.OutOrStdout(), data, true)
}

// PATCH: trimModels normalizes vehicle-model strings from the upstream
// /prices response, which can carry leading/trailing spaces (e.g.
// " BMW 5 Series"). Empty entries are dropped.
func trimModels(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	for _, m := range in {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
