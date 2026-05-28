// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

// near "<address>" --checkin --checkout [--radius Nmi] [--center "lat,lng"]
// Geo-radius search around a specific address. Google Hotels accepts
// free-text addresses in its query (so "hotels near 1600 Amphitheatre"
// works), but Google doesn't expose a hard radius filter. This command
// applies a Haversine distance filter post-fetch.
//
// Geocoding the --center pivot is done via Nominatim (free, no key) when
// --center is not supplied explicitly. Nominatim has a polite rate limit
// (~1 req/sec) and a free-use policy — we identify the requester via the
// User-Agent.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/parser"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNearCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout string
	var radiusStr, centerStr string
	var opts hotelSearchOpts
	var hotelClassCSV string

	cmd := &cobra.Command{
		Use:   "near <address>",
		Short: "Search hotels near a specific address, optionally filtered by radius",
		Long: `Search hotels near a specific address.

Address is passed through to Google Hotels as "hotels near <address>" so
Google geocodes it natively. If --radius is set, results are filtered by
Haversine distance from a center point. The center is either:

  - the explicit --center "lat,lng" you supply, or
  - the address geocoded via OpenStreetMap Nominatim (free, no API key)

Per-hotel filters (brand, max-price, min-rating, etc.) apply normally.`,
		Example: strings.Trim(`
  hotel-goat-pp-cli near "Times Square, New York" --checkin 2026-08-15 --checkout 2026-08-17
  hotel-goat-pp-cli near "1600 Amphitheatre Pkwy, Mountain View, CA" --checkin 2026-08-15 --checkout 2026-08-17 --radius 2mi
  hotel-goat-pp-cli near "Moscone Center, San Francisco" --checkin 2026-09-10 --checkout 2026-09-13 --radius 1mi --max-price 400
  hotel-goat-pp-cli near "SFO" --checkin 2026-08-15 --checkout 2026-08-17 --radius 3mi --center "37.7749,-122.4194"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return cmd.Help()
			}
			address := args[0]
			if checkin == "" || checkout == "" {
				return fmt.Errorf("--checkin and --checkout are required (YYYY-MM-DD)")
			}
			if err := validateYYYYMMDD("checkin", checkin); err != nil {
				return err
			}
			if err := validateYYYYMMDD("checkout", checkout); err != nil {
				return err
			}

			// Parse radius into miles
			radiusMiles, err := parseRadius(radiusStr)
			if err != nil {
				return err
			}

			// Resolve center coordinates when --radius is set.
			// The dryRunOK short-circuit at the top of RunE means the
			// dry-run-vs-live branch here is no longer reachable —
			// always do the real geocode when we get this far.
			var centerLat, centerLng float64
			var centerSource string
			if radiusMiles > 0 {
				if centerStr != "" {
					centerLat, centerLng, err = parseLatLng(centerStr)
					if err != nil {
						return fmt.Errorf("invalid --center %q: %w", centerStr, err)
					}
					centerSource = "user"
				} else {
					centerLat, centerLng, err = geocodeNominatim(cmd.Context(), address)
					if err != nil {
						return fmt.Errorf("geocoding %q via Nominatim: %w", address, err)
					}
					centerSource = "nominatim"
				}
			}

			// Parse hotel-class CSV
			if hotelClassCSV != "" {
				for _, s := range strings.Split(hotelClassCSV, ",") {
					var c int
					if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &c); err == nil && c >= 1 && c <= 5 {
						opts.HotelClass = append(opts.HotelClass, c)
					}
				}
			}

			query := "hotels near " + address

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			hotels, source, err := fetchAndParseHotels(ctx, query, checkin, checkout, opts)
			if err != nil {
				return err
			}

			// Apply radius filter (post-fetch Haversine) when --radius set
			filtered := hotels
			if radiusMiles > 0 {
				filtered = filtered[:0]
				for _, h := range hotels {
					if h.Latitude == 0 && h.Longitude == 0 {
						continue
					}
					d := haversineMiles(centerLat, centerLng, h.Latitude, h.Longitude)
					if d <= radiusMiles {
						h.NearbyDistanceMiles = d
						filtered = append(filtered, h)
					}
				}
			}

			meta := map[string]any{
				"address":        address,
				"checkin":        checkin,
				"checkout":       checkout,
				"count":          len(filtered),
				"unfiltered":     len(hotels),
				"source":         source,
				"fetched_at":     time.Now().UTC().Format(time.RFC3339),
				"parser_version": parser.ParserVersion,
			}
			if radiusMiles > 0 {
				meta["radius_miles"] = radiusMiles
				meta["center_lat"] = centerLat
				meta["center_lng"] = centerLng
				meta["center_source"] = centerSource
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"meta":    meta,
				"results": filtered,
			}, flags)
		},
	}

	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&radiusStr, "radius", "", "Filter to hotels within this radius (e.g. '2mi', '5km')")
	cmd.Flags().StringVar(&centerStr, "center", "", "Explicit center coordinates 'lat,lng' (skips geocoding)")
	cmd.Flags().StringVar(&opts.Currency, "currency", "", "ISO 4217 currency code (USD, EUR, ...)")
	cmd.Flags().Float64Var(&opts.MinRating, "min-rating", 0, "Filter to hotels with rating >= this (0-5)")
	cmd.Flags().Float64Var(&opts.MaxPrice, "max-price", 0, "Filter to hotels with price/night <= this")
	cmd.Flags().Float64Var(&opts.MinPrice, "min-price", 0, "Filter to hotels with price/night >= this")
	cmd.Flags().StringVar(&hotelClassCSV, "hotel-class", "", "Filter to specific star ratings (CSV: 3,4,5)")
	cmd.Flags().StringSliceVar(&opts.Brand, "brand", nil, "Filter by brand prefix (CSV: Hyatt,Marriott)")
	cmd.Flags().StringSliceVar(&opts.Amenities, "amenities", nil, "Filter by amenities (CSV)")
	cmd.Flags().IntVar(&opts.Limit, "limit", 20, "Max hotels to return after filtering")

	return cmd
}

// parseRadius accepts "5mi", "5 mi", "5", "5km" — returns miles.
func parseRadius(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	unit := "mi"
	for _, suffix := range []string{"miles", "mile", "mi", "kilometers", "kilometer", "km", "k"} {
		if strings.HasSuffix(s, suffix) {
			s = strings.TrimSuffix(s, suffix)
			s = strings.TrimSpace(s)
			if suffix == "km" || suffix == "k" || suffix == "kilometer" || suffix == "kilometers" {
				unit = "km"
			}
			break
		}
	}
	var n float64
	if _, err := fmt.Sscanf(s, "%f", &n); err != nil || n < 0 {
		return 0, fmt.Errorf("invalid --radius %q (e.g. '2mi', '5km')", s)
	}
	if unit == "km" {
		n = n * 0.621371 // km -> mi
	}
	return n, nil
}

func parseLatLng(s string) (float64, float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected 'lat,lng'")
	}
	var lat, lng float64
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &lat); err != nil {
		return 0, 0, fmt.Errorf("invalid latitude %q", parts[0])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &lng); err != nil {
		return 0, 0, fmt.Errorf("invalid longitude %q", parts[1])
	}
	if math.Abs(lat) > 90 || math.Abs(lng) > 180 {
		return 0, 0, fmt.Errorf("lat/lng out of range")
	}
	return lat, lng, nil
}

// geocodeNominatim resolves a free-text address to lat/lng via the public
// OpenStreetMap Nominatim service. No API key required; respect the polite
// rate limit (~1 req/sec) and identify ourselves via the User-Agent.
func geocodeNominatim(ctx context.Context, address string) (float64, float64, error) {
	params := url.Values{}
	params.Set("q", address)
	params.Set("format", "json")
	params.Set("limit", "1")
	u := "https://nominatim.openstreetmap.org/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "hotel-goat-pp-cli/1.0 (https://github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "application/json")

	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("nominatim HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}
	var raw []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, 0, fmt.Errorf("parse nominatim response: %w", err)
	}
	if len(raw) == 0 {
		return 0, 0, fmt.Errorf("no geocoding result for %q (try a more specific address or supply --center 'lat,lng')", address)
	}
	var lat, lng float64
	if _, err := fmt.Sscanf(raw[0].Lat, "%f", &lat); err != nil {
		return 0, 0, fmt.Errorf("invalid lat from nominatim: %q", raw[0].Lat)
	}
	if _, err := fmt.Sscanf(raw[0].Lon, "%f", &lng); err != nil {
		return 0, 0, fmt.Errorf("invalid lng from nominatim: %q", raw[0].Lon)
	}
	return lat, lng, nil
}
