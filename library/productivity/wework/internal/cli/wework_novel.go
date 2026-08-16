// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel + friendly commands. Markerless & self-registering via
// registerNovelCommand so `generate --force` preserves both source and wiring.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// Registered before the scaffold `desks` (root wires hooks first), so
		// addNovelCommandIfAbsent skips the scaffold and this real one wins.
		addNovelCommandIfAbsent(root, newWeworkDesksCmd(flags))
		addNovelCommandIfAbsent(root, newCitiesCmd(flags))
		addNovelCommandIfAbsent(root, newBookingsCmd(flags))
		addNovelCommandIfAbsent(root, newLocationsCmd(flags))
		addNovelCommandIfAbsent(root, newBookCmd(flags))
		addNovelCommandIfAbsent(root, newCancelCmd(flags))
	})
}

// ---- desks: city-name search with derived bounding box + local ranking ----
//
// pp:data-source live
// The desks feature queries the WeWork API live on every call (get-affiliate-cities,
// get-affiliate-locations, get-spaces) and derives the map bounding box from city geo;
// it does not read the local store. Ranking/filtering happen in-process on the live result.

type deskCity struct {
	Name      string `json:"name"`
	Marketgeo struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"marketgeo"`
}

type deskSearchResponse struct {
	GetSharedWorkspaces struct {
		TotalCount int               `json:"totalCount"`
		Workspaces []json.RawMessage `json:"workspaces"`
	} `json:"getSharedWorkspaces"`
}

type deskLite struct {
	Credits        float64 `json:"credits"`
	SeatsAvailable int     `json:"seatsAvailable"`
	Seat           struct {
		Available int `json:"available"`
	} `json:"seat"`
	ProductPrice struct {
		Price struct {
			Amount float64 `json:"amount"`
		} `json:"price"`
	} `json:"productPrice"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
}

func (d deskLite) available() int {
	if d.Seat.Available > 0 {
		return d.Seat.Available
	}
	return d.SeatsAvailable
}

func normalizeWeworkDate(value string) (string, error) {
	date := strings.TrimSpace(value)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", usageErr(fmt.Errorf("--date must be YYYY-MM-DD, got %q", date))
	}
	return date, nil
}

func validateWeworkLiveDataSource(flags *rootFlags) error {
	if err := validateDataSourceStrategy(flags, "live"); err != nil {
		return usageErr(err)
	}
	return nil
}

func printWeworkLiveJSONFiltered(w io.Writer, value any, flags *rootFlags) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(w, json.RawMessage(raw), flags, map[string]any{"source": "live"})
}

func newWeworkDesksCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagDate, flagSort string
	var flagAvailableOnly bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "desks",
		Short: "Search bookable desks by city name and date, deriving the map bounding box from cached city geo.",
		Long: "Search bookable WeWork day-desks by city name and date. Resolves the city to its\n" +
			"map coordinates (via get-affiliate-cities), derives the bounding box the desk-search\n" +
			"API requires, then optionally ranks by credits/price and filters to open seats.\n" +
			"Requires auth (WEWORK_TOKEN + WEWORK_UUID + WEWORK_MEMBER_TYPE).",
		Example: strings.Trim(`
  wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --agent
  wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --sort credits --available-only
  wework-pp-cli desks --city "New York, NY" --agent --select desks.location.name,desks.credits`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateWeworkLiveDataSource(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagCity) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--city is required (e.g. --city \"Austin, TX\")"))
			}
			if flagSort != "" && flagSort != "credits" && flagSort != "price" {
				return usageErr(fmt.Errorf("--sort must be 'credits' or 'price', got %q", flagSort))
			}
			date, err := normalizeWeworkDate(flagDate)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			citiesRaw, err := c.Get(ctx, "/wework-yardi/location/get-affiliate-cities", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var cities []deskCity
			if err := json.Unmarshal(citiesRaw, &cities); err != nil {
				return fmt.Errorf("parsing cities: %w", err)
			}
			match, ok := matchCity(cities, flagCity)
			if !ok {
				return usageErr(fmt.Errorf("no bookable WeWork city matched %q — try 'wework-pp-cli cities' to list valid names", flagCity))
			}
			lat, lng := match.Marketgeo.Latitude, match.Marketgeo.Longitude
			if lat == 0 && lng == 0 {
				return fmt.Errorf("city %q has no coordinates; cannot derive a search area", match.Name)
			}

			const dLat, dLng = 0.18, 0.22
			bounds := map[string]string{
				"boundnwLat":    formatCLIParamValue(lat + dLat),
				"boundnwLng":    formatCLIParamValue(lng - dLng),
				"boundseLat":    formatCLIParamValue(lat - dLat),
				"boundseLng":    formatCLIParamValue(lng + dLng),
				"userLatitude":  formatCLIParamValue(lat),
				"userLongitude": formatCLIParamValue(lng),
			}
			// get-affiliate-locations wants the bare city name (no ", ST" suffix).
			cityParam := match.Name
			if i := strings.Index(cityParam, ","); i >= 0 {
				cityParam = strings.TrimSpace(cityParam[:i])
			}

			// Step 1: buildings in the city (their numeric location UUIDs).
			alParams := map[string]string{"city": cityParam, "date": date, "endDate": "", "type": "0", "platFormType": "1"}
			for k, v := range bounds {
				alParams[k] = v
			}
			alRaw, err := c.Get(ctx, "/spaces/get-affiliate-locations", alParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var alResp struct {
				LocationsByGeo []struct {
					Uuid string `json:"uuid"`
				} `json:"locationsByGeo"`
			}
			if err := json.Unmarshal(alRaw, &alResp); err != nil {
				return fmt.Errorf("parsing locations: %w", err)
			}
			uuids := make([]string, 0, len(alResp.LocationsByGeo))
			for _, l := range alResp.LocationsByGeo {
				if l.Uuid != "" {
					uuids = append(uuids, l.Uuid)
				}
			}

			// Step 2: desks across those buildings. get-spaces returns nothing
			// without locationUUIDs, and the full param set below is required.
			var resp deskSearchResponse
			if len(uuids) > 0 {
				params := map[string]string{
					"date": date, "endDate": "", "type": "0", "locationType": "1", "platFormType": "1",
					"capacity": "0", "duration": "0", "roomTypeFilter": "", "isWeb": "false", "isFromWp": "false",
					"offset": "0", "limit": "500", "closestCity": "", "locationUUIDs": strings.Join(uuids, ","),
				}
				for k, v := range bounds {
					params[k] = v
				}
				raw, err := c.Get(ctx, "/spaces/get-spaces", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if err := json.Unmarshal(raw, &resp); err != nil {
					return fmt.Errorf("parsing desks: %w", err)
				}
			}

			type entry struct {
				raw  json.RawMessage
				lite deskLite
			}
			entries := make([]entry, 0, len(resp.GetSharedWorkspaces.Workspaces))
			for _, w := range resp.GetSharedWorkspaces.Workspaces {
				var l deskLite
				_ = json.Unmarshal(w, &l)
				if flagAvailableOnly && l.available() <= 0 {
					continue
				}
				entries = append(entries, entry{raw: w, lite: l})
			}
			if flagSort != "" {
				sort.SliceStable(entries, func(i, j int) bool {
					if flagSort == "price" {
						return entries[i].lite.ProductPrice.Price.Amount < entries[j].lite.ProductPrice.Price.Amount
					}
					return entries[i].lite.Credits < entries[j].lite.Credits
				})
			}
			if flagLimit > 0 && len(entries) > flagLimit {
				entries = entries[:flagLimit]
			}

			result := make([]json.RawMessage, 0, len(entries))
			for _, e := range entries {
				result = append(result, e.raw)
			}
			out := map[string]any{"city": match.Name, "date": date, "count": len(result), "desks": result}

			if flags.csv || flags.plain || flags.quiet {
				return printWeworkLiveJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printWeworkLiveJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No bookable desks found in %s for %s.\n", match.Name, date)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %8s  %6s  %5s\n", "LOCATION", "CREDITS", "PRICE", "AVAIL")
			for _, e := range entries {
				name := e.lite.Location.Name
				if len(name) > 40 {
					name = name[:39] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %8.0f  %6.0f  %5d\n", name, e.lite.Credits, e.lite.ProductPrice.Price.Amount, e.lite.available())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d desk(s) in %s for %s\n", len(entries), match.Name, date)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City name to search, e.g. \"Austin, TX\" (required)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Booking date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&flagSort, "sort", "", "Sort by 'credits' or 'price' (ascending)")
	cmd.Flags().BoolVar(&flagAvailableOnly, "available-only", false, "Only desks with open seats")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Max desks to return (0 = all)")
	return cmd
}

func matchCity(cities []deskCity, want string) (deskCity, bool) {
	w := strings.ToLower(strings.TrimSpace(want))
	for _, c := range cities {
		if strings.ToLower(strings.TrimSpace(c.Name)) == w {
			return c, true
		}
	}
	wCity := w
	if i := strings.Index(w, ","); i >= 0 {
		wCity = strings.TrimSpace(w[:i])
	}
	for _, c := range cities {
		cn := strings.ToLower(c.Name)
		if strings.HasPrefix(cn, wCity) || strings.Contains(cn, w) {
			return c, true
		}
	}
	return deskCity{}, false
}

type cityFilterIdentity struct {
	Name      string `json:"name"`
	Marketgeo struct {
		Name             string `json:"name"`
		NameAbbreviation string `json:"name_abbreviation"`
	} `json:"marketgeo"`
	Countrygeo struct {
		Name             string `json:"name"`
		ISO              string `json:"iso"`
		NameAbbreviation string `json:"name_abbreviation"`
	} `json:"countrygeo"`
	NearbyLocation struct {
		City           string `json:"city"`
		State          string `json:"state"`
		DefaultCountry string `json:"default_country"`
	} `json:"nearby_location"`
}

func normalizeCityFilterText(value string) string {
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.Join(parts, " ")
}

func cityMatchesIdentityFilter(raw json.RawMessage, filter string) bool {
	needle := normalizeCityFilterText(filter)
	if needle == "" {
		return false
	}
	var identity cityFilterIdentity
	if err := json.Unmarshal(raw, &identity); err != nil {
		return false
	}
	haystack := normalizeCityFilterText(strings.Join([]string{
		identity.Name,
		identity.Marketgeo.Name,
		identity.Marketgeo.NameAbbreviation,
		identity.Countrygeo.Name,
		identity.Countrygeo.ISO,
		identity.Countrygeo.NameAbbreviation,
		identity.NearbyLocation.City,
		identity.NearbyLocation.State,
		identity.NearbyLocation.DefaultCountry,
	}, " "))
	return strings.Contains(haystack, needle)
}

// ---- friendly top-level aliases over generated endpoint commands ----

func newCitiesCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagFilter string
	cmd := &cobra.Command{
		Use:         "cities",
		Short:       "List bookable WeWork cities",
		Example:     "  wework-pp-cli cities --limit 5\n  wework-pp-cli cities --filter austin --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWeworkLiveDataSource(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), "/wework-yardi/location/get-affiliate-cities", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var cities []json.RawMessage
			if err := json.Unmarshal(raw, &cities); err != nil {
				return fmt.Errorf("parsing cities: %w", err)
			}
			if flagFilter != "" {
				kept := make([]json.RawMessage, 0, len(cities))
				for _, ci := range cities {
					if cityMatchesIdentityFilter(ci, flagFilter) {
						kept = append(kept, ci)
					}
				}
				cities = kept
			}
			if flagLimit > 0 && len(cities) > flagLimit {
				cities = cities[:flagLimit]
			}
			return printWeworkLiveJSONFiltered(cmd.OutOrStdout(), cities, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Max cities to return (0 = all)")
	cmd.Flags().StringVar(&flagFilter, "filter", "", "Case-insensitive city identity filter (name, market, state, or country)")
	return cmd
}

func newBookingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bookings",
		Short:       "List my upcoming WeWork desk bookings",
		Example:     "  wework-pp-cli bookings\n  wework-pp-cli bookings --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWeworkLiveDataSource(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), "/common-booking/upcoming-bookings", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live"})
		},
	}
	return cmd
}
